// Thermostat state machine — port of the C++ ThermostatEntity /
// HomeAssistantThermostat pair. Options (hvac/fan/preset/swing modes) come
// from the entity_data JSON written by Django ({value,icon,label} lists).
package entities

import (
	"log"
	"math"
	"sync"

	"github.com/golang/protobuf/proto"

	"nspanel-core-bridge/ha"
	"nspanel-core-bridge/mqtt"
	pb "nspanel-core-bridge/proto"
)

// ThermostatOption mirrors ThermostatOptionHolder.
type ThermostatOption struct {
	Value string
	Icon  string
	Label string
}

// Thermostat is one climate entity.
type Thermostat struct {
	mu sync.RWMutex

	ID       int64
	Name     string
	RoomID   int64
	PageID   int64
	PageSlot int64

	Controller       string
	ControllerEntity string

	StepSize float64

	SupportedModes   []ThermostatOption
	SupportedFan     []ThermostatOption
	SupportedPresets []ThermostatOption
	SupportedSwing   []ThermostatOption
	SupportedSwingH  []ThermostatOption

	CurrentMode   ThermostatOption
	CurrentFan    ThermostatOption
	CurrentPreset ThermostatOption
	CurrentSwing  ThermostatOption
	CurrentSwingH ThermostatOption

	CurrentTemperature float64
	HasCurrentTemp     bool // comes from the room sensor, not HA
	RequestedTemperature float64

	RequestedMode   ThermostatOption
	RequestedFan    ThermostatOption
	RequestedPreset ThermostatOption
	RequestedSwing  ThermostatOption
	RequestedSwingH ThermostatOption

	lastPublished []byte
	onChanged     []func(*Thermostat)
}

// AttachChangedCallback registers a change callback (room state updates).
func (t *Thermostat) AttachChangedCallback(cb func(*Thermostat)) {
	t.mu.Lock()
	t.onChanged = append(t.onChanged, cb)
	t.mu.Unlock()
}

func (t *Thermostat) fireChanged() {
	t.mu.RLock()
	cbs := append([]func(*Thermostat){}, t.onChanged...)
	t.mu.RUnlock()
	for _, cb := range cbs {
		cb(t)
	}
}

// LoadConfig populates the thermostat from entity_data JSON.
func (t *Thermostat) LoadConfig(e EntityRow) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.ID = e.ID
	t.Name = e.FriendlyName
	t.RoomID = e.RoomID
	t.PageID = e.EntitiesPageID
	t.PageSlot = e.RoomViewPosition

	data := map[string]interface{}{}
	_ = jsonUnmarshal(e.EntityData, &data)

	t.Controller = jsonString(data, "controller", "home_assistant")
	t.ControllerEntity = jsonString(data, "home_assistant_name", "")
	t.StepSize = jsonFloat(data, "step_size", 0.5)

	t.SupportedFan = loadOptions(data, "fan_modes", &t.CurrentFan, &t.RequestedFan)
	t.SupportedModes = loadOptions(data, "hvac_modes", &t.CurrentMode, &t.RequestedMode)
	t.SupportedPresets = loadOptions(data, "preset_modes", &t.CurrentPreset, &t.RequestedPreset)
	t.SupportedSwing = loadOptions(data, "swing_modes", &t.CurrentSwing, &t.RequestedSwing)
	t.SupportedSwingH = loadOptions(data, "swing_horizontal_modes", &t.CurrentSwingH, &t.RequestedSwingH)
}

// loadOptions parses a {value,icon,label} list, defaulting current/requested
// to the first option when unset (same fallback as the C++ code).
func loadOptions(data map[string]interface{}, key string, current, requested *ThermostatOption) []ThermostatOption {
	list, _ := data[key].([]interface{})
	out := make([]ThermostatOption, 0, len(list))
	for _, raw := range list {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		opt := ThermostatOption{
			Value: jsonString(m, "value", ""),
			Icon:  jsonString(m, "icon", ""),
			Label: jsonString(m, "label", ""),
		}
		if opt.Label == "" {
			opt.Label = opt.Value
		}
		out = append(out, opt)
	}
	if len(out) > 0 && current.Value == "" {
		*current = out[0]
		*requested = out[0]
	}
	return out
}

// HAEvent ingests a HA state_changed event for this thermostat.
func (t *Thermostat) HAEvent(data map[string]interface{}) {
	event, _ := data["event"].(map[string]interface{})
	if event == nil {
		return
	}
	ed, _ := event["data"].(map[string]interface{})
	if ed == nil {
		return
	}
	if entityID, _ := ed["entity_id"].(string); entityID != t.ControllerEntity {
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

	t.mu.Lock()
	changed := false

	if temp, ok := attrs["temperature"]; ok && temp != nil {
		temperature := jsonNumber(temp)
		if temperature != t.CurrentTemperature {
			t.CurrentTemperature = temperature
			t.RequestedTemperature = temperature
			changed = true
		}
	} else {
		log.Printf("[thermostat %d] update has no valid set temperature", t.ID)
	}

	if applyOptionChange(attrs, "hvac_mode", "hvac_modes", t.SupportedModes, &t.CurrentMode, &t.RequestedMode) {
		changed = true
	}
	if applyOptionChange(attrs, "fan_mode", "fan_modes", t.SupportedFan, &t.CurrentFan, &t.RequestedFan) {
		changed = true
	}
	if applyOptionChange(attrs, "preset_mode", "preset_modes", t.SupportedPresets, &t.CurrentPreset, &t.RequestedPreset) {
		changed = true
	}
	if applyOptionChange(attrs, "swing_mode", "swing_modes", t.SupportedSwing, &t.CurrentSwing, &t.RequestedSwing) {
		changed = true
	}
	if applyOptionChange(attrs, "swing_horizontal_mode", "swing_horizontal_modes", t.SupportedSwingH, &t.CurrentSwingH, &t.RequestedSwingH) {
		changed = true
	}
	t.mu.Unlock()

	if changed {
		t.SendStateToNSPanel()
		t.fireChanged()
	}
}

// applyOptionChange updates current/requested option when HA reports one.
func applyOptionChange(attrs map[string]interface{}, attrKey, listKey string, supported []ThermostatOption, current, requested *ThermostatOption) bool {
	v, ok := attrs[attrKey]
	if !ok || v == nil {
		return false
	}
	value, _ := v.(string)
	for _, opt := range supported {
		if opt.Value == value {
			if current.Value != value {
				*current = opt
				*requested = opt
				return true
			}
			return false
		}
	}
	_ = listKey
	return false
}

// SetTemperature requests a new set temperature (0.5 step rounding applied
// upstream by the panel slider; here we pass through).
func (t *Thermostat) SetTemperature(temperature float64) {
	t.mu.Lock()
	t.RequestedTemperature = temperature
	t.mu.Unlock()
	t.SendStateToController()
	t.fireChanged()
}

// SetOption requests a mode/fan/preset/swing change by option value.
func (t *Thermostat) SetOption(kind, value string) {
	t.mu.Lock()
	switch kind {
	case "mode":
		if opt := findOption(t.SupportedModes, value); opt != nil {
			t.RequestedMode = *opt
		}
	case "fan":
		if opt := findOption(t.SupportedFan, value); opt != nil {
			t.RequestedFan = *opt
		}
	case "preset":
		if opt := findOption(t.SupportedPresets, value); opt != nil {
			t.RequestedPreset = *opt
		}
	case "swing":
		if opt := findOption(t.SupportedSwing, value); opt != nil {
			t.RequestedSwing = *opt
		}
	case "swingh":
		if opt := findOption(t.SupportedSwingH, value); opt != nil {
			t.RequestedSwingH = *opt
		}
	}
	t.mu.Unlock()
	t.SendStateToController()
	t.fireChanged()
}

func findOption(list []ThermostatOption, value string) *ThermostatOption {
	for i := range list {
		if list[i].Value == value {
			return &list[i]
		}
	}
	return nil
}

// SendStateToController issues HA climate service calls for any changed
// option plus set_temperature (same flow as the C++ thermostat).
func (t *Thermostat) SendStateToController() {
	t.mu.Lock()
	defer t.mu.Unlock()

	target := map[string]interface{}{"entity_id": t.ControllerEntity}

	if t.RequestedMode != t.CurrentMode {
		ha.SendJSON(map[string]interface{}{
			"type": "call_service", "domain": "climate", "target": target,
			"service": "set_hvac_mode",
			"service_data": map[string]interface{}{"hvac_mode": t.RequestedMode.Value},
		})
		t.CurrentMode = t.RequestedMode
	}
	if t.RequestedFan != t.CurrentFan {
		ha.SendJSON(map[string]interface{}{
			"type": "call_service", "domain": "climate", "target": target,
			"service": "set_fan_mode",
			"service_data": map[string]interface{}{"fan_mode": t.RequestedFan.Value},
		})
		t.CurrentFan = t.RequestedFan
	}
	if t.RequestedPreset != t.CurrentPreset {
		ha.SendJSON(map[string]interface{}{
			"type": "call_service", "domain": "climate", "target": target,
			"service": "set_preset_mode",
			"service_data": map[string]interface{}{"preset_mode": t.RequestedPreset.Value},
		})
		t.CurrentPreset = t.RequestedPreset
	}
	if t.RequestedSwing != t.CurrentSwing {
		ha.SendJSON(map[string]interface{}{
			"type": "call_service", "domain": "climate", "target": target,
			"service": "set_swing_mode",
			"service_data": map[string]interface{}{"swing_mode": t.RequestedSwing.Value},
		})
		t.CurrentSwing = t.RequestedSwing
	}
	if t.RequestedSwingH != t.CurrentSwingH {
		ha.SendJSON(map[string]interface{}{
			"type": "call_service", "domain": "climate", "target": target,
			"service": "set_swing_horizontal_mode",
			"service_data": map[string]interface{}{"swing_horizontal_mode": t.RequestedSwingH.Value},
		})
		t.CurrentSwingH = t.RequestedSwingH
	}
	if math.Abs(t.RequestedTemperature-t.CurrentTemperature) > 0.01 {
		ha.SendJSON(map[string]interface{}{
			"type": "call_service", "domain": "climate", "target": target,
			"service": "set_temperature",
			"service_data": map[string]interface{}{"temperature": t.RequestedTemperature},
		})
		t.CurrentTemperature = t.RequestedTemperature
	}
}

// SetRoomTemperature feeds the room-sensor temperature used in the panel UI.
func (t *Thermostat) SetRoomTemperature(temp float64, hasTemp bool) {
	t.mu.Lock()
	t.CurrentTemperature = temp
	t.HasCurrentTemp = hasTemp
	t.mu.Unlock()
}

// SendStateToNSPanel publishes NSPanelEntityState(Thermostat) — mirror of
// ThermostatEntity::send_state_update_to_nspanel including option lists.
func (t *Thermostat) SendStateToNSPanel() {
	t.mu.RLock()
	th := &pb.NSPanelEntityState_Thermostat{
		ThermostatId:          int32(t.ID),
		Name:                  t.Name,
		SetTemperature:        float32(t.CurrentTemperature),
		StepSize:              float32(t.StepSize),
		HasCurrentTemperature: t.HasCurrentTemp,
	}
	if t.HasCurrentTemp {
		th.CurrentTemperature = float32(t.CurrentTemperature)
	}
	addOption := func(name string, current ThermostatOption, list []ThermostatOption) {
		if len(list) == 0 {
			return
		}
		opt := &pb.NSPanelEntityState_Thermostat_ThermostatOption{
			Name:         name,
			CurrentValue: current.Label,
			CurrentIcon:  current.Icon,
		}
		for _, m := range list {
			opt.Options = append(opt.Options, &pb.NSPanelEntityState_Thermostat_ThermostatOption_ThermostatOptionValue{
				Value: m.Label,
				Icon:  m.Icon,
			})
		}
		th.Options = append(th.Options, opt)
	}
	addOption("Mode", t.CurrentMode, t.SupportedModes)
	addOption(thermostatFanLabel, t.CurrentFan, t.SupportedFan)
	addOption(thermostatPresetLabel, t.CurrentPreset, t.SupportedPresets)
	addOption(thermostatSwingLabel, t.CurrentSwing, t.SupportedSwing)
	addOption(thermostatSwingHLabel, t.CurrentSwingH, t.SupportedSwingH)

	state := &pb.NSPanelEntityState{
		Entity: &pb.NSPanelEntityState_Thermostat_{Thermostat: th},
	}
	t.mu.RUnlock()

	raw, err := proto.Marshal(state)
	if err != nil {
		return
	}
	t.mu.Lock()
	identical := t.lastPublished != nil && bytesEqual(t.lastPublished, raw)
	if !identical {
		t.lastPublished = raw
	}
	t.mu.Unlock()
	if identical {
		return
	}
	mqtt.Publish(mqtt.ThermostatStateTopic(managerSettings.ManagerAddress, t.ID), base64Encode(raw), true)
}

// Labels used for the option groups (mirror of the C++ static labels).
var (
	thermostatFanLabel     = "Fan speed"
	thermostatPresetLabel  = "Presets"
	thermostatSwingLabel   = "Swing"
	thermostatSwingHLabel  = "Swing horizontal"
)
