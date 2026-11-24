// Package server builds the fusion-service HTTP mux, shared by
// cmd/fusion-service/main.go and the integration tests so both exercise
// identical routing.
package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/config"
	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/entity"
	"sar-drone-prototype/fusion-service/internal/handlers"
	"sar-drone-prototype/fusion-service/internal/localqueue"
	"sar-drone-prototype/fusion-service/internal/logger"
	"sar-drone-prototype/fusion-service/internal/ws"
)

func NewMux(
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	hub *ws.Hub,
	queue *localqueue.Queue,
	monitor *connectivity.Monitor,
	clusterer *entity.Clusterer,
	alertMgr *alerting.Manager,
	cfg config.Config,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handlers.Health(pool, redisClient))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"service":"fusion-service","status":"running"}`))
	})

	// Detections
	mux.HandleFunc("GET /detections", handlers.ListDetections(pool))
	mux.HandleFunc("GET /detections/{id}", handlers.GetDetectionByID(pool))

	// Entities (Survivors and Hazards)
	mux.HandleFunc("GET /entities", handlers.ListEntities(pool, clusterer))
	mux.HandleFunc("GET /entities/{id}", handlers.GetEntityByID(pool, clusterer))
	mux.HandleFunc("POST /entities/{id}/resolve", handlers.ResolveEntity(pool, clusterer, alertMgr))

	// Hazard Polygons (GeoJSON)
	mux.HandleFunc("GET /hazards/geojson", handlers.HazardsGeoJSON(pool))

	// Mission summary & route status
	mux.HandleFunc("GET /mission/summary", handlers.MissionSummary(pool, cfg.HighPriorityThreshold))
	mux.HandleFunc("GET /mission/route-status", handlers.RouteStatus(pool, cfg.RouteObstructionMeters, cfg.RouteStatusTelemetryLimit))

	// Alerts
	mux.HandleFunc("GET /alerts", handlers.ListAlerts(pool, alertMgr))
	mux.HandleFunc("GET /alerts/{id}", handlers.GetAlertByID(pool, alertMgr))
	mux.HandleFunc("POST /alerts/{id}/resolve", handlers.ResolveAlert(pool, alertMgr))

	// WebSocket live feed
	mux.HandleFunc("/ws/live", hub.HandleLive)

	// OpenAPI spec
	mux.HandleFunc("GET /openapi.json", handlers.OpenAPI())

	// Offline-buffering demo/test controls
	mux.HandleFunc("POST /debug/set-offline", handlers.SetOffline(monitor))
	mux.HandleFunc("GET /debug/offline-status", handlers.OfflineStatus(monitor))
	mux.HandleFunc("GET /debug/queue", handlers.QueueStatus(queue))

	return logger.Middleware(mux)
}
