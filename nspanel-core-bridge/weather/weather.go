// Package weather polls open-meteo and publishes NSPanelWeatherUpdate
// protobuf updates — a port of the C++ MQTTManagerWeather.
package weather

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"nspanel-core-bridge/mqtt"
	pb "nspanel-core-bridge/proto"
)

type Config struct {
	Latitude          string
	Longitude         string
	IntervalMinutes   int
	TemperatureUnit   string // "celsius" | "fahrenheit"
	PrecipitationUnit string // "mm" | "inch"
	WindSpeedUnit     string // "kmh" | "ms" | "mph" | "kn"
	WindSpeedFormat   string
	Timezone          string
	ManagerAddress    string
	UseFahrenheit     bool
}

// Map of WMO weather codes → icon names used by the panel HMI
// (mirrors the C++ weather_code → icon mapping).
var weatherIcons = map[int64]string{
	0: "sunny", 1: "partlytuner weather_day", 2: "cloudy_weather_day", 3: "overcast",
	45: "fog", 48: "fog",
	51: "rainy", 53: "rainy", 55: "rainy",
	56: "sleet", 57: "sleet",
	61: "rainy", 63: "rainy", 65: "rainy",
	66: "sleet", 67: "sleet",
	71: "snowy", 73: "snowy", 75: "snowy", 77: "snowy",
	80: "rainy", 81: "rainy", 82: "rainy",
	85: "snowy", 86: "snowy",
	95: "thunderstorm", 96: "thunderstorm", 99: "thunderstorm",
}

func iconForCode(code int64) string {
	if name, ok := weatherIcons[code]; ok {
		return name
	}
	return "cloudy_weather_day"
}

type openMeteoResponse struct {
	Current struct {
		Temperature2m      float64 `json:"temperature_2m"`
		RelativeHumidity2m float64 `json:"relative_humidity_2m"`
		Precipitation      float64 `json:"precipitation"`
		WeatherCode        int64   `json:"weather_code"`
		WindSpeed10m       float64 `json:"wind_speed_10m"`
		WindGusts10m       float64 `json:"wind_gusts_10m"`
	} `json:"current"`
	Daily struct {
		WeatherCode               []int64   `json:"weather_code"`
		Temperature2mMax          []float64 `json:"temperature_2m_max"`
		Temperature2mMin          []float64 `json:"temperature_2m_min"`
		Sunrise                   []int64   `json:"sunrise"`
		Sunset                    []int64   `json:"sunset"`
		PrecipitationSum          []float64 `json:"precipitation_sum"`
		PrecipitationProbabilityMax []float64 `json:"precipitation_probability_max"`
		WindSpeed10mMax           []float64 `json:"wind_speed_10m_max"`
		WindGusts10mMax           []float64 `json:"wind_gusts_10m_max"`
	} `json:"daily"`
}

// fetchAndPublish pulls the forecast and publishes the protobuf update.
func fetchAndPublish(cfg Config) error {
	units := fmt.Sprintf(
		"wind_speed_unit=%s&precipitation_unit=%s&temperature_unit=%s",
		windParam(cfg.WindSpeedFormat), precipParam(cfg.PrecipitationUnit), tempParam(cfg))
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&current=temperature_2m,relative_humidity_2m,precipitation,weather_code,wind_speed_10m,wind_gusts_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset,precipitation_sum,precipitation_probability_max,wind_speed_10m_max,wind_gusts_10m_max&timeformat=unixtime&%s&timezone=%s",
		cfg.Latitude, cfg.Longitude, units, cfg.Timezone)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("weather fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("weather fetch: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var data openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("weather decode: %w", err)
	}

	update := &pb.NSPanelWeatherUpdate{
		CurrentWeatherIcon:      iconForCode(data.Current.WeatherCode),
		CurrentTemperatureString: formatTemp(data.Current.Temperature2m, cfg),
		CurrentMaxminTemperature: fmt.Sprintf("%s / %s",
			formatTemp(first(data.Daily.Temperature2mMax), cfg),
			formatTemp(first(data.Daily.Temperature2mMin), cfg)),
		CurrentWindString:         formatWind(data.Current.WindSpeed10m, cfg),
		SunriseString:             formatTime(at64(data.Daily.Sunrise, 0), cfg.Timezone),
		SunsetString:              formatTime(at64(data.Daily.Sunset, 0), cfg.Timezone),
		CurrentPrecipitationString: fmt.Sprintf("%g %s", data.Current.Precipitation, precipUnit(cfg.PrecipitationUnit)),
	}

	for i := 0; i < len(data.Daily.WeatherCode) && i < 4; i++ {
		item := &pb.NSPanelWeatherUpdate_ForecastItem{
			WeatherIcon:           iconForCode(data.Daily.WeatherCode[i]),
			PrecipitationString:   fmt.Sprintf("%s %.0f%%", precipUnit(cfg.PrecipitationUnit), firstF(data.Daily.PrecipitationProbabilityMax, i)),
			TemperatureMaxminString: fmt.Sprintf("%s / %s",
				formatTemp(at(data.Daily.Temperature2mMax, i), cfg),
				formatTemp(at(data.Daily.Temperature2mMin, i), cfg)),
			WindString:  formatWind(at(data.Daily.WindSpeed10mMax, i), cfg),
			DisplayString: "",
		}
		update.ForecastItems = append(update.ForecastItems, item)
	}

	// Serialize protobuf → base64 → publish to the manager status/weather topic.
	raw, err := proto.Marshal(update)
	if err != nil {
		return fmt.Errorf("weather marshal: %w", err)
	}
	mqtt.Publish(mqtt.WeatherTopic(cfg.ManagerAddress), base64.StdEncoding.EncodeToString(raw), true)
	return nil
}

// Start launches the weather update loop.
func Start(getConfig func() Config) {
	go func() {
		for {
			cfg := getConfig()
			if cfg.Latitude == "" || cfg.Longitude == "" {
				log.Printf("[weather] no location configured, skipping weather updates")
				time.Sleep(10 * time.Minute)
				continue
			}
			if err := fetchAndPublish(cfg); err != nil {
				log.Printf("[weather] %v", err)
			}
			interval := time.Duration(cfg.IntervalMinutes) * time.Minute
			if interval < time.Minute {
				interval = 30 * time.Minute
			}
			time.Sleep(interval)
		}
	}()
}



// --- formatting helpers ---

func formatTemp(celsius float64, cfg Config) string {
	if cfg.UseFahrenheit {
		return fmt.Sprintf("%d°F", int(math.Round(celsius*9/5+32)))
	}
	return fmt.Sprintf("%d°C", int(math.Round(celsius)))
}

func formatWind(speed float64, cfg Config) string {
	switch cfg.WindSpeedFormat {
	case "ms":
		return fmt.Sprintf("%.1f m/s", speed)
	case "mph":
		return fmt.Sprintf("%d mph", int(math.Round(speed)))
	case "kn":
		return fmt.Sprintf("%d kn", int(math.Round(speed)))
	default:
		return fmt.Sprintf("%d km/h", int(math.Round(speed)))
	}
}

func windParam(format string) string {
	switch format {
	case "ms":
		return "ms"
	case "mph":
		return "mph"
	case "kn":
		return "kn"
	default:
		return "kmh"
	}
}

func precipParam(format string) string {
	if strings.EqualFold(format, "inch") {
		return "inch"
	}
	return "mm"
}

func precipUnit(format string) string {
	if strings.EqualFold(format, "inch") {
		return "in"
	}
	return "mm"
}

func tempParam(cfg Config) string {
	if cfg.UseFahrenheit {
		return "fahrenheit"
	}
	return "celsius"
}

func formatTime(unix int64, tz string) string {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	return time.Unix(unix, 0).In(loc).Format("15:04")
}

func at(list []float64, i int) float64 {
	if i >= 0 && i < len(list) {
		return list[i]
	}
	return 0
}

func first(list []float64) float64 {
	return at(list, 0)
}

func firstF(list []float64, i int) float64 { return at(list, i) }

func at64(list []int64, i int) int64 {
	if i >= 0 && i < len(list) {
		return list[i]
	}
	return 0
}
