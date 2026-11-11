// Package stomp implements the STOMP-over-WebSocket server on :8002 that the
// kiosk web UI (and any STOMP client) connects to — a faithful port of the
// C++ WebsocketServer/StompTopic pair, including last-will, retained topics
// and the message-id/subscription header semantics.
package stomp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// FrameType enumerates the STOMP frame commands the server handles.
type FrameType int

const (
	FrameConnect FrameType = iota
	FrameDisconnect
	FrameSend
	FrameSubscribe
	FrameUnsubscribe
	FrameAck
	FrameNack
	FrameConnected
	FrameMessage
	FrameReceipt
	FrameError
)

// Frame is a parsed STOMP frame.
type Frame struct {
	Type    FrameType
	Headers map[string]string
	Body    string
}

// LastWill mirrors the C++ StompLastWill struct.
type LastWill struct {
	Topic    string
	Message  string
	Retained bool
}

// topic is a registry entry with subscribers and a retained value.
type topic struct {
	name       string
	retained   bool
	value      string
	hasValue   bool
	mu         sync.RWMutex
	subscribers map[*conn]string // conn -> subscription id
}

// conn wraps a websocket connection with its STOMP state.
type conn struct {
	ws       *websocket.Conn
	sendMu   sync.Mutex
	lastWill LastWill
	closed   bool
}

// Server is the STOMP-over-WebSocket server.
type Server struct {
	mu       sync.RWMutex
	topics   map[string]*topic
	conns    map[*conn]struct{}
	callbacks map[string]func(frame Frame) // per-destination SEND callbacks
	globalCallbacks []func(frame Frame)   // called for every SEND
	uid      int64
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // nginx-proxied
}

// NewServer creates a STOMP server.
func NewServer() *Server {
	return &Server{
		topics:    make(map[string]*topic),
		conns:     make(map[*conn]struct{}),
		callbacks: make(map[string]func(Frame)),
	}
}

// Start begins listening on addr (e.g. ":8002"). Blocks; call in a goroutine.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWS)
	for {
		log.Printf("Starting STOMP WebSocket server on %s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("STOMP server error: %v — retrying in 1s", err)
		}
		time.Sleep(time.Second)
	}
}



// AttachCallback registers a per-destination SEND callback (exact mirror of
// WebsocketServer::attach_stomp_callback).
func (s *Server) AttachCallback(destination string, cb func(frame Frame)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks[destination] = cb
}

// AttachGlobalCallback registers a callback invoked for every SEND frame.
func (s *Server) AttachGlobalCallback(cb func(frame Frame)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.globalCallbacks = append(s.globalCallbacks, cb)
}

// UpdateTopicValue sets a topic's value and fans out MESSAGE frames to
// subscribers (retained topics keep the value for later subscribers).
func (s *Server) UpdateTopicValue(topicName, value string) {
	s.mu.RLock()
	t := s.topics[topicName]
	s.mu.RUnlock()
	if t != nil {
		t.update(value)
	}
}

// SetTopicRetained sets the retained flag, creating the topic when missing
// (exact mirror of set_stomp_topic_retained).
func (s *Server) SetTopicRetained(topicName string, retained bool) {
	s.mu.Lock()
	t := s.topics[topicName]
	if t == nil {
		t = &topic{name: topicName, subscribers: make(map[*conn]string)}
		s.topics[topicName] = t
	}
	t.mu.Lock()
	t.retained = retained
	t.mu.Unlock()
	s.mu.Unlock()
}

// HasSubscribers reports whether any STOMP client is connected.
func (s *Server) HasSubscribers() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.conns) > 0
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/stomp") {
		log.Printf("Connected websocket does not end with /stomp. Unknown protocol. Closing.")
		_ = ws.Close()
		return
	}

	c := &conn{ws: ws}
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
	log.Printf("STOMP client connected: %s", r.RemoteAddr)

	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		s.fireLastWill(c)
		for _, t := range s.snapshotTopics() {
			t.unsubscribe(c, "")
		}
		_ = ws.Close()
		log.Printf("STOMP client disconnected: %s", r.RemoteAddr)
	}()

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return
		}
		// Heartbeat: single \n frame.
		if len(data) == 1 && data[0] == '\n' {
			continue
		}
		frame, ok := decodeFrame(string(data))
		if !ok {
			log.Printf("Failed to parse STOMP frame")
			continue
		}
		s.processFrame(c, frame)
	}
}

func (s *Server) snapshotTopics() []*topic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*topic, 0, len(s.topics))
	for _, t := range s.topics {
		out = append(out, t)
	}
	return out
}

func (s *Server) processFrame(c *conn, frame Frame) {
	switch frame.Type {
	case FrameConnect:
		acceptVersion := frame.Headers["accept-version"]
		supported := false
		for _, v := range strings.Split(acceptVersion, ",") {
			if strings.TrimSpace(v) == "1.2" {
				supported = true
				break
			}
		}
		if acceptVersion == "" {
			log.Printf("Accepting websocket connection even though accept-version header is empty. Assuming STOMP v1.2.")
			supported = true
		}
		if !supported {
			errFrame := Frame{Type: FrameError, Headers: map[string]string{"version": "1.2"},
				Body: "Supported protocol versions are 1.2"}
			c.send(errFrame)
			return
		}

		connected := Frame{
			Type: FrameConnected,
			Headers: map[string]string{
				"version": "1.2",
				"session": newUUID(),
				"server":  "NSPanelManager",
			},
		}
		if hb := frame.Headers["heart-beat"]; hb != "" && hb != "0,0" {
			parts := strings.Split(hb, ",")
			if len(parts) >= 2 && parts[0] != "0" {
				connected.Headers["heart-beat"] = "0,1000"
			} else if len(parts) >= 1 && parts[0] != "0" {
				connected.Headers["heart-beat"] = "0,1000"
			}
		}
		c.send(connected)

		lw := LastWill{
			Topic:    frame.Headers["last_will_topic"],
			Message:  frame.Headers["last_will_message"],
			Retained: strings.EqualFold(frame.Headers["last_will_retained"], "true"),
		}
		if lw.Topic != "" {
			log.Printf("Client requested last will message '%s' -> '%s' be sent on disconnect. Retained? %v", lw.Topic, lw.Message, lw.Retained)
			c.lastWill = lw
		}

	case FrameSubscribe:
		destination := frame.Headers["destination"]
		subID := frame.Headers["id"]
		if destination == "" || subID == "" {
			log.Printf("Got STOMP SUBSCRIBE frame but 'destination' or 'id' missing")
			return
		}
		log.Printf("STOMP subscribing to topic %s (id: %s)", destination, subID)
		s.mu.RLock()
		t := s.topics[destination]
		s.mu.RUnlock()
		if t == nil {
			s.mu.Lock()
			t = s.topics[destination]
			if t == nil {
				t = &topic{name: destination, subscribers: make(map[*conn]string)}
				s.topics[destination] = t
			}
			s.mu.Unlock()
		}
		t.subscribe(c, subID)

	case FrameUnsubscribe:
		destination := frame.Headers["destination"]
		log.Printf("STOMP subscription ID %s unsubscribing.", frame.Headers["id"])
		for _, t := range s.snapshotTopics() {
			if destination == "" || t.name == destination {
				t.unsubscribe(c, frame.Headers["id"])
			}
		}

	case FrameSend:
		destination := frame.Headers["destination"]
		log.Printf("Received STOMP SEND command for topic '%s'", destination)
		s.mu.RLock()
		globals := append([]func(Frame){}, s.globalCallbacks...)
		cb, hasCb := s.callbacks[destination]
		s.mu.RUnlock()

		for _, g := range globals {
			g(frame)
		}
		if hasCb {
			cb(frame)
		} else if !strings.HasPrefix(destination, "mqtt/") {
			log.Printf("Received STOMP SEND command for unknown topic. Will not publish, topic '%s'", destination)
		}

	case FrameDisconnect:
		// Client-initiated disconnect; connection closes via defer.
	default:
		log.Printf("Received unknown STOMP frame type %d.", frame.Type)
	}
}

// fireLastWill publishes a connection's last will on disconnect.
func (s *Server) fireLastWill(c *conn) {
	lw := c.lastWill
	if lw.Topic == "" {
		return
	}
	log.Printf("Websocket had last will. Will send last will message on '%s' -> '%s'. Retained? %v", lw.Topic, lw.Message, lw.Retained)
	s.SetTopicRetained(lw.Topic, lw.Retained)
	s.UpdateTopicValue(lw.Topic, lw.Message)
	s.mu.RLock()
	globals := append([]func(Frame){}, s.globalCallbacks...)
	cb, hasCb := s.callbacks[lw.Topic]
	s.mu.RUnlock()
	lwFrame := Frame{Type: FrameSend, Headers: map[string]string{"destination": lw.Topic}, Body: lw.Message}
	for _, g := range globals {
		g(lwFrame)
	}
	if hasCb {
		cb(lwFrame)
	}
}

// subscribe registers a subscriber and immediately delivers the retained
// value if one exists (mirror of StompTopic::subscribe).
func (t *topic) subscribe(c *conn, subID string) {
	t.mu.Lock()
	t.subscribers[c] = subID
	hasValue := t.hasValue && t.value != ""
	value := t.value
	t.mu.Unlock()
	if hasValue {
		frame := Frame{
			Type: FrameMessage,
			Headers: map[string]string{
				"message-id":   newUUID(),
				"content-type": "text/plain",
				"destination":  t.name,
				"subscription": subID,
			},
			Body: value,
		}
		c.send(frame)
	}
}

// unsubscribe removes a subscription; empty subID removes all for the conn.
func (t *topic) unsubscribe(c *conn, subID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for cc, id := range t.subscribers {
		if cc == c && (id == subID || subID == "") {
			delete(t.subscribers, cc)
		}
	}
}

// update stores (if retained) and fans out the value to subscribers.
func (t *topic) update(value string) {
	t.mu.Lock()
	if t.retained {
		t.value = value
		t.hasValue = true
	}
	t.mu.Unlock()

	t.mu.RLock()
	n := len(t.subscribers)
	t.mu.RUnlock()
	if n == 0 {
		return
	}
	for c, subID := range t.snapshotSubscribers() {
		frame := Frame{
			Type: FrameMessage,
			Headers: map[string]string{
				"message-id":   newUUID(),
				"content-type": "text/plain",
				"destination":  t.name,
				"subscription": subID,
			},
			Body: value,
		}
		c.send(frame)
	}
}

func (t *topic) snapshotSubscribers() map[*conn]string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[*conn]string, len(t.subscribers))
	for c, id := range t.subscribers {
		out[c] = id
	}
	return out
}

// send writes a frame to the connection, serialized per-connection.
func (c *conn) send(frame Frame) {
	msg := encodeFrame(frame)
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.closed {
		return
	}
	_ = c.ws.SetWriteDeadline(writeDeadline())
	if err := c.ws.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		log.Printf("STOMP write failed: %v", err)
	}
}

// decodeFrame parses a STOMP frame — headers with escape sequences
// (\r \n \c \\), body terminated by NUL (or content-length).
func decodeFrame(data string) (Frame, bool) {
	frame := Frame{Headers: make(map[string]string)}
	lines := strings.Split(data, "\n")
	i := 0
	for ; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")
		if line == "" {
			i++
			break
		}
		if colon := strings.Index(line, ":"); colon >= 0 {
			key := line[:colon]
			value := line[colon+1:]
			value = strings.ReplaceAll(value, "\\r", "\r")
			value = strings.ReplaceAll(value, "\\n", "\n")
			value = strings.ReplaceAll(value, "\\c", ":")
			value = strings.ReplaceAll(value, "\\\\", "\\")
			if _, exists := frame.Headers[key]; !exists { // first header wins (STOMP 1.2)
				frame.Headers[key] = value
			}
		} else {
			switch line {
			case "CONNECT":
				frame.Type = FrameConnect
			case "DISCONNECT":
				frame.Type = FrameDisconnect
			case "SEND":
				frame.Type = FrameSend
			case "SUBSCRIBE":
				frame.Type = FrameSubscribe
			case "UNSUBSCRIBE":
				frame.Type = FrameUnsubscribe
			case "ACK":
				frame.Type = FrameAck
			case "NACK":
				frame.Type = FrameNack
			case "CONNECTED":
				frame.Type = FrameConnected
			case "MESSAGE":
				frame.Type = FrameMessage
			case "RECEIPT":
				frame.Type = FrameReceipt
			case "ERROR":
				frame.Type = FrameError
			}
		}
	}
	if i > len(lines) {
		i = len(lines)
	}
	rest := strings.Join(lines[i:], "\n")
	if cl, ok := frame.Headers["content-length"]; ok {
		var n int
		if _, err := fmt.Sscanf(cl, "%d", &n); err == nil && n >= 0 && n <= len(rest) {
			frame.Body = rest[:n]
			return frame, true
		}
	}
	if idx := strings.IndexByte(rest, 0); idx >= 0 {
		frame.Body = rest[:idx]
	} else {
		frame.Body = rest
	}
	return frame, true
}

// encodeFrame serializes a frame for the wire.
func encodeFrame(frame Frame) string {
	var b strings.Builder
	switch frame.Type {
	case FrameConnect:
		b.WriteString("CONNECT\n")
	case FrameDisconnect:
		b.WriteString("DISCONNECT\n")
	case FrameSend:
		b.WriteString("SEND\n")
	case FrameSubscribe:
		b.WriteString("SUBSCRIBE\n")
	case FrameUnsubscribe:
		b.WriteString("UNSUBSCRIBE\n")
	case FrameAck:
		b.WriteString("ACK\n")
	case FrameNack:
		b.WriteString("NACK\n")
	case FrameConnected:
		b.WriteString("CONNECTED\n")
	case FrameMessage:
		b.WriteString("MESSAGE\n")
	case FrameReceipt:
		b.WriteString("RECEIPT\n")
	case FrameError:
		b.WriteString("ERROR\n")
	}
	for k, v := range frame.Headers {
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(escapeHeaderValue(v))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(frame.Body)
	b.WriteByte(0)
	return b.String()
}

func escapeHeaderValue(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\r", "\\r")
	v = strings.ReplaceAll(v, "\n", "\\n")
	v = strings.ReplaceAll(v, ":", "\\c")
	return v
}

func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func writeDeadline() time.Time {
	return time.Now().Add(30 * time.Second)
}
