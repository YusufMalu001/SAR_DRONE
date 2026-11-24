//go:build integration

package integration

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/config"
	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/entity"
	"sar-drone-prototype/fusion-service/internal/fusion"
	"sar-drone-prototype/fusion-service/internal/localqueue"
	migraterunner "sar-drone-prototype/fusion-service/internal/migrate"
	"sar-drone-prototype/fusion-service/internal/migrations"
	"sar-drone-prototype/fusion-service/internal/priority"
	"sar-drone-prototype/fusion-service/internal/server"
	"sar-drone-prototype/fusion-service/internal/telemetry"
	"sar-drone-prototype/fusion-service/internal/ws"
)

// fastTestInterval is used for the connectivity monitor's ping loop and the
// drain loop in tests, so offline->online recovery is observed quickly
// instead of waiting on the multi-second production defaults.
const fastTestInterval = 200 * time.Millisecond

func testConfig(t *testing.T) config.Config {
	t.Helper()

	getenv := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	return config.Config{
		Port:                  "0",
		RedisAddr:             getenv("REDIS_ADDR", "localhost:6379"),
		DatabaseURL:           getenv("DATABASE_URL", "postgres://sar:sar@localhost:5432/sar?sslmode=disable"),
		DetectionsCh:          "detections",
		TelemetryCh:           "telemetry",
		ConsumerGroup:         "fusion-service-integration-test",
		ConsumerName:          "test-consumer",
		TelemetryToleranceMS:  200,
		CameraHFOVDeg:         90,
		CameraVFOVDeg:         60,
		HazardProximityMeters: 50,
		HazardWindowSeconds:   60,
		HighPriorityThreshold: 0.7,

		RouteObstructionMeters:    20,
		RouteStatusTelemetryLimit: 5000,
	}
}

// harness wires up a full fusion-service instance (consumer, drain loop,
// connectivity monitor, HTTP server) against real Postgres/Redis and a
// throwaway per-test SQLite queue file, and registers cleanup for all of
// it via t.Cleanup.
type harness struct {
	cfg         config.Config
	pool        *pgxpool.Pool
	redisClient *redis.Client
	queue       *localqueue.Queue
	monitor     *connectivity.Monitor
	hub         *ws.Hub
	consumer    *fusion.Consumer
	httpServer  *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	cfg := testConfig(t)
	cfg.LocalQueuePath = filepath.Join(t.TempDir(), "fusion-queue.db")

	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelSetup()

	if err := migraterunner.Run(cfg.DatabaseURL, migrations.FS); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	pool, err := db.NewPool(setupCtx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect to postgres (is `docker compose up -d postgres` running?): %v", err)
	}
	if err := pool.Ping(setupCtx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(setupCtx).Err(); err != nil {
		t.Fatalf("connect to redis (is `docker compose up -d redis` running?): %v", err)
	}

	queue, err := localqueue.Open(cfg.LocalQueuePath)
	if err != nil {
		t.Fatalf("open local queue: %v", err)
	}

	monitor := connectivity.NewMonitor(pool)

	hub := ws.NewHub()
	telemetryBuf := telemetry.NewBuffer(2*time.Minute, 100)
	hazards := priority.NewHazardTracker(time.Duration(cfg.HazardWindowSeconds)*time.Second, cfg.HazardProximityMeters)
	clusterer := entity.NewClusterer(entity.DefaultClusterConfig())
	alertMgr := alerting.NewManager(alerting.DefaultConfig())
	consumer := fusion.NewConsumer(redisClient, pool, hub, telemetryBuf, hazards, clusterer, alertMgr, queue, monitor, cfg)

	runCtx, stopRun := context.WithCancel(context.Background())
	go monitor.Run(runCtx, fastTestInterval)
	go func() { _ = consumer.Run(runCtx) }()
	go consumer.RunDrainLoop(runCtx, fastTestInterval)

	httpServer := httptest.NewServer(server.NewMux(pool, redisClient, hub, queue, monitor, clusterer, alertMgr, cfg))

	t.Cleanup(func() {
		httpServer.Close()
		stopRun()
		_ = queue.Close()
		_ = redisClient.Close()
		pool.Close()
	})

	return &harness{
		cfg:         cfg,
		pool:        pool,
		redisClient: redisClient,
		queue:       queue,
		monitor:     monitor,
		hub:         hub,
		consumer:    consumer,
		httpServer:  httpServer,
	}
}
