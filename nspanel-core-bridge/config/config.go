// Package config loads the bridge configuration from the shared SQLite
// database, /etc/timezone and environment variables — the same sources the
// C++ MqttManagerConfig::load uses.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"nspanel-core-bridge/db"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	Timezone string

	// MQTT
	MqttServer   string
	MqttPort     int
	MqttUsername string
	MqttPassword string

	// Manager
	ManagerAddress string
	ManagerPort    int

	// Home Assistant
	IsHomeAssistantAddon bool
	HaAddress            string
	HaToken              string

	// Clock / format
	ClockUSStyle bool
	DateFormat   string
	UseFahrenheit bool

	// Weather
	LocationLatitude  string
	LocationLongitude string
	WeatherUpdateInterval int
	PrecipitationFormat string
	WindSpeedFormat     string

	// Behaviour
	OptimisticMode bool
	TurnOnBehaviour string
	LightTurnOnBrightness int

	// Screensaver
	ScreensaverActivationTimeout int
	ScreensaverDimLevel          int
	ScreenDimLevel               int
	ScreensaverMode              string

	// Misc
	MqttmanagerLogLevel string
}

// LogLevelFromEnv maps the LOG_LEVEL env value (spdlog-compatible) to a name.
func LogLevelFromEnv(v string) string {
	switch strings.ToLower(v) {
	case "error":
		return "error"
	case "warning", "warn":
		return "warning"
	case "info":
		return "info"
	case "debug":
		return "debug"
	case "trace":
		return "trace"
	default:
		log.Printf("No valid LOG_LEVEL set. Will assume level debug.")
		return "debug"
	}
}

// LoadTimezone reads the system timezone exactly like the C++ loader.
func LoadTimezone() string {
	b, err := os.ReadFile("/etc/timezone")
	if err != nil {
		log.Printf("Failed to read /etc/timezone: %v — falling back to TZ env or UTC", err)
		if tz := os.Getenv("TZ"); tz != "" {
			return tz
		}
		return "UTC"
	}
	return strings.TrimSpace(string(b))
}

// Load reads the full configuration.
func Load() (*Config, error) {
	c := &Config{}
	c.Timezone = LoadTimezone()
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		log.Printf("Timezone %q is not a valid TZ identifier: %v", c.Timezone, err)
	}

	c.IsHomeAssistantAddon = strings.EqualFold(os.Getenv("IS_HOME_ASSISTANT_ADDON"), "true")

	c.MqttServer = db.GetSetting("mqtt_server", "")
	c.MqttPort = atoi(db.GetSetting("mqtt_port", "1883"))
	c.MqttUsername = db.GetSetting("mqtt_username", "")
	c.MqttPassword = db.GetSetting("mqtt_password", "")

	c.ManagerAddress = db.GetSetting("manager_address", "")
	c.ManagerPort = atoi(db.GetSetting("manager_port", "8000"))

	c.HaAddress = db.GetSetting("home_assistant_address", "")
	c.HaToken = db.GetSetting("home_assistant_token", "")

	c.ClockUSStyle = db.GetSetting("clock_us_style", "true") == "true"
	c.DateFormat = db.GetSetting("date_format", "%d/%m")
	c.UseFahrenheit = db.GetSetting("use_fahrenheit", "false") == "true"

	c.LocationLatitude = db.GetSetting("location_latitude", "")
	c.LocationLongitude = db.GetSetting("location_longitude", "")
	c.WeatherUpdateInterval = atoi(db.GetSetting("weather_update_interval", "30"))
	c.PrecipitationFormat = db.GetSetting("weather_precipitation_format", "mm")
	c.WindSpeedFormat = db.GetSetting("weather_wind_speed_format", "kmh")

	c.OptimisticMode = db.GetSetting("optimistic_mode", "false") == "true"
	c.TurnOnBehaviour = db.GetSetting("turn_on_light_behaviour", "color_temp")
	c.LightTurnOnBrightness = atoi(db.GetSetting("light_turn_on_brightness", "0"))

	c.ScreensaverActivationTimeout = atoi(db.GetSetting("screensaver_activation_timeout", "15"))
	c.ScreensaverDimLevel = atoi(db.GetSetting("screensaver_dim_level", "20"))
	c.ScreenDimLevel = atoi(db.GetSetting("screen_dim_level", "0"))
	c.ScreensaverMode = db.GetSetting("screensaver_mode", "WEATHER_WITH_BACKGROUND")

	c.MqttmanagerLogLevel = db.GetSetting("mqttmanager_log_level", "debug")

	return c, nil
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// String is a debug helper.
func (c *Config) String() string {
	return fmt.Sprintf("Config{timezone=%s mqtt=%s:%d manager=%s:%d ha=%s addon=%v}",
		c.Timezone, c.MqttServer, c.MqttPort, c.ManagerAddress, c.ManagerPort, c.HaAddress, c.IsHomeAssistantAddon)
}
