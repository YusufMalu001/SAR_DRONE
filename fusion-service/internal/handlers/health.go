package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/localqueue"
	"sar-drone-prototype/fusion-service/internal/messaging"
)

type DegradedOperations struct {
	Active []string `json:"active"`
	Paused []string `json:"paused"`
}

type HealthResponse struct {
	Status               string             `json:"status"`
	Service              string             `json:"service"`
	Mode                 string             `json:"mode"`
	Postgres             string             `json:"postgres"`
	Redis                string             `json:"redis"`
	QueueDepth           int                `json:"queue_depth"`
	OldestItemAgeSeconds float64            `json:"oldest_item_age_seconds"`
	LastSuccessfulSync   *string            `json:"last_successful_sync,omitempty"`
	DegradedOperations   DegradedOperations `json:"degraded_operations"`
}

func Health(
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	monitor *connectivity.Monitor,
	queue *localqueue.Queue,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		postgres := "down"
		if db.Ping(pool) {
			postgres = "up"
		}

		redisStatus := "down"
		if messaging.Ping(redisClient) {
			redisStatus = "up"
		}

		isOffline := postgres != "up"
		if monitor != nil && monitor.IsOffline() {
			isOffline = true
		}

		mode := "online"
		if isOffline {
			mode = "offline"
			status = "degraded"
		} else if redisStatus != "up" {
			mode = "degraded"
			status = "degraded"
		}

		activeOps := []string{
			"inference",
			"geotagging",
			"scoring",
			"local_alerts",
			"local_dashboard",
			"offline_tiles",
		}
		var pausedOps []string
		if isOffline {
			pausedOps = []string{"remote_delivery", "cloud_sync"}
		}

		var queueDepth int
		var oldestItemAge float64
		var lastSyncStr *string

		if queue != nil {
			metricsCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			if metrics, err := queue.GetMetrics(metricsCtx); err == nil {
				queueDepth = metrics.QueueDepth
				oldestItemAge = metrics.OldestItemAgeSeconds
				if metrics.LastSuccessfulSync != nil {
					s := metrics.LastSuccessfulSync.UTC().Format(time.RFC3339Nano)
					lastSyncStr = &s
				}
			}
			cancel()
		}

		resp := HealthResponse{
			Status:               status,
			Service:              "fusion-service",
			Mode:                 mode,
			Postgres:             postgres,
			Redis:                redisStatus,
			QueueDepth:           queueDepth,
			OldestItemAgeSeconds: oldestItemAge,
			LastSuccessfulSync:   lastSyncStr,
			DegradedOperations: DegradedOperations{
				Active: activeOps,
				Paused: pausedOps,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
