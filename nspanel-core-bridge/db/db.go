// Package db provides direct access to the shared Django SQLite database
// (/data/nspanelmanager_db.sqlite3) — the same access pattern the C++ bridge
// uses via sqlite_orm. Django writes configuration; the bridge reads it and
// writes back live entity state and checksums.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

// Setting mirrors the web_settings table.
type Setting struct {
	ID    int64
	Name  string
	Value string
}

// NSPanelSetting mirrors web_nspanelsettingholder (per-panel overrides).
type NSPanelSetting struct {
	ID        int64
	Name      string
	Value     string
	NSPanelID int64
}

// NSPanel mirrors web_nspanel.
type NSPanel struct {
	ID                          int64
	MacAddress                  string
	FriendlyName                string
	Model                       string
	RoomID                      sql.NullInt64
	Version                     string
	Button1DetachedModeEntityID sql.NullInt64
	Button1Mode                 int64
	Button2DetachedModeEntityID sql.NullInt64
	Button2Mode                 int64
	Md5DataFile                 string
	Md5Firmware                 string
	Md5TftFile                  string
	Denied                      bool
	Accepted                    bool
}

// Room mirrors web_room.
type Room struct {
	ID              int64
	FriendlyName    string
	RoomTempProvider string
	RoomTempSensor  string
	DisplayOrder    int64
}

// Scene mirrors web_scene.
type Scene struct {
	ID               int64
	RoomID           sql.NullInt64
	FriendlyName     string
	SceneType        string
	BackendName      string
	RoomViewPosition int64
	EntitiesPageID   int64
}

// SceneLightState mirrors web_scenelightstate.
type SceneLightState struct {
	ID               int64
	LightLevel       sql.NullInt64
	ColorMode        string
	ColorTemperature sql.NullInt64
	Hue              sql.NullInt64
	Saturation       sql.NullInt64
	LightID          int64
	SceneID          int64
}

// RoomEntitiesPage mirrors web_roomentitiespage.
type RoomEntitiesPage struct {
	ID           int64
	DisplayOrder int64
	PageType     int64
	IsScenesPage bool
	RoomID       sql.NullInt64
}

// Entity mirrors web_entity. EntityData is a JSON blob with live state that
// the bridge reads AND writes back (same as the C++ Entity::set_entity_data_json).
type Entity struct {
	ID               int64
	EntityType       string
	FriendlyName     string
	EntitiesPageID   int64
	RoomViewPosition int64
	RoomID           int64
	EntityData       string
}

var database *sql.DB

// Open opens (creating if needed) the shared database. Test mode uses the
// temp path, mirroring the C++ TEST_MODE behavior.
func Open() error {
	path := "/data/nspanelmanager_db.sqlite3"
	if os.Getenv("TEST_MODE") == "1" {
		path = "/tmp/nspanelmanager_db_test.sqlite3"
	}
	d, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	// Shared file with Django — allow multiple connections but limit write contention.
	d.SetMaxOpenConns(4)
	database = d
	return d.Ping()
}

func Close() {
	if database != nil {
		_ = database.Close()
	}
}

// GetSetting returns the value for a setting name, or def when missing/empty.
func GetSetting(name, def string) string {
	var v string
	err := database.QueryRow(`SELECT value FROM web_settings WHERE name = ? LIMIT 1`, name).Scan(&v)
	if err != nil || v == "" {
		return def
	}
	return v
}

// GetNSPanelSetting returns a per-panel override, falling back to the global
// setting and then def — mirroring the C++ get_setting_with_default flow.
func GetNSPanelSetting(nspanelID int64, name, def string) string {
	var v string
	err := database.QueryRow(
		`SELECT value FROM web_nspanelsettingholder WHERE nspanel_id = ? AND name = ? LIMIT 1`,
		nspanelID, name).Scan(&v)
	if err == nil && v != "" {
		return v
	}
	return GetSetting(name, def)
}

// SetSetting upserts a global setting value.
func SetSetting(name, value string) error {
	_, err := database.Exec(
		`INSERT INTO web_settings (name, value) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET value = excluded.value`, name, value)
	return err
}

// SetNSPanelSetting upserts a per-panel setting.
func SetNSPanelSetting(nspanelID int64, name, value string) error {
	var id int64
	err := database.QueryRow(
		`SELECT id FROM web_nspanelsettingholder WHERE nspanel_id = ? AND name = ?`,
		nspanelID, name).Scan(&id)
	if err == nil {
		_, err = database.Exec(`UPDATE web_nspanelsettingholder SET value = ? WHERE id = ?`, value, id)
		return err
	}
	_, err = database.Exec(
		`INSERT INTO web_nspanelsettingholder (name, value, nspanel_id) VALUES (?, ?, ?)`,
		name, value, nspanelID)
	return err
}

// ListNSPanels returns all panels in the database.
func ListNSPanels() ([]NSPanel, error) {
	rows, err := database.Query(`SELECT id, mac_address, friendly_name, model, room_id, version,
		button1_detached_mode_entity_id, button1_mode, button2_detached_mode_entity_id, button2_mode,
		md5_data_file, md5_firmware, md5_tft_file, denied, accepted FROM web_nspanel`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NSPanel
	for rows.Next() {
		var p NSPanel
		if err := rows.Scan(&p.ID, &p.MacAddress, &p.FriendlyName, &p.Model, &p.RoomID, &p.Version,
			&p.Button1DetachedModeEntityID, &p.Button1Mode, &p.Button2DetachedModeEntityID, &p.Button2Mode,
			&p.Md5DataFile, &p.Md5Firmware, &p.Md5TftFile, &p.Denied, &p.Accepted); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetNSPanelByID fetches a single panel.
func GetNSPanelByID(id int64) (*NSPanel, error) {
	var p NSPanel
	err := database.QueryRow(`SELECT id, mac_address, friendly_name, model, room_id, version,
		button1_detached_mode_entity_id, button1_mode, button2_detached_mode_entity_id, button2_mode,
		md5_data_file, md5_firmware, md5_tft_file, denied, accepted FROM web_nspanel WHERE id = ?`, id).
		Scan(&p.ID, &p.MacAddress, &p.FriendlyName, &p.Model, &p.RoomID, &p.Version,
			&p.Button1DetachedModeEntityID, &p.Button1Mode, &p.Button2DetachedModeEntityID, &p.Button2Mode,
			&p.Md5DataFile, &p.Md5Firmware, &p.Md5TftFile, &p.Denied, &p.Accepted)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetNSPanelByMac fetches a panel by MAC address.
func GetNSPanelByMac(mac string) (*NSPanel, error) {
	var p NSPanel
	err := database.QueryRow(`SELECT id, mac_address, friendly_name, model, room_id, version,
		button1_detached_mode_entity_id, button1_mode, button2_detached_mode_entity_id, button2_mode,
		md5_data_file, md5_firmware, md5_tft_file, denied, accepted FROM web_nspanel WHERE mac_address = ?`, mac).
		Scan(&p.ID, &p.MacAddress, &p.FriendlyName, &p.Model, &p.RoomID, &p.Version,
			&p.Button1DetachedModeEntityID, &p.Button1Mode, &p.Button2DetachedModeEntityID, &p.Button2Mode,
			&p.Md5DataFile, &p.Md5Firmware, &p.Md5TftFile, &p.Denied, &p.Accepted)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListRooms returns all rooms ordered by display order.
func ListRooms() ([]Room, error) {
	rows, err := database.Query(`SELECT id, friendly_name, room_temp_provider, room_temp_sensor, display_order FROM web_room ORDER BY display_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.FriendlyName, &r.RoomTempProvider, &r.RoomTempSensor, &r.DisplayOrder); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListScenes returns all scenes.
func ListScenes() ([]Scene, error) {
	rows, err := database.Query(`SELECT id, room_id, friendly_name, scene_type, backend_name, room_view_position, entities_page_id FROM web_scene`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Scene
	for rows.Next() {
		var s Scene
		if err := rows.Scan(&s.ID, &s.RoomID, &s.FriendlyName, &s.SceneType, &s.BackendName, &s.RoomViewPosition, &s.EntitiesPageID); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListEntityPages returns all room entity pages.
func ListEntityPages() ([]RoomEntitiesPage, error) {
	rows, err := database.Query(`SELECT id, display_order, page_type, is_scenes_page, room_id FROM web_roomentitiespage`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RoomEntitiesPage
	for rows.Next() {
		var p RoomEntitiesPage
		if err := rows.Scan(&p.ID, &p.DisplayOrder, &p.PageType, &p.IsScenesPage, &p.RoomID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListEntitiesByPage returns entities for an entity page ordered by position.
func ListEntitiesByPage(pageID int64) ([]Entity, error) {
	rows, err := database.Query(`SELECT id, entity_type, friendly_name, entities_page_id, room_view_position, room_id, entity_data
		FROM web_entity WHERE entities_page_id = ? ORDER BY room_view_position`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.EntityType, &e.FriendlyName, &e.EntitiesPageID, &e.RoomViewPosition, &e.RoomID, &e.EntityData); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateEntityData writes back the live-state JSON blob for an entity.
func UpdateEntityData(entityID int64, data string) error {
	_, err := database.Exec(`UPDATE web_entity SET entity_data = ? WHERE id = ?`, data, entityID)
	return err
}

// UpdateNSPanelChecksums persists md5 checksums for OTA comparisons.
func UpdateNSPanelChecksums(id int64, md5Data, md5Firmware, md5Tft string) error {
	_, err := database.Exec(`UPDATE web_nspanel SET md5_data_file = ?, md5_firmware = ?, md5_tft_file = ? WHERE id = ?`,
		md5Data, md5Firmware, md5Tft, id)
	return err
}

// ListScenesForPage returns scenes attached to an entity page.
func ListScenesForPage(pageID int64) ([]Scene, error) {
	rows, err := database.Query(`SELECT id, room_id, friendly_name, scene_type, backend_name, room_view_position, entities_page_id
		FROM web_scene WHERE entities_page_id = ? ORDER BY room_view_position`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Scene
	for rows.Next() {
		var s Scene
		if err := rows.Scan(&s.ID, &s.RoomID, &s.FriendlyName, &s.SceneType, &s.BackendName, &s.RoomViewPosition, &s.EntitiesPageID); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// NormalizeMAC trims and uppercases a MAC address.
func NormalizeMAC(mac string) string {
	return strings.TrimSpace(strings.ToUpper(mac))
}

// SceneLightStateRow mirrors web_scenelightstate.
type SceneLightStateRow struct {
	ID               int64
	LightLevel       sql.NullInt64
	ColorMode        string
	ColorTemperature sql.NullInt64
	Hue              sql.NullInt64
	Saturation       sql.NullInt64
	LightID          int64
	SceneID          int64
}

// ListSceneLightStates returns saved states for a scene.
func ListSceneLightStates(sceneID int64) ([]SceneLightStateRow, error) {
	rows, err := database.Query(`SELECT id, light_level, color_mode, color_temperature, hue, saturation, light_id, scene_id
		FROM web_scenelightstate WHERE scene_id = ?`, sceneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SceneLightStateRow
	for rows.Next() {
		var s SceneLightStateRow
		if err := rows.Scan(&s.ID, &s.LightLevel, &s.ColorMode, &s.ColorTemperature, &s.Hue, &s.Saturation, &s.LightID, &s.SceneID); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteSceneLightStates removes all saved states for a scene.
func DeleteSceneLightStates(sceneID int64) error {
	_, err := database.Exec(`DELETE FROM web_scenelightstate WHERE scene_id = ?`, sceneID)
	return err
}

// InsertSceneLightState saves one light state for a scene.
func InsertSceneLightState(sceneID, lightID, lightLevel int64, colorMode string, colorTemp, hue, sat int64) error {
	_, err := database.Exec(`INSERT INTO web_scenelightstate (light_level, color_mode, color_temperature, hue, saturation, light_id, scene_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, lightLevel, colorMode, colorTemp, hue, sat, lightID, sceneID)
	return err
}
