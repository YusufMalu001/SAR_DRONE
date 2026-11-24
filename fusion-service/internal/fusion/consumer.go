// Package fusion is the core pipeline: it reads Detection/Telemetry events
// off Redis streams, matches each detection to the nearest-in-time
// telemetry sample, geo-projects the detection into a lat/lon, scores its
// priority, persists it, generates an alert if it's high-priority enough,
// and broadcasts both to WebSocket clients.
//
// Known tradeoff: for a detection drained from the offline queue
// (RunDrainLoop), the hazard-proximity context used for its alert message
// is re-evaluated against whatever the HazardTracker holds *at drain time*
// rather than what was actually nearby when the detection was first seen —
// that original context isn't persisted through the SQLite queue. In
// practice this only matters if the drain is delayed long enough for the
// tracker's window to have moved on.
package fusion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/config"
	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
	"sar-drone-prototype/fusion-service/internal/geo"
	"sar-drone-prototype/fusion-service/internal/localqueue"
	"sar-drone-prototype/fusion-service/internal/priority"
	"sar-drone-prototype/fusion-service/internal/telemetry"
	"sar-drone-prototype/fusion-service/internal/types"
	"sar-drone-prototype/fusion-service/internal/ws"
)

// Broadcaster is the subset of ws.Hub the consumer needs — narrowed so
// tests can supply a stub.
type Broadcaster interface {
	Broadcast(payload []byte)
}

var _ Broadcaster = (*ws.Hub)(nil)

// Consumer runs the Redis-streams-to-Postgres fusion pipeline.
type Consumer struct {
	redis        *redis.Client
	pool         *pgxpool.Pool
	hub          Broadcaster
	telemetryBuf *telemetry.Buffer
	hazards      *priority.HazardTracker
	queue        *localqueue.Queue
	connectivity *connectivity.Monitor
	cfg          config.Config
}

func NewConsumer(
	redisClient *redis.Client,
	pool *pgxpool.Pool,
	hub Broadcaster,
	telemetryBuf *telemetry.Buffer,
	hazards *priority.HazardTracker,
	queue *localqueue.Queue,
	monitor *connectivity.Monitor,
	cfg config.Config,
) *Consumer {
	return &Consumer{
		redis:        redisClient,
		pool:         pool,
		hub:          hub,
		telemetryBuf: telemetryBuf,
		hazards:      hazards,
		queue:        queue,
		connectivity: monitor,
		cfg:          cfg,
	}
}

// Run blocks, consuming both streams via a Redis consumer group until ctx
// is canceled.
func (c *Consumer) Run(ctx context.Context) error {
	for _, stream := range []string{c.cfg.DetectionsCh, c.cfg.TelemetryCh} {
		err := c.redis.XGroupCreateMkStream(ctx, stream, c.cfg.ConsumerGroup, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("create consumer group for stream %s: %w", stream, err)
		}
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		result, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.cfg.ConsumerGroup,
			Consumer: c.cfg.ConsumerName,
			Streams:  []string{c.cfg.DetectionsCh, c.cfg.TelemetryCh, ">", ">"},
			Count:    20,
			Block:    5 * time.Second,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue // block timeout, nothing new
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("fusion: xreadgroup error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _, stream := range result {
			for _, msg := range stream.Messages {
				c.handleMessage(ctx, stream.Stream, msg)
			}
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, stream string, msg redis.XMessage) {
	defer func() {
		if err := c.redis.XAck(ctx, stream, c.cfg.ConsumerGroup, msg.ID).Err(); err != nil {
			log.Printf("fusion: xack failed for %s/%s: %v", stream, msg.ID, err)
		}
	}()

	raw, ok := msg.Values["data"].(string)
	if !ok {
		log.Printf("fusion: message %s/%s missing string \"data\" field", stream, msg.ID)
		return
	}

	switch stream {
	case c.cfg.TelemetryCh:
		c.handleTelemetry(ctx, raw)
	case c.cfg.DetectionsCh:
		c.handleDetection(ctx, raw)
	}
}

func (c *Consumer) handleTelemetry(ctx context.Context, raw string) {
	var t types.Telemetry
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		log.Printf("fusion: bad telemetry payload: %v", err)
		return
	}

	at, err := time.Parse(time.RFC3339Nano, t.Timestamp)
	if err != nil {
		log.Printf("fusion: bad telemetry timestamp %q: %v", t.Timestamp, err)
		return
	}

	c.telemetryBuf.Add(telemetry.Sample{Lat: t.Lat, Lon: t.Lon, Alt: t.Alt, Heading: t.Heading, At: at})

	// Best-effort: the recorded flight path (GET /mission/route-status) is
	// a secondary feature, not the primary detection pipeline, so a
	// Postgres hiccup here is logged rather than sent through the offline
	// queue like detections are.
	insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.InsertTelemetry(insertCtx, c.pool, db.TelemetryRow{
		Lat: t.Lat, Lon: t.Lon, Alt: t.Alt, Heading: t.Heading, RecordedAt: at,
	}); err != nil {
		log.Printf("fusion: %v", err)
	}
}

func (c *Consumer) handleDetection(ctx context.Context, raw string) {
	var d types.Detection
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		log.Printf("fusion: bad detection payload: %v", err)
		return
	}

	detectedAt, err := time.Parse(time.RFC3339Nano, d.Timestamp)
	if err != nil {
		log.Printf("fusion: bad detection timestamp %q: %v", d.Timestamp, err)
		return
	}

	tolerance := time.Duration(c.cfg.TelemetryToleranceMS) * time.Millisecond
	sample, ok := c.telemetryBuf.Nearest(detectedAt, tolerance)
	if !ok {
		slog.WarnContext(ctx, "dropping detection: no telemetry within tolerance",
			slog.String("detection_id", d.ID),
			slog.String("frame_id", d.FrameID),
			slog.String("tolerance", tolerance.String()),
		)
		return
	}

	point := geo.ProjectDetection(
		d.BBox.X+d.BBox.W/2,
		d.BBox.Y+d.BBox.H/2,
		c.cfg.CameraHFOVDeg,
		c.cfg.CameraVFOVDeg,
		sample.Alt,
		sample.Heading,
		sample.Lat,
		sample.Lon,
	)

	// Captured before Record() below so a hazard-class detection never
	// matches itself as "the nearest hazard" — both for the priority boost
	// and for the alert message.
	match, matchFound := c.hazards.Nearest(point, detectedAt)
	nearHazard := matchFound && match.DistanceMeters <= c.cfg.HazardProximityMeters
	score := priority.Score(d.Class, d.Confidence, nearHazard)
	c.hazards.Record(d.Class, point, detectedAt)

	var hazardMatch *priority.HazardMatch
	if nearHazard {
		hazardMatch = &match
	}

	fd := db.FusedDetection{
		ID:            d.ID,
		Class:         d.Class,
		Confidence:    d.Confidence,
		BBoxX:         d.BBox.X,
		BBoxY:         d.BBox.Y,
		BBoxW:         d.BBox.W,
		BBoxH:         d.BBox.H,
		FrameID:       d.FrameID,
		Source:        d.Source,
		DetectedAt:    detectedAt,
		TelemetryAt:   sample.At,
		Lat:           point.Lat,
		Lon:           point.Lon,
		PriorityScore: score,
	}

	slog.InfoContext(ctx, "fused detection",
		slog.String("detection_id", d.ID),
		slog.String("frame_id", d.FrameID),
		slog.String("class", d.Class),
		slog.Float64("priority_score", score),
	)

	// Offline buffering: SQLite is written FIRST, always — Postgres (or a
	// future external/cloud sync) is only attempted after this succeeds,
	// so nothing is lost if that next step is unreachable.
	if err := c.queue.Enqueue(ctx, fd); err != nil {
		log.Printf("fusion: %v", err)
		return
	}

	if c.connectivity.IsOffline() {
		c.broadcastDetection(fd, dto.StatusQueued)
		return
	}

	insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = db.InsertDetection(insertCtx, c.pool, fd)
	cancel()
	if err != nil {
		log.Printf("fusion: insert detection %s failed, left queued for retry: %v", fd.ID, err)
		c.broadcastDetection(fd, dto.StatusQueued)
		return
	}

	if err := c.queue.Delete(ctx, fd.ID); err != nil {
		log.Printf("fusion: %v", err)
	}
	c.broadcastDetection(fd, dto.StatusLive)
	c.generateAlert(ctx, fd, hazardMatch)
}

// RunDrainLoop drains the local offline queue into Postgres whenever
// connectivity is (or becomes) available: on a timer as a fallback, and
// immediately whenever the connectivity monitor reports an offline->online
// transition.
func (c *Consumer) RunDrainLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.drainIfOnline(ctx)
		case <-c.connectivity.BecameOnline():
			c.drainIfOnline(ctx)
		}
	}
}

func (c *Consumer) drainIfOnline(ctx context.Context) {
	if c.connectivity.IsOffline() {
		return
	}

	items, err := c.queue.ListOrdered(ctx)
	if err != nil {
		log.Printf("fusion: drain: %v", err)
		return
	}

	for _, fd := range items {
		if c.connectivity.IsOffline() {
			return // connectivity dropped mid-drain; resume next trigger
		}

		insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := db.InsertDetection(insertCtx, c.pool, fd)
		cancel()
		if err != nil {
			log.Printf("fusion: drain: insert %s failed, will retry: %v", fd.ID, err)
			return // stop this pass; earlier items are unordered relative to fd otherwise
		}

		if err := c.queue.Delete(ctx, fd.ID); err != nil {
			log.Printf("fusion: drain: %v", err)
		}
		c.broadcastDetection(fd, dto.StatusSyncedFromOfflineQueue)

		// The original hazard-proximity context from when this detection
		// was first processed isn't preserved through the queue, so this
		// is a best-effort re-evaluation against whatever the tracker
		// currently holds — see the package doc comment for the tradeoff.
		point := geo.Point{Lat: fd.Lat, Lon: fd.Lon}
		match, matchFound := c.hazards.Nearest(point, fd.DetectedAt)
		nearHazard := matchFound && match.DistanceMeters <= c.cfg.HazardProximityMeters
		c.hazards.Record(fd.Class, point, fd.DetectedAt)

		var hazardMatch *priority.HazardMatch
		if nearHazard {
			hazardMatch = &match
		}
		c.generateAlert(ctx, fd, hazardMatch)
	}
}

// generateAlert inserts and broadcasts an Alert if fd's priority score
// clears the alerting threshold. hazardMatch is the hazard (if any) that
// was nearby at the time fd was originally scored/processed — nil for an
// isolated detection or one with no hazard within HazardProximityMeters.
func (c *Consumer) generateAlert(ctx context.Context, fd db.FusedDetection, hazardMatch *priority.HazardMatch) {
	if fd.PriorityScore <= alerting.AlertThreshold {
		return
	}

	alert := db.Alert{
		DetectionID:  fd.ID,
		PriorityTier: alerting.Tier(fd.PriorityScore),
		Message:      alerting.Message(fd.Class, hazardMatch),
	}

	insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	inserted, err := db.InsertAlert(insertCtx, c.pool, alert)
	cancel()
	if err != nil {
		log.Printf("fusion: insert alert for detection %s: %v", fd.ID, err)
		return
	}
	if inserted == nil {
		return // an alert already exists for this detection
	}

	c.broadcastAlert(*inserted)
}

func (c *Consumer) broadcastDetection(fd db.FusedDetection, status string) {
	record := dto.DetectionRecord{
		ID:            fd.ID,
		Class:         fd.Class,
		Confidence:    fd.Confidence,
		Lat:           fd.Lat,
		Lon:           fd.Lon,
		FrameID:       fd.FrameID,
		Source:        fd.Source,
		DetectedAt:    fd.DetectedAt,
		PriorityScore: fd.PriorityScore,
		Status:        status,
	}

	envelope, err := dto.NewWSMessage(dto.WSMessageDetection, record)
	if err != nil {
		log.Printf("fusion: %v", err)
		return
	}
	c.hub.Broadcast(envelope)
}

func (c *Consumer) broadcastAlert(alert db.Alert) {
	record := dto.AlertRecord{
		ID:           alert.ID,
		DetectionID:  alert.DetectionID,
		PriorityTier: alert.PriorityTier,
		Message:      alert.Message,
		CreatedAt:    alert.CreatedAt,
	}

	envelope, err := dto.NewWSMessage(dto.WSMessageAlert, record)
	if err != nil {
		log.Printf("fusion: %v", err)
		return
	}
	c.hub.Broadcast(envelope)
}
