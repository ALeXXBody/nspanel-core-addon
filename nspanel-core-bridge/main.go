// Command nspm_mqttmanager is the Go rewrite of the NSPanel MQTT bridge.
// Drop-in replacement for the C++ binary: same process path, same DB, same
// MQTT topics, same STOMP WebSocket API on :8002, same SIGUSR1 reload.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nspanel-core-bridge/config"
	"nspanel-core-bridge/db"
	"nspanel-core-bridge/ha"
	"nspanel-core-bridge/mqtt"
	"nspanel-core-bridge/stomp"
	"nspanel-core-bridge/timepub"
	"nspanel-core-bridge/weather"
)

var version = "dev"

func main() {
	log.Printf("Starting MQTTManager (nspanel-core-bridge %s).", version)

	if err := db.Open(); err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Map LOG_LEVEL env (same values as the C++ spdlog setup).
	logLevel := config.LogLevelFromEnv(os.Getenv("LOG_LEVEL"))
	log.Printf("Log level: %s", logLevel)

	// Initial config load (reads DB + /etc/timezone + env).
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect MQTT.
	client, err := mqtt.Connect(cfg.MqttServer, cfg.MqttPort, cfg.MqttUsername, cfg.MqttPassword)
	if err != nil {
		log.Fatalf("MQTT connect failed: %v", err)
	}
	_ = client

	// Time/date publisher — reads fresh config each minute so SIGUSR1
	// reloads (timezone/format changes) take effect immediately.
	timepub.Start(func() timepub.Config {
		c, _ := config.Load()
		return timepub.Config{
			Timezone:     c.Timezone,
			ClockUSStyle: c.ClockUSStyle,
			DateFormat:   c.DateFormat,
			ManagerAddr:  c.ManagerAddress,
		}
	})

	// STOMP WebSocket server on :8002 (kiosk web UI connects here via nginx).
	stompServer := stomp.NewServer()
	stompServer.SetTopicRetained("mqttmanager/warnings", true)
	go func() {
		if err := stompServer.Start(":8002"); err != nil {
			log.Printf("STOMP server exited: %v", err)
		}
	}()
	log.Printf("Starting Websocket Server on port 8002.")

	// HA reload on SIGUSR1: update credentials if changed.
	// (handled inside the SIGUSR1 goroutine below)

	log.Printf("Config loaded. Starting components.")

	// Home Assistant WebSocket client — connects, auths, subscribes to
	// state_changed and requests all states (same lifecycle as the C++ one).
	haClient := ha.NewClient(cfg.HaAddress, cfg.HaToken)
	ha.SetGlobal(haClient)
	haClient.Connect()

	// Weather loop — reads fresh config each iteration.
	weather.Start(func() weather.Config {
		c, _ := config.Load()
		return weather.Config{
			Latitude:          c.LocationLatitude,
			Longitude:         c.LocationLongitude,
			IntervalMinutes:   c.WeatherUpdateInterval,
			TemperatureUnit:   "celsius",
			PrecipitationUnit: c.PrecipitationFormat,
			WindSpeedFormat:   c.WindSpeedFormat,
			Timezone:          c.Timezone,
			ManagerAddress:    c.ManagerAddress,
			UseFahrenheit:     c.UseFahrenheit,
		}
	})

	// SIGUSR1 → reload config from DB and republish (same as C++ handler).
	sigUsr1 := make(chan os.Signal, 1)
	signal.Notify(sigUsr1, syscall.SIGUSR1)
	go func() {
		for range sigUsr1 {
			log.Printf("Reloading config from manager.")
			time.Sleep(50 * time.Millisecond)
			if newCfg, err := config.Load(); err != nil {
				log.Printf("Config reload failed: %v", err)
				continue
			} else {
				haClient.SetCredentials(newCfg.HaAddress, newCfg.HaToken)
			}
			mqtt.Publish(mqtt.ConfigReloadTopic(), "1", false)
			log.Printf("Reload signal processing completed.")
		}
	}()

	log.Printf("Config loaded. Bridge core online. (STOMP server, HA client, entities pending integration)")

	// Keep alive forever, same as the C++ main loop.
	select {}
}
