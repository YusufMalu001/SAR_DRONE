package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port         string
	RedisAddr    string
	DatabaseURL  string
	DetectionsCh string
	TelemetryCh  string

	ConsumerGroup string
	ConsumerName  string

	// How close (in wall-clock time) a telemetry sample must be to a
	// detection's own timestamp to be considered a match.
	TelemetryToleranceMS int

	// Pinhole camera model parameters for projecting a detection's bbox
	// center into a ground lat/lon. Assumes a nadir (straight-down)
	// mounted camera whose image "up" axis faces the drone's heading.
	CameraHFOVDeg float64
	CameraVFOVDeg float64

	// Priority scoring: a detection gets a boost if a hazard-class
	// detection was recorded within this radius and time window.
	HazardProximityMeters float64
	HazardWindowSeconds   int

	// GET /mission/summary counts detections at/above this score as
	// "high priority".
	HighPriorityThreshold float64

	// Offline buffering: every fused detection is written to this local
	// SQLite file first, before Postgres is attempted.
	LocalQueuePath string

	// How often the connectivity monitor pings Postgres to decide whether
	// writes should go straight through or queue locally.
	ConnectivityCheckIntervalMS int

	// How often the drain loop checks for a non-empty local queue while
	// online (in addition to draining immediately on an offline->online
	// transition).
	DrainIntervalMS int

	// GET /mission/route-status flags a flight-path segment "obstructed" if
	// it passes within this many meters of a hazard-class detection.
	RouteObstructionMeters float64

	// Max number of recorded telemetry points GET /mission/route-status
	// reconstructs the flight path from.
	RouteStatusTelemetryLimit int

	// Directory containing offline pre-cached raster tiles (/tiles/{z}/{x}/{y}.png).
	TileDir string

	// Guard for /debug/set-offline toggle in production builds.
	AllowDebugToggle bool

	// Outbound alerting adapters (comma-separated: mock, webhook, mqtt, email, sms).
	OutboundAdapters []string
	WebhookURL       string
	MQTTHost         string
	EmailSMTP        string
	SMSEndpoint      string
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func Load() Config {
	adaptersStr := getenv("OUTBOUND_ADAPTERS", "mock")
	var adapters []string
	for _, a := range strings.Split(adaptersStr, ",") {
		trimmed := strings.TrimSpace(a)
		if trimmed != "" {
			adapters = append(adapters, trimmed)
		}
	}

	return Config{
		Port:         getenv("PORT", "8002"),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		DatabaseURL:  getenv("DATABASE_URL", "postgres://sar:sar@localhost:5432/sar?sslmode=disable"),
		DetectionsCh: getenv("DETECTIONS_CHANNEL", "detections"),
		TelemetryCh:  getenv("TELEMETRY_CHANNEL", "telemetry"),

		ConsumerGroup: getenv("CONSUMER_GROUP", "fusion-service"),
		ConsumerName:  getenv("CONSUMER_NAME", "fusion-service-1"),

		TelemetryToleranceMS: getenvInt("TELEMETRY_TOLERANCE_MS", 200),

		CameraHFOVDeg: getenvFloat("CAMERA_HFOV_DEG", 90.0),
		CameraVFOVDeg: getenvFloat("CAMERA_VFOV_DEG", 60.0),

		HazardProximityMeters: getenvFloat("HAZARD_PROXIMITY_METERS", 50.0),
		HazardWindowSeconds:   getenvInt("HAZARD_WINDOW_SECONDS", 60),

		HighPriorityThreshold: getenvFloat("HIGH_PRIORITY_THRESHOLD", 0.7),

		LocalQueuePath:              getenv("LOCAL_QUEUE_PATH", "/data/fusion-queue.db"),
		ConnectivityCheckIntervalMS: getenvInt("CONNECTIVITY_CHECK_INTERVAL_MS", 5000),
		DrainIntervalMS:             getenvInt("DRAIN_INTERVAL_MS", 2000),

		RouteObstructionMeters:    getenvFloat("ROUTE_OBSTRUCTION_METERS", 20.0),
		RouteStatusTelemetryLimit: getenvInt("ROUTE_STATUS_TELEMETRY_LIMIT", 5000),
		TileDir:                   getenv("TILE_DIR", "/data/tiles"),

		AllowDebugToggle: getenvBool("ALLOW_DEBUG_TOGGLE", true),

		OutboundAdapters: adapters,
		WebhookURL:       getenv("ALERT_WEBHOOK_URL", ""),
		MQTTHost:         getenv("ALERT_MQTT_HOST", ""),
		EmailSMTP:        getenv("ALERT_EMAIL_SMTP", ""),
		SMSEndpoint:      getenv("ALERT_SMS_ENDPOINT", ""),
	}
}
