package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/messaging"
)

func Health(pool *pgxpool.Pool, redisClient *redis.Client) http.HandlerFunc {
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

		if postgres != "up" || redisStatus != "up" {
			status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":   status,
			"service":  "fusion-service",
			"postgres": postgres,
			"redis":    redisStatus,
		})
	}
}
