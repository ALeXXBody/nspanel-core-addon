// Package ha implements a client for the Home Assistant WebSocket API —
// the same integration the C++ HomeAssistantManager provides: auth with a
// long-lived access token, subscribe to state_changed events, fetch all
// states at startup and dispatch entity callbacks.
package ha

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client is the Home Assistant WebSocket API client.
type Client struct {
	mu       sync.RWMutex
	token    string
	address  string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	nextID   int64
	authed   bool
	connected bool

	allStatesID int64
	observers   map[string][]func(event map[string]interface{})
	observerMu  sync.RWMutex

	stopCh chan struct{}
}

// NewClient creates a client for the given address/token.
func NewClient(address, token string) *Client {
	return &Client{
		address:   address,
		token:     token,
		nextID:    1,
		observers: make(map[string][]func(map[string]interface{})),
		stopCh:    make(chan struct{}),
	}
}

// SetCredentials updates credentials for the next (re)connect.
func (c *Client) SetCredentials(address, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.address = address
	c.token = token
}

// Connect dials the HA WebSocket API, authenticates, subscribes to
// state_changed events and requests all states. It then reads messages in a
// loop until stopped; auto-reconnects with backoff.
func (c *Client) Connect() {
	go func() {
		backoff := 5 * time.Second
		for {
			select {
			case <-c.stopCh:
				return
			default:
			}
			if err := c.connectOnce(); err != nil {
				log.Printf("[ha] connect failed: %v — retrying in %v", err, backoff)
			}
			select {
			case <-c.stopCh:
				return
			case <-time.After(backoff):
			}
		}
	}()
}

func (c *Client) connectOnce() error {
	wsURL, err := c.websocketURL()
	if err != nil {
		return err
	}
	log.Printf("[ha] Will connect to Home Assistant websocket at %s", wsURL)

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		c.setConnected(false)
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.nextID = 1
	c.mu.Unlock()

	// Read loop.
	readErr := make(chan error, 1)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			c.processMessage(string(data))
		}
	}()

	select {
	case <-c.stopCh:
		_ = conn.Close()
		return nil
	case err := <-readErr:
		c.setConnected(false)
		return fmt.Errorf("connection lost: %w", err)
	}
}

func (c *Client) websocketURL() (string, error) {
	c.mu.RLock()
	addr, token := c.address, c.token
	c.mu.RUnlock()
	if addr == "" || token == "" {
		return "", fmt.Errorf("home assistant address or token not configured")
	}
	if strings.HasPrefix(addr, "https://") {
		addr = "wss://" + strings.TrimPrefix(addr, "https://")
	} else if strings.HasPrefix(addr, "http://") {
		addr = "ws://" + strings.TrimPrefix(addr, "http://")
	} else if !strings.HasPrefix(addr, "ws") {
		addr = "ws://" + addr
	}
	u, err := url.Parse(strings.TrimRight(addr, "/") + "/api/websocket")
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// processMessage handles the HA WebSocket message lifecycle.
func (c *Client) processMessage(message string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(message), &data); err != nil {
		log.Printf("[ha] failed to parse message: %v", err)
		return
	}
	msgType, _ := data["type"].(string)

	switch msgType {
	case "auth_required":
		log.Printf("[ha] authenticating to Home Assistant")
		c.sendRaw(map[string]interface{}{
			"type":         "auth",
			"access_token": c.token,
		})
	case "auth_ok":
		log.Printf("[ha] Successfully authenticated to Home Assistant websocket API.")
		c.mu.Lock()
		c.authed = true
		c.mu.Unlock()
		// Subscribe to all state changes.
		c.SendJSON(map[string]interface{}{
			"type":      "subscribe_events",
			"event_type": "state_changed",
		})
		c.requestAllStates()
	case "auth_invalid":
		log.Printf("[ha] Authentication FAILED — check the Home Assistant token.")
		c.setConnected(false)
	case "result":
		success, _ := data["success"].(bool)
		if !success {
			log.Printf("[ha] Command failed: %s", message)
		}
		id, _ := data["id"].(float64)
		c.mu.RLock()
		allID := c.allStatesID
		c.mu.RUnlock()
		if int64(id) == allID {
			// Convert each state into a state_changed-shaped event so the
			// entity layer handles both identically (same as the C++ code).
			result, _ := data["result"].([]interface{})
			for _, entity := range result {
				entityData, ok := entity.(map[string]interface{})
				if !ok {
					continue
				}
				modified := map[string]interface{}{
					"event": map[string]interface{}{
						"data": map[string]interface{}{
							"new_state": entityData,
							"entity_id": entityData["entity_id"],
						},
						"event_type": "state_changed",
					},
				}
				c.dispatch(modified)
			}
		}
	case "event":
		c.dispatch(data)
	default:
		log.Printf("[ha] Got unhandled message: %s", message)
	}
}

// dispatch routes a state_changed event to registered entity observers.
func (c *Client) dispatch(event map[string]interface{}) {
	eventData, _ := event["event"].(map[string]interface{})
	if eventData == nil {
		return
	}
	eventType, _ := eventData["event_type"].(string)
	if eventType != "state_changed" {
		return
	}
	ed, _ := eventData["data"].(map[string]interface{})
	if ed == nil {
		return
	}
	entityID, _ := ed["entity_id"].(string)
	if entityID == "" {
		return
	}

	c.observerMu.RLock()
	cbs := append([]func(map[string]interface{}){}, c.observers[entityID]...)
	c.observerMu.RUnlock()
	for _, cb := range cbs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ha] observer panic for %s: %v", entityID, r)
				}
			}()
			cb(event)
		}()
	}
}

// AttachObserver registers a callback for an entity's state_changed events
// and immediately triggers a full state refresh (mirror of attach_event_observer).
func (c *Client) AttachObserver(entityID string, cb func(event map[string]interface{})) {
	c.observerMu.Lock()
	// Prevent duplicate registration (same as the C++ disconnect-then-connect).
	existing := c.observers[entityID]
	filtered := existing[:0]
	for _, f := range existing {
		_ = f
	}
	c.observers[entityID] = append(filtered, cb)
	c.observerMu.Unlock()
	c.requestAllStates()
}

// DetachObserver removes all callbacks for an entity.
func (c *Client) DetachObserver(entityID string) {
	c.observerMu.Lock()
	delete(c.observers, entityID)
	c.observerMu.Unlock()
}

// SendJSON sends a command payload with an auto-incremented id.
func (c *Client) SendJSON(payload map[string]interface{}) {
	c.mu.Lock()
	payload["id"] = c.nextID
	c.nextID++
	c.mu.Unlock()
	c.sendRaw(payload)
}

// CallService invokes a Home Assistant service.
func (c *Client) CallService(domain, service, entityID string, serviceData map[string]interface{}) {
	payload := map[string]interface{}{
		"type":           "call_service",
		"domain":         domain,
		"service":        service,
		"target":         map[string]interface{}{"entity_id": entityID},
	}
	if serviceData != nil {
		payload["service_data"] = serviceData
	}
	c.SendJSON(payload)
}

// requestAllStates fetches all states; results are re-dispatched as
// state_changed-shaped events.
func (c *Client) requestAllStates() {
	c.mu.Lock()
	c.allStatesID = c.nextID
	c.nextID++
	c.mu.Unlock()
	c.SendJSON(map[string]interface{}{"type": "get_states"})
}

// sendRaw serializes and sends, guarding writes with a mutex.
func (c *Client) sendRaw(payload map[string]interface{}) {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()
	if conn == nil || !connected {
		log.Printf("[ha] Cannot send — not connected")
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ha] marshal failed: %v", err)
		return
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("[ha] send failed: %v", err)
	}
}

// setConnected flips the connected flag; when connected, mark authed flow done.
func (c *Client) setConnected(v bool) {
	c.mu.Lock()
	c.connected = v
	if v {
		c.authed = true
	}
	c.mu.Unlock()
}

// Connected reports connection status.
func (c *Client) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.authed
}

// IsHTTPS reports whether the address is https (for ws scheme selection).
func IsHTTPS(address string) bool {
	return strings.HasPrefix(strings.ToLower(address), "https://")
}

// Ensure http import is used even if unused paths change.
var _ = http.StatusOK

// Global client handle used by packages that don't own the client
// (mirrors the C++ static HomeAssistantManager::send_json).
var (
	globalClient   *Client
	globalClientMu sync.RWMutex
)

// SetGlobal registers the process-wide client.
func SetGlobal(c *Client) {
	globalClientMu.Lock()
	globalClient = c
	globalClientMu.Unlock()
}

// SendJSON sends via the global client (no-op if not set).
func SendJSON(payload map[string]interface{}) {
	globalClientMu.RLock()
	c := globalClient
	globalClientMu.RUnlock()
	if c != nil {
		c.SendJSON(payload)
	}
}

// Global returns the process-wide client.
func Global() *Client {
	globalClientMu.RLock()
	defer globalClientMu.RUnlock()
	return globalClient
}
