package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sar-drone-prototype/fusion-service/internal/config"
	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/coverage"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/entity"
	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/fusion"
	"sar-drone-prototype/fusion-service/internal/localqueue"
	"sar-drone-prototype/fusion-service/internal/logger"
	migraterunner "sar-drone-prototype/fusion-service/internal/migrate"
	"sar-drone-prototype/fusion-service/internal/migrations"
	"sar-drone-prototype/fusion-service/internal/messaging"
	"sar-drone-prototype/fusion-service/internal/priority"
	"sar-drone-prototype/fusion-service/internal/server"
	"sar-drone-prototype/fusion-service/internal/telemetry"
	"sar-drone-prototype/fusion-service/internal/ws"
)

func main() {
	logger.Init()
	cfg := config.Load()

	if err := migraterunner.Run(cfg.DatabaseURL, migrations.FS); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to create postgres pool: %v", err)
	}
	defer pool.Close()

	redisClient := messaging.NewClient(cfg.RedisAddr)
	defer redisClient.Close()

	queue, err := localqueue.Open(cfg.LocalQueuePath)
	if err != nil {
		log.Fatalf("failed to open local offline queue: %v", err)
	}
	defer queue.Close()

	monitor := connectivity.NewMonitor(pool)
	go monitor.Run(ctx, time.Duration(cfg.ConnectivityCheckIntervalMS)*time.Millisecond)

	hub := ws.NewHub()
	telemetryBuf := telemetry.NewBuffer(2*time.Minute, 2000)
	hazards := priority.NewHazardTracker(
		time.Duration(cfg.HazardWindowSeconds)*time.Second,
		cfg.HazardProximityMeters,
	)
	clusterer := entity.NewClusterer(entity.DefaultClusterConfig())
	alertMgr := alerting.NewManager(alerting.DefaultConfig())
	covGrid := coverage.NewGrid(18.0, 72.0, 29.0, 79.0, 20.0)

	consumer := fusion.NewConsumer(redisClient, pool, hub, telemetryBuf, hazards, clusterer, alertMgr, covGrid, queue, monitor, cfg)
	go func() {
		if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("fusion consumer stopped unexpectedly: %v", err)
		}
	}()
	go consumer.RunDrainLoop(ctx, time.Duration(cfg.DrainIntervalMS)*time.Millisecond)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: server.NewMux(pool, redisClient, hub, queue, monitor, clusterer, alertMgr, covGrid, cfg),
	}

	go func() {
		log.Printf("fusion-service listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("fusion-service shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown error: %v", err)
	}
}
