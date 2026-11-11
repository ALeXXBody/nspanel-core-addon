// Package entities implements the light state machines — a faithful port of
// the C++ Light/HomeAssistantLight pair: config from entity_data JSON, HA
// event ingestion, service calls with optimistic mode, and protobuf state
// publishing to the manager MQTT topics.
package entities

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/golang/protobuf/proto"

	"nspanel-core-bridge/ha"
	"nspanel-core-bridge/mqtt"
	pb "nspanel-core-bridge/proto"
)

// LightType distinguishes ceiling vs table lights (room aggregation).
type LightType int

const (
	LightTypeTable LightType = iota
	LightTypeCeiling
)

// LightMode mirrors MQTT_MANAGER_LIGHT_MODE.
type LightMode int

const (
	LightModeDefault LightMode = iota // brightness / color-temp control
	LightModeRGB                      // HSV control
)

// Light is one light entity.
type Light struct {
	mu sync.RWMutex

	ID       int64
	Name     string
	RoomID   int64
	PageID   int64
	PageSlot int64

	Controller       string // "home_assistant"
	ControllerEntity string // HA entity_id (e.g. "light.kitchen")
	ControllerIsSwitch bool

	Type            LightType
	ControlledFromMainPage bool

	CanDim            bool
	CanColorTemp      bool
	CanRGB            bool

	// Current state (confirmed by HA, or optimistic)
	CurrentState       bool
	CurrentBrightness  int
	CurrentColorTemp   int // kelvin
	CurrentHue         int
	CurrentSaturation  int
	CurrentMode        LightMode

	// Requested state (what we asked HA for)
	RequestedState       bool
	RequestedBrightness  int
	RequestedColorTemp   int
	RequestedHue         int
	RequestedSaturation  int
	RequestedMode        LightMode

	// last published protobuf state (skip identical publishes)
	lastPublished []byte

	onChanged []func(*Light)
}

// AttachChangedCallback registers a callback fired when the light's state
// changes (used by room aggregation to republish room status).
func (l *Light) AttachChangedCallback(cb func(*Light)) {
	l.mu.Lock()
	l.onChanged = append(l.onChanged, cb)
	l.mu.Unlock()
}

func (l *Light) fireChanged() {
	l.mu.RLock()
	cbs := append([]func(*Light){}, l.onChanged...)
	l.mu.RUnlock()
	for _, cb := range cbs {
		cb(l)
	}
}

// LoadConfig populates the light from a web_entity row's entity_data JSON.
func (l *Light) LoadConfig(e EntityRow, colorTempMin, colorTempMax int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.ID = e.ID
	l.Name = e.FriendlyName
	l.RoomID = e.RoomID
	l.PageID = e.EntitiesPageID
	l.PageSlot = e.RoomViewPosition

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(e.EntityData), &data); err != nil {
		log.Printf("[light %d] failed to parse entity_data: %v — using defaults", e.ID, err)
		data = map[string]interface{}{}
	}

	l.ControlledFromMainPage = jsonBool(data, "controlled_by_nspanel_main_page", true)
	if jsonBool(data, "is_ceiling_light", false) {
		l.Type = LightTypeCeiling
	} else {
		l.Type = LightTypeTable
	}

	controller := jsonString(data, "controller", "home_assistant")
	if controller != "home_assistant" {
		log.Printf("[light %d] unknown controller %q — defaulting to home_assistant", e.ID, controller)
		controller = "home_assistant"
	}
	l.Controller = controller
	l.ControllerEntity = jsonString(data, "home_assistant_name", "")

	l.CanDim = jsonBool(data, "can_dim", false)
	l.CanColorTemp = jsonBool(data, "can_color_temperature", false)
	l.CanRGB = jsonBool(data, "can_rgb", false)

	if l.CanDim {
		l.CurrentMode, l.RequestedMode = LightModeDefault, LightModeDefault
	} else if l.CanRGB {
		l.CurrentMode, l.RequestedMode = LightModeRGB, LightModeRGB
	} else {
		l.CurrentMode, l.RequestedMode = LightModeDefault, LightModeDefault
	}

	// Restore live state from the entity_data blob if the bridge wrote it before.
	l.CurrentBrightness = int(jsonFloat(data, "current_brightness", 0))
	l.CurrentColorTemp = int(jsonFloat(data, "current_color_temperature", float64(colorTempMin)))
	l.CurrentHue = int(jsonFloat(data, "current_hue", 0))
	l.CurrentSaturation = int(jsonFloat(data, "current_saturation", 0))
	l.CurrentState = jsonBool(data, "current_state", false)
	l.RequestedState = l.CurrentState
	l.RequestedBrightness = l.CurrentBrightness
	l.RequestedColorTemp = l.CurrentColorTemp
	l.RequestedHue = l.CurrentHue
	l.RequestedSaturation = l.CurrentSaturation
}

// PersistState writes the live-state fields back into the entity_data JSON
// blob (same fields the C++ Entity::set_entity_data_json flow maintains).
func (l *Light) PersistState() {
	l.mu.Lock()
	defer l.mu.Unlock()
	data := map[string]interface{}{
		"controller":                   l.Controller,
		"home_assistant_name":          l.ControllerEntity,
		"can_dim":                      l.CanDim,
		"can_color_temperature":        l.CanColorTemp,
		"can_rgb":                      l.CanRGB,
		"is_ceiling_light":             l.Type == LightTypeCeiling,
		"controlled_by_nspanel_main_page": l.ControlledFromMainPage,
		"current_state":                l.CurrentState,
		"current_brightness":           l.CurrentBrightness,
		"current_color_temperature":    l.CurrentColorTemp,
		"current_hue":                  l.CurrentHue,
		"current_saturation":           l.CurrentSaturation,
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	l.persistEntityData(string(raw))
}

// persistEntityData is overridable in tests; production wires db.UpdateEntityData.
var persistEntityDataImpl = func(light *Light, data string) {}

func (l *Light) persistEntityData(data string) {
	persistEntityDataImpl(l, data)
}

// SetPersistImpl wires the DB write-back (called once at startup).
func SetPersistImpl(fn func(light *Light, data string)) {
	persistEntityDataImpl = fn
}

// --- commands (same semantics as the C++ Light) ---

func (l *Light) TurnOn(sendUpdate bool) {
	l.mu.Lock()
	l.RequestedState = true
	l.mu.Unlock()
	if sendUpdate {
		l.SendStateToController()
	}
	l.fireChanged()
}

func (l *Light) TurnOff(sendUpdate bool) {
	l.mu.Lock()
	l.RequestedState = false
	l.mu.Unlock()
	if sendUpdate {
		l.SendStateToController()
	}
	l.fireChanged()
}

func (l *Light) GetState() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.CurrentState
}

func (l *Light) SetBrightness(brightness int, sendUpdate bool) {
	if brightness < 0 {
		brightness = 0
	} else if brightness > 100 {
		brightness = 100
	}
	l.mu.Lock()
	l.RequestedBrightness = brightness
	l.mu.Unlock()
	if sendUpdate {
		l.SendStateToController()
	}
	l.fireChanged()
}

func (l *Light) SetColorTemperature(kelvin int, sendUpdate bool) {
	l.mu.Lock()
	l.RequestedMode = LightModeDefault
	l.RequestedColorTemp = kelvin
	l.mu.Unlock()
	if sendUpdate {
		l.SendStateToController()
	}
	l.fireChanged()
}

func (l *Light) SetHue(hue int, sendUpdate bool) {
	l.mu.Lock()
	l.RequestedMode = LightModeRGB
	l.RequestedHue = hue
	l.mu.Unlock()
	if sendUpdate {
		l.SendStateToController()
	}
	l.fireChanged()
}

func (l *Light) SetSaturation(sat int, sendUpdate bool) {
	l.mu.Lock()
	l.RequestedMode = LightModeRGB
	l.RequestedSaturation = sat
	l.mu.Unlock()
	if sendUpdate {
		l.SendStateToController()
	}
	l.fireChanged()
}

func (l *Light) SetHSB(hue int, sat int, brightness int, sendUpdate bool) {
	l.mu.Lock()
	l.RequestedMode = LightModeRGB
	l.RequestedHue = hue
	l.RequestedSaturation = sat
	l.RequestedBrightness = brightness
	l.mu.Unlock()
	if sendUpdate {
		l.SendStateToController()
	}
	l.fireChanged()
}

// Snapshot returns a consistent copy of the light state.
type LightSnapshot struct {
	ID              int64
	Name            string
	RoomID          int64
	Type            LightType
	State           bool
	Brightness      int
	ColorTemp       int
	Hue             int
	Saturation      int
	Mode            LightMode
	CanDim          bool
	CanColorTemp    bool
	CanRGB          bool
	ControlledFromMainPage bool
	PageID          int64
	PageSlot        int64
}

func (l *Light) Snapshot() LightSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return LightSnapshot{
		ID: l.ID, Name: l.Name, RoomID: l.RoomID, Type: l.Type,
		State: l.CurrentState, Brightness: l.CurrentBrightness,
		ColorTemp: l.CurrentColorTemp, Hue: l.CurrentHue, Saturation: l.CurrentSaturation,
		Mode: l.CurrentMode, CanDim: l.CanDim, CanColorTemp: l.CanColorTemp, CanRGB: l.CanRGB,
		ControlledFromMainPage: l.ControlledFromMainPage, PageID: l.PageID, PageSlot: l.PageSlot,
	}
}

// HAEvent ingests a Home Assistant state_changed event for this light —
// the exact attribute mapping from HomeAssistantLight::home_assistant_event_callback.
func (l *Light) HAEvent(data map[string]interface{}) {
	event, _ := data["event"].(map[string]interface{})
	if event == nil {
		return
	}
	if et, _ := event["event_type"].(string); et != "state_changed" {
		return
	}
	ed, _ := event["data"].(map[string]interface{})
	if ed == nil {
		return
	}
	if entityID, _ := ed["entity_id"].(string); entityID != l.ControllerEntity {
		return
	}
	newState, _ := ed["new_state"].(map[string]interface{})
	if newState == nil {
		return
	}
	attrs, _ := newState["attributes"].(map[string]interface{})
	if attrs == nil {
		attrs = map[string]interface{}{}
	}

	l.mu.Lock()
	changed := false
	state, _ := newState["state"].(string)

	switch state {
	case "on":
		if !l.CurrentState {
			changed = true
		}
		l.CurrentState = true
		l.RequestedState = true
		if l.CanDim {
			if b, ok := attrs["brightness"]; ok && b != nil {
				newBrightness := int(haToPct(b))
				if newBrightness != l.CurrentBrightness {
					changed = true
				}
				l.CurrentBrightness = newBrightness
				l.RequestedBrightness = newBrightness
			} else {
				log.Printf("[light %d] dimmable but no brightness in update — using 100%%", l.ID)
				if l.CurrentBrightness != 100 {
					changed = true
				}
				l.CurrentBrightness = 100
				l.RequestedBrightness = 100
			}
		} else {
			if l.CurrentState {
				changed = true
			}
			l.CurrentBrightness = 100
			l.RequestedBrightness = 100
		}
	case "off":
		if l.CurrentState {
			changed = true
		}
		l.CurrentState = false
		l.RequestedState = false
	default:
		log.Printf("[light %d] unknown entity state: %q", l.ID, state)
	}

	if colorMode, ok := attrs["color_mode"].(string); ok {
		switch {
		case l.CanColorTemp && colorMode == "color_temp":
			l.CurrentMode, l.RequestedMode = LightModeDefault, LightModeDefault
			if k, ok := attrs["color_temp_kelvin"]; ok && k != nil {
				kelvin := int(jsonNumber(k))
				if kelvin != l.CurrentColorTemp {
					changed = true
				}
				l.CurrentColorTemp = kelvin
				l.RequestedColorTemp = kelvin
			}
		case colorMode == "onoff":
			// no color support
		case l.CanRGB:
			l.CurrentMode, l.RequestedMode = LightModeRGB, LightModeRGB
			if hs, ok := attrs["hs_color"].([]interface{}); ok && len(hs) == 2 {
				hue := int(jsonNumber(hs[0]))
				sat := int(jsonNumber(hs[1]))
				if hue != l.CurrentHue || sat != l.CurrentSaturation {
					changed = true
				}
				l.CurrentHue, l.RequestedHue = hue, hue
				l.CurrentSaturation, l.RequestedSaturation = sat, sat
			}
		}
	}
	l.mu.Unlock()

	if changed {
		l.PersistState()
		l.SendStateToNSPanel()
		l.fireChanged()
	}
}

// SendStateToController issues the HA call_service request — the exact
// service-data logic from HomeAssistantLight::send_state_update_to_controller.
func (l *Light) SendStateToController() {
	l.mu.Lock()

	domain := "light"
	if l.ControllerIsSwitch {
		domain = "switch"
	}
	serviceData := map[string]interface{}{
		"type":   "call_service",
		"domain": domain,
		"target": map[string]interface{}{"entity_id": l.ControllerEntity},
	}

	optimistic := managerSettings.OptimisticMode

	if !l.ControllerIsSwitch && l.RequestedState {
		serviceData["service"] = "turn_on"
		if optimistic {
			l.CurrentState = true
		}
		if l.RequestedBrightness != l.CurrentBrightness {
			serviceData["service_data"] = map[string]interface{}{"brightness_pct": l.RequestedBrightness}
			if optimistic {
				l.CurrentBrightness = l.RequestedBrightness
			}
		}
		turnOnIsColorTemp := l.RequestedMode == LightModeDefault ||
			(!l.CurrentState && managerSettings.TurnOnBehaviour == "color_temp")
		if turnOnIsColorTemp {
			kelvin := l.RequestedColorTemp
			if kelvin <= 0 {
				kelvin = managerSettings.ColorTempMin
			}
			sd, _ := serviceData["service_data"].(map[string]interface{})
			sd["color_temp_kelvin"] = kelvin
			serviceData["service_data"] = sd
			if optimistic {
				l.CurrentColorTemp = l.RequestedColorTemp
				l.CurrentMode = LightModeDefault
			}
		}
		if l.RequestedMode == LightModeDefault && l.RequestedColorTemp != l.CurrentColorTemp {
			kelvin := l.RequestedColorTemp
			if kelvin <= 0 {
				kelvin = managerSettings.ColorTempMin
			}
			sd, _ := serviceData["service_data"].(map[string]interface{})
			sd["color_temp_kelvin"] = kelvin
			serviceData["service_data"] = sd
			if optimistic {
				l.CurrentColorTemp = l.RequestedColorTemp
				l.CurrentMode = LightModeDefault
			}
		} else if l.RequestedMode == LightModeRGB && (l.RequestedHue != l.CurrentHue || l.RequestedSaturation != l.CurrentSaturation) {
			sd, _ := serviceData["service_data"].(map[string]interface{})
			sd["hs_color"] = []interface{}{l.RequestedHue, l.RequestedSaturation}
			serviceData["service_data"] = sd
			if optimistic {
				l.CurrentHue = l.RequestedHue
				l.CurrentSaturation = l.RequestedSaturation
				l.CurrentMode = LightModeRGB
			}
		}
	} else if l.RequestedState {
		serviceData["service"] = "turn_on"
		if optimistic {
			l.CurrentState = true
		}
	} else {
		serviceData["service"] = "turn_off"
		if optimistic {
			l.CurrentState = false
		}
	}
	l.mu.Unlock()

	ha.SendJSON(serviceData)

	l.mu.RLock()
	opt := managerSettings.OptimisticMode
	l.mu.RUnlock()
	if opt {
		l.SendStateToNSPanel()
		l.fireChanged()
	}
}

// SendStateToNSPanel publishes NSPanelEntityState protobuf to the manager
// MQTT topic, skipping identical payloads (mirror of MessageDifferencer use).
func (l *Light) SendStateToNSPanel() {
	snap := l.Snapshot()

	state := &pb.NSPanelEntityState{
		Entity: &pb.NSPanelEntityState_Light_{
			Light: &pb.NSPanelEntityState_Light{
			LightId:     int32(snap.ID),
			Name:        snap.Name,
			State:       snap.State,
			CanColorTemp: snap.CanColorTemp,
			CanColor:    snap.CanRGB,
			Brightness:  int32(snap.Brightness),
			Hue:         int32(snap.Hue),
				Saturation:  int32(snap.Saturation),
			},
		},
	}
	if snap.Mode == LightModeRGB {
		state.GetLight().CurrentLightMode = pb.NSPanelEntityState_Light_RGB
	} else {
		state.GetLight().CurrentLightMode = pb.NSPanelEntityState_Light_COLOR_TEMP
	}

	kelvinPct := float64(snap.ColorTemp-managerSettings.ColorTempMin) /
		float64(managerSettings.ColorTempMax-managerSettings.ColorTempMin) * 100
	if managerSettings.ReverseColorTemp {
		kelvinPct = 100 - kelvinPct
	}
	state.GetLight().ColorTemp = int32(kelvinPct)

	raw, err := proto.Marshal(state)
	if err != nil {
		log.Printf("[light %d] marshal failed: %v", snap.ID, err)
		return
	}
	l.mu.Lock()
	identical := l.lastPublished != nil && bytesEqual(l.lastPublished, raw)
	if !identical {
		l.lastPublished = raw
	}
	l.mu.Unlock()
	if identical {
		return
	}
	mqtt.Publish(mqtt.LightStateTopic(managerSettings.ManagerAddress, snap.ID), base64Encode(raw), true)
}

// --- manager settings bridge (set at startup) ---

var managerSettings struct {
	ManagerAddress   string
	OptimisticMode   bool
	TurnOnBehaviour  string
	ColorTempMin     int
	ColorTempMax     int
	ReverseColorTemp bool
}

// SetManagerSettings wires global settings needed by the entity layer.
func SetManagerSettings(managerAddress string, optimistic bool, turnOnBehaviour string, colorTempMin, colorTempMax int, reverseColorTemp bool) {
	managerSettings = struct {
		ManagerAddress   string
		OptimisticMode   bool
		TurnOnBehaviour  string
		ColorTempMin     int
		ColorTempMax     int
		ReverseColorTemp bool
	}{managerAddress, optimistic, turnOnBehaviour, colorTempMin, colorTempMax, reverseColorTemp}
}

// --- json helpers ---

func jsonString(data map[string]interface{}, key, def string) string {
	if v, ok := data[key].(string); ok && v != "" {
		return v
	}
	return def
}

func jsonBool(data map[string]interface{}, key string, def bool) bool {
	if v, ok := data[key].(bool); ok {
		return v
	}
	if v, ok := data[key].(string); ok {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func jsonNumber(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func jsonFloat(data map[string]interface{}, key string, def float64) float64 {
	if v, ok := data[key]; ok {
		if n := jsonNumber(v); n != 0 {
			return n
		}
	}
	return def
}

// haToPct converts HA's 0-255 brightness to 0-100%.
func haToPct(v interface{}) float64 {
	return jsonNumber(v) / 255.0 * 100
}

// EntityRow mirrors the fields the db package returns for web_entity rows.
type EntityRow struct {
	ID               int64
	EntityType       string
	FriendlyName     string
	EntitiesPageID   int64
	RoomViewPosition int64
	RoomID           int64
	EntityData       string
}

// bytesEqual avoids importing bytes for one call.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// base64Encode wraps base64 encoding.
func base64Encode(data []byte) string {
	return b64StdEncode(data)
}

// fmt import guard
var _ = fmt.Sprintf

func b64StdEncode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// jsonUnmarshal is a small wrapper to avoid importing json in each file.
func jsonUnmarshal(data string, target *map[string]interface{}) error {
	return json.Unmarshal([]byte(data), target)
}
