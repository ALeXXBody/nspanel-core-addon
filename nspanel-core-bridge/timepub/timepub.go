// Package timepub publishes time/date to MQTT every minute, aligned to the
// minute boundary — a direct port of the C++ publish_time_and_date thread.
package timepub

import (
	"fmt"
	"log"
	"strings"
	"time"

	"nspanel-core-bridge/mqtt"
)

// Config holds the knobs the C++ code reads from the DB each iteration.
type Config struct {
	Timezone     string // TZ database name, e.g. "Europe/Stockholm"
	ClockUSStyle bool   // AM/PM style
	DateFormat   string // strftime-like format string from settings
	ManagerAddr  string
}

// Start launches the publisher loop. Runs until the process exits.
// Re-reads Config from the provider each minute so SIGUSR1 config reloads
// take effect without restarting the loop.
func Start(getConfig func() Config) {
	go func() {
		for {
			cfg := getConfig()
			publishOnce(cfg)
			sleepUntilNextMinute()
		}
	}()
}

func publishOnce(cfg Config) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Printf("[timepub] invalid timezone %q: %v — falling back to system local", cfg.Timezone, err)
		loc = time.Local
	}
	now := time.Now().In(loc)

	timeStr := now.Format("15:04")
	ampmStr := ""
	if cfg.ClockUSStyle {
		timeStr = now.Format("03:04")
		ampmStr = now.Format("PM")
	}

	dateStr := strftimeLike(cfg.DateFormat, now)

	mqtt.Publish(mqtt.TimeTopic(cfg.ManagerAddr), timeStr, true)
	mqtt.Publish(mqtt.AmpmTopic(cfg.ManagerAddr), ampmStr, true)
	mqtt.Publish(mqtt.DateTopic(cfg.ManagerAddr), dateStr, true)
}

// sleepUntilNextMinute waits until the next minute boundary (second 1).
func sleepUntilNextMinute() {
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute + time.Second)
	time.Sleep(time.Until(next))
}

// strftimeLike converts a small strftime subset to Go time layout.
// Supports: %H %I %M %S %p %A %a %B %b %d %m %Y %y %Z %%
func strftimeLike(format string, t time.Time) string {
	if format == "" {
		format = "%d/%m"
	}
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		if i+1 >= len(format) {
			b.WriteByte('%')
			break
		}
		switch format[i+1] {
		case 'H':
			b.WriteString(t.Format("15"))
		case 'I':
			b.WriteString(t.Format("03"))
		case 'M':
			b.WriteString(t.Format("04"))
		case 'S':
			b.WriteString(t.Format("05"))
		case 'p':
			b.WriteString(t.Format("PM"))
		case 'A':
			b.WriteString(t.Format("Monday"))
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'B':
			b.WriteString(t.Format("January"))
		case 'b':
			b.WriteString(t.Format("Jan"))
		case 'd':
			b.WriteString(t.Format("02"))
		case 'm':
			b.WriteString(t.Format("01"))
		case 'Y':
			b.WriteString(t.Format("2006"))
		case 'y':
			b.WriteString(t.Format("06"))
		case 'Z':
			b.WriteString(t.Format("MST"))
		case '%':
			b.WriteByte('%')
		default:
			b.WriteString(fmt.Sprintf("%%%c", format[i+1]))
		}
		i++
	}
	return b.String()
}
