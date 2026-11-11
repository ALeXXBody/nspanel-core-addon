// Scene handling — port of HomeAssistantScene (scene.x / script.x activation
// with context variables) and NSPMScene (applies saved SceneLightState rows
// to lights; save writes current light states back to the DB).
package entities

import (
	"database/sql"
	"log"

	"nspanel-core-bridge/db"
	"nspanel-core-bridge/ha"
)

// Scene is one scene entity (HA-backed or NSPM-managed).
type Scene struct {
	ID           int64
	Name         string
	RoomID       sql.NullInt64
	SceneType    string // "home_assistant_scenario" | "nspanel" (NSPM)
	BackendName  string // HA entity id for HA scenes ("scene.x"/"script.x")
	PageID       int64
	IsGlobal     bool
}

// LoadScene builds a Scene from a web_scene row.
func LoadScene(row db.Scene, pageID int64) *Scene {
	s := &Scene{
		ID:          row.ID,
		Name:        row.FriendlyName,
		RoomID:      row.RoomID,
		SceneType:   row.SceneType,
		BackendName: row.BackendName,
		PageID:      pageID,
		IsGlobal:    !row.RoomID.Valid,
	}
	return s
}

// Activate runs the scene — dispatching to the HA or NSPM implementation.
func (s *Scene) Activate(triggeringRoomID *int64) {
	log.Printf("Activating scene %d::%s.", s.ID, s.Name)
	if s.SceneType == "nspanel" {
		s.activateNSPM()
		return
	}
	s.activateHA(triggeringRoomID)
}

// activateHA mirrors HomeAssistantScene::activate.
func (s *Scene) activateHA(triggeringRoomID *int64) {
	if len(s.BackendName) >= 6 && s.BackendName[:6] == "scene." {
		ha.SendJSON(map[string]interface{}{
			"type":   "call_service",
			"domain": "scene",
			"service": "turn_on",
			"target": map[string]interface{}{"entity_id": s.BackendName},
		})
		return
	}
	if len(s.BackendName) >= 7 && s.BackendName[:7] == "script." {
		context := map[string]interface{}{
			"scene_name": s.Name,
			"scene_id":   s.ID,
		}
		if triggeringRoomID != nil {
			if room := RoomName(*triggeringRoomID); room != "" {
				context["triggering_room_id"] = *triggeringRoomID
				context["triggering_room_name"] = room
			}
		}
		if !s.IsGlobal {
			if room := RoomName(s.RoomID.Int64); room != "" {
				context["scene_room_id"] = s.RoomID.Int64
				context["scene_room_name"] = room
			}
		}
		ha.SendJSON(map[string]interface{}{
			"type":   "call_service",
			"domain": "script",
			"service": "turn_on",
			"target": map[string]interface{}{"entity_id": s.BackendName},
			"service_data": map[string]interface{}{
				"variables": map[string]interface{}{"nspanelmanager": context},
			},
		})
		return
	}
	log.Printf("Got request to activate HA scene %d::%s but entity '%s' is not scene. or script.", s.ID, s.Name, s.BackendName)
}

// RoomName looks up a room's friendly name (registered at startup).
var RoomName = func(roomID int64) string { return "" }

// activateNSPM mirrors NSPMScene::activate — applies saved light states.
func (s *Scene) activateNSPM() {
	states, err := db.ListSceneLightStates(s.ID)
	if err != nil {
		log.Printf("Tried to activate scene %d::%s: failed to load states: %v", s.ID, s.Name, err)
		return
	}
	for _, ls := range states {
		light := GetRegistry().Light(ls.LightID)
		if light == nil {
			log.Printf("Tried to apply scene %d::%s state to light %d but no such light found.", s.ID, s.Name, ls.LightID)
			continue
		}
		if ls.LightLevel.Valid && ls.LightLevel.Int64 == 0 {
			light.TurnOff(true)
			continue
		}
		switch ls.ColorMode {
		case "dimmer":
			if ls.LightLevel.Valid && ls.LightLevel.Int64 <= 0 {
				light.TurnOff(true)
			} else {
				light.TurnOn(false)
				if ls.ColorTemperature.Valid {
					light.SetColorTemperature(int(ls.ColorTemperature.Int64), false)
				}
				light.SetBrightness(int(ls.LightLevel.Int64), true)
			}
		case "color":
			if ls.LightLevel.Valid && ls.LightLevel.Int64 <= 0 {
				light.TurnOff(true)
			} else {
				light.TurnOn(false)
				if ls.Hue.Valid {
					light.SetHue(int(ls.Hue.Int64), false)
				}
				if ls.Saturation.Valid {
					light.SetSaturation(int(ls.Saturation.Int64), false)
				}
				light.SetBrightness(int(ls.LightLevel.Int64), true)
			}
		default:
			log.Printf("Scene %d::%s: unknown color mode %q for light %d", s.ID, s.Name, ls.ColorMode, ls.LightID)
		}
	}
}

// SaveNSPM mirrors NSPMScene::save — replace saved states with the lights'
// current values.
func (s *Scene) SaveNSPM(lights []*Light) error {
	if err := db.DeleteSceneLightStates(s.ID); err != nil {
		return err
	}
	for _, light := range lights {
		snap := light.Snapshot()
		colorMode := "dimmer"
		if snap.Mode == LightModeRGB {
			colorMode = "color"
		}
		if err := db.InsertSceneLightState(s.ID, snap.ID, int64(snap.Brightness), colorMode, int64(snap.ColorTemp), int64(snap.Hue), int64(snap.Saturation)); err != nil {
			return err
		}
	}
	return nil
}
