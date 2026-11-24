// Package server builds the fusion-service HTTP mux, shared by
// cmd/fusion-service/main.go and the integration tests so both exercise
// identical routing.
package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sar-drone-prototype/fusion-service/internal/config"
	"sar-drone-prototype/fusion-service/internal/connectivity"
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
	cfg config.Config,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handlers.Health(pool, redisClient))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"service":"fusion-service","status":"running"}`))
	})

	mux.HandleFunc("GET /detections", handlers.ListDetections(pool))
	mux.HandleFunc("GET /detections/{id}", handlers.GetDetectionByID(pool))
	mux.HandleFunc("GET /mission/summary", handlers.MissionSummary(pool, cfg.HighPriorityThreshold))
	mux.HandleFunc("GET /mission/route-status", handlers.RouteStatus(pool, cfg.RouteObstructionMeters, cfg.RouteStatusTelemetryLimit))

	mux.HandleFunc("GET /alerts", handlers.ListAlerts(pool))

	mux.HandleFunc("/ws/live", hub.HandleLive)

	// Offline-buffering demo/test controls (see internal/connectivity,
	// internal/localqueue).
	mux.HandleFunc("POST /debug/set-offline", handlers.SetOffline(monitor))
	mux.HandleFunc("GET /debug/offline-status", handlers.OfflineStatus(monitor))
	mux.HandleFunc("GET /debug/queue", handlers.QueueStatus(queue))

	return logger.Middleware(mux)
}
