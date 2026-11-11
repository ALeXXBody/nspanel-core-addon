// Package mqtt wraps the MQTT client and provides the topic helpers that
// exactly mirror the C++ MQTT_Manager.
package mqtt

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client pahomqtt.Client
	mu     sync.RWMutex
	prefix string // "nspanel"
}

var (
	global      *Client
	globalMu    sync.RWMutex
	messageHandlers []struct {
		topic   string
		handler pahomqtt.MessageHandler
	}
)

// Connect establishes the MQTT connection and returns the client.
func Connect(server string, port int, username, password string) (*Client, error) {
	opts := pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", strings.TrimPrefix(strings.TrimPrefix(server, "http://"), "https://"), port)).
		SetClientID(fmt.Sprintf("nspanel-core-bridge-%d", time.Now().UnixNano())).
		SetUsername(username).
		SetPassword(password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetOrderMatters(false)

	c := pahomqtt.NewClient(opts)
	token := c.Connect()
	if !token.WaitTimeout(30 * time.Second) {
		return nil, fmt.Errorf("mqtt connect timeout to %s:%d", server, port)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect: %w", err)
	}

	cl := &Client{client: c, prefix: "nspanel"}
	globalMu.Lock()
	global = cl
	globalMu.Unlock()
	return cl, nil
}

// Global returns the connected client (nil before Connect).
func Global() *Client {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// Publish sends a payload to a topic, optionally retained. It is safe to call
// before/after disconnects — paho queues when reconnecting.
func (c *Client) Publish(topic string, payload string, retained bool) {
	if c == nil {
		return
	}
	token := c.client.Publish(topic, 0, retained, payload)
	go func() {
		if !token.WaitTimeout(10 * time.Second) {
			log.Printf("[mqtt] publish timeout on %s", topic)
		}
	}()
}

// PublishGlobal publishes via the global client, ignoring nil (pre-connect).
func PublishGlobal(topic string, payload string, retained bool) {
	Publish(topic, payload, retained)
}

// Publish publishes on the global client; no-op before connect.
func Publish(topic string, payload string, retained bool) {
	globalMu.RLock()
	c := global
	globalMu.RUnlock()
	if c != nil {
		c.Publish(topic, payload, retained)
	}
}

// Subscribe registers a handler for a topic filter.
func (c *Client) Subscribe(topic string, handler pahomqtt.MessageHandler) error {
	token := c.client.Subscribe(topic, 0, handler)
	if !token.WaitTimeout(30 * time.Second) {
		return fmt.Errorf("subscribe timeout on %s", topic)
	}
	return token.Error()
}

// SubscribeGlobal subscribes on the global client once connected.
func SubscribeGlobal(topic string, handler pahomqtt.MessageHandler) error {
	globalMu.RLock()
	c := global
	globalMu.RUnlock()
	if c == nil {
		return fmt.Errorf("mqtt not connected")
	}
	return c.Subscribe(topic, handler)
}

// Topic builders — exact mirrors of the C++ format strings.

func ManagerPrefix(managerAddress string) string {
	return fmt.Sprintf("nspanel/mqttmanager_%s", managerAddress)
}

func TimeTopic(managerAddress string) string       { return ManagerPrefix(managerAddress) + "/status/time" }
func AmpmTopic(managerAddress string) string       { return ManagerPrefix(managerAddress) + "/status/ampm" }
func DateTopic(managerAddress string) string       { return ManagerPrefix(managerAddress) + "/status/date" }
func WeatherTopic(managerAddress string) string    { return ManagerPrefix(managerAddress) + "/status/weather" }
func StatusTopic(managerAddress string) string     { return ManagerPrefix(managerAddress) + "/status/status" }
func ManagerCommandTopic() string                  { return "nspanel/mqttmanager/command" }
func PanelStatusTopic(mac string) string           { return fmt.Sprintf("nspanel/%s/status", mac) }
func PanelCommandTopic(mac string) string          { return fmt.Sprintf("nspanel/%s/command", mac) }
func PanelStatusReportTopic(mac string) string     { return fmt.Sprintf("nspanel/%s/status_report", mac) }
func PanelLogTopic(mac string) string              { return fmt.Sprintf("nspanel/%s/log", mac) }
func PanelTemperatureTopic(mac string) string      { return fmt.Sprintf("nspanel/%s/temperature", mac) }
func PanelHumidityTopic(mac string) string         { return fmt.Sprintf("nspanel/%s/humidity", mac) }
func PanelPressureTopic(mac string) string         { return fmt.Sprintf("nspanel/%s/pressure", mac) }
func PanelScreenStateTopic(mac string) string      { return fmt.Sprintf("nspanel/%s/screen_state", mac) }
func PanelScreenCmdTopic(mac string) string        { return fmt.Sprintf("nspanel/%s/screen_cmd", mac) }
func PanelScreensaverModeStateTopic(mac string) string {
	return fmt.Sprintf("nspanel/%s/screensaver_mode_state", mac)
}
func PanelScreensaverModeCmdTopic(mac string) string {
	return fmt.Sprintf("nspanel/%s/screensaver_mode_cmd", mac)
}
func PanelRelay1StateTopic(mac string) string { return fmt.Sprintf("nspanel/%s/relay1_state", mac) }
func PanelRelay1CmdTopic(mac string) string   { return fmt.Sprintf("nspanel/%s/relay1_cmd", mac) }
func PanelRelay2StateTopic(mac string) string { return fmt.Sprintf("nspanel/%s/relay2_state", mac) }
func PanelRelay2CmdTopic(mac string) string   { return fmt.Sprintf("nspanel/%s/relay2_cmd", mac) }
func PanelHomePageAllTopic(mac string) string { return fmt.Sprintf("nspanel/%s/home_page_all", mac) }

func RoomStateTopic(managerAddress string, roomID int64) string {
	return fmt.Sprintf("%s/room/%d/state", ManagerPrefix(managerAddress), roomID)
}

func RoomTemperatureStateTopic(managerAddress string, roomID int64) string {
	return fmt.Sprintf("%s/room/%d/temperature_state", ManagerPrefix(managerAddress), roomID)
}

func EntityPageStateTopic(managerAddress string, pageID int64) string {
	return fmt.Sprintf("%s/entity_pages/%d/state", ManagerPrefix(managerAddress), pageID)
}

func LightStateTopic(managerAddress string, lightID int64) string {
	return fmt.Sprintf("%s/entities/lights/%d/state", ManagerPrefix(managerAddress), lightID)
}

func ThermostatStateTopic(managerAddress string, thermostatID int64) string {
	return fmt.Sprintf("%s/entities/thermostats/%d/state", ManagerPrefix(managerAddress), thermostatID)
}

func AllRoomsStatusTopic(managerAddress string) string {
	return ManagerPrefix(managerAddress) + "/all_rooms_status"
}

func ConfigReloadTopic() string { return "nspanel/config/reload" }
