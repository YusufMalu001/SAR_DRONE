// Package fusion is the core pipeline: it reads Detection/Telemetry events
// off Redis streams, matches each detection to interpolated telemetry,
// geo-projects the detection into a lat/lon using a 3D camera ray-casting model,
// clusters detections into Survivor/Hazard entities, scores priority, persists them,
// manages alerts with hysteresis, and broadcasts versioned updates over WebSocket.
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
	"sar-drone-prototype/fusion-service/internal/coverage"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
	"sar-drone-prototype/fusion-service/internal/entity"
	"sar-drone-prototype/fusion-service/internal/geo"
	"sar-drone-prototype/fusion-service/internal/localqueue"
	"sar-drone-prototype/fusion-service/internal/priority"
	"sar-drone-prototype/fusion-service/internal/telemetry"
	"sar-drone-prototype/fusion-service/internal/types"
	"sar-drone-prototype/fusion-service/internal/ws"
)

// Broadcaster is the subset of ws.Hub the consumer needs.
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
	clusterer    *entity.Clusterer
	alertMgr     *alerting.Manager
	coverageGrid *coverage.Grid
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
	clusterer *entity.Clusterer,
	alertMgr *alerting.Manager,
	covGrid *coverage.Grid,
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
		clusterer:    clusterer,
		alertMgr:     alertMgr,
		coverageGrid: covGrid,
		queue:        queue,
		connectivity: monitor,
		cfg:          cfg,
	}
}

// Run blocks, consuming both streams via a Redis consumer group until ctx is canceled.
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
			Block:    500 * time.Millisecond,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				continue
			}
			log.Printf("fusion: XReadGroup: %v", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
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
			log.Printf("fusion: XAck %s/%s: %v", stream, msg.ID, err)
		}
	}()

	raw, ok := msg.Values["data"].(string)
	if !ok {
		log.Printf("fusion: message on %s missing 'data' field", stream)
		return
	}

	switch stream {
	case c.cfg.TelemetryCh:
		c.handleTelemetry(ctx, raw)
	case c.cfg.DetectionsCh:
		c.handleDetection(ctx, raw)
	default:
		log.Printf("fusion: unknown stream %q", stream)
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

	c.telemetryBuf.Add(telemetry.Sample{
		Lat:     t.Lat,
		Lon:     t.Lon,
		Alt:     t.Alt,
		Heading: t.Heading,
		Pitch:   t.Pitch,
		Roll:    t.Roll,
		Yaw:     t.Heading,
		At:      at,
	})

	if c.coverageGrid != nil {
		c.coverageGrid.RecordTelemetry(t.Lat, t.Lon, t.Alt, c.cfg.CameraHFOVDeg, c.cfg.CameraVFOVDeg, at)
	}

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

	// 1. Telemetry association: Interpolate position/attitude bracketing frame timestamp
	tolerance := time.Duration(c.cfg.TelemetryToleranceMS) * time.Millisecond
	sample, err := c.telemetryBuf.Interpolate(detectedAt, 1*time.Second, tolerance)
	if err != nil {
		var ok bool
		sample, ok = c.telemetryBuf.Nearest(detectedAt, tolerance)
		if !ok {
			slog.WarnContext(ctx, "dropping detection: no telemetry within tolerance",
				slog.String("detection_id", d.ID),
				slog.String("frame_id", d.FrameID),
				slog.String("tolerance", tolerance.String()),
			)
			return
		}
	}

	// 2. Full 3D Camera Ray-Casting Projection
	u, v := geo.GroundContactPoint(geo.BBox{
		X: d.BBox.X,
		Y: d.BBox.Y,
		W: d.BBox.W,
		H: d.BBox.H,
	}, d.Class)

	cam := geo.CameraModel{
		HFOVDeg: c.cfg.CameraHFOVDeg,
		VFOVDeg: c.cfg.CameraVFOVDeg,
	}
	pose := geo.DronePose{
		Lat:        sample.Lat,
		Lon:        sample.Lon,
		Alt:        sample.Alt,
		HeadingDeg: sample.Heading,
		PitchDeg:   sample.Pitch,
		RollDeg:    sample.Roll,
	}

	point, err := geo.ProjectRay(u, v, cam, pose)
	if err != nil {
		point = geo.ProjectDetection(
			d.BBox.X+d.BBox.W/2,
			d.BBox.Y+d.BBox.H/2,
			c.cfg.CameraHFOVDeg,
			c.cfg.CameraVFOVDeg,
			sample.Alt,
			sample.Heading,
			sample.Lat,
			sample.Lon,
		)
	}

	// 3. Hazard Proximity Evaluation
	match, matchFound := c.hazards.Nearest(point, detectedAt)
	nearHazard := matchFound && match.DistanceMeters <= c.cfg.HazardProximityMeters
	hazardClass := ""
	hazardDist := 0.0
	var hazardMatch *priority.HazardMatch
	if nearHazard {
		hazardClass = match.Class
		hazardDist = match.DistanceMeters
		hazardMatch = &match
	}
	c.hazards.Record(d.Class, point, detectedAt)

	// 4. Geo-Clustering into Survivor / Hazard Entity
	ent, _ := c.clusterer.IngestDetection(
		d.ID,
		d.Class,
		d.Confidence,
		point,
		d.TrackID,
		d.Modalities,
		detectedAt,
		hazardClass,
		hazardDist,
	)

	score := ent.PriorityScore

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
		TrackID:       d.TrackID,
		Modalities:    d.Modalities,
	}

	slog.InfoContext(ctx, "fused detection",
		slog.String("detection_id", d.ID),
		slog.String("entity_id", ent.ID),
		slog.String("frame_id", d.FrameID),
		slog.String("class", d.Class),
		slog.Float64("priority_score", score),
	)

	if c.coverageGrid != nil {
		c.coverageGrid.RecordDetection(fd.Lat, fd.Lon, d.Confidence)
	}

	// Offline write-ahead queue
	if err := c.queue.Enqueue(ctx, fd); err != nil {
		log.Printf("fusion: %v", err)
		return
	}

	if c.connectivity.IsOffline() {
		c.broadcastDetection(fd, dto.StatusQueued)
		c.broadcastEntity(ent)
		return
	}

	insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = db.InsertDetection(insertCtx, c.pool, fd)
	cancel()
	if err != nil {
		log.Printf("fusion: insert detection %s failed, left queued for retry: %v", fd.ID, err)
		c.broadcastDetection(fd, dto.StatusQueued)
		c.broadcastEntity(ent)
		return
	}

	if err := c.queue.Delete(ctx, fd.ID); err != nil {
		log.Printf("fusion: %v", err)
	}

	// Persist Entity
	entCtx, cancelEnt := context.WithTimeout(ctx, 5*time.Second)
	_ = db.UpsertEntity(entCtx, c.pool, db.EntityRow{
		ID:                ent.ID,
		Type:              string(ent.Type),
		Class:             ent.Class,
		Lat:               ent.Lat,
		Lon:               ent.Lon,
		UncertaintyRadius: ent.UncertaintyRadius,
		TrackIDs:          ent.TrackIDs,
		ObservationCount:  ent.ObservationCount,
		Confidence:        ent.Confidence,
		Modalities:        ent.Modalities,
		PriorityScore:     ent.PriorityScore,
		Status:            string(ent.Status),
		FirstSeen:         ent.FirstSeen,
		LastSeen:          ent.LastSeen,
	})
	cancelEnt()

	// Update hazard polygons
	if priority.IsHazardClass(ent.Class) {
		polyCtx, cancelPoly := context.WithTimeout(ctx, 5*time.Second)
		_ = db.RecomputeHazardPolygons(polyCtx, c.pool)
		cancelPoly()
	}

	// 5. Stateful Alert Management (hysteresis, cooldown, escalation)
	alertObj, action := c.alertMgr.Evaluate(ent, hazardMatch, d.ID, detectedAt)
	if action == alerting.ActionCreated || action == alerting.ActionEscalated || action == alerting.ActionRefreshed {
		alertCtx, cancelAlert := context.WithTimeout(ctx, 5*time.Second)
		_, _ = db.UpsertAlertRecord(alertCtx, c.pool, db.Alert{
			AlertID:        alertObj.ID,
			EntityID:       alertObj.EntityID,
			DetectionID:    alertObj.DetectionID,
			PriorityTier:   alertObj.Tier,
			Message:        alertObj.Message,
			PriorityScore:  alertObj.PriorityScore,
			Status:         string(alertObj.Status),
			LastNotifiedAt: alertObj.LastNotifiedAt,
			CreatedAt:      alertObj.CreatedAt,
		})
		cancelAlert()
		c.broadcastAlert(alertObj)
	}

	c.broadcastDetection(fd, dto.StatusLive)
	c.broadcastEntity(ent)
}

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
			return
		}

		insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := db.InsertDetection(insertCtx, c.pool, fd)
		cancel()
		if err != nil {
			log.Printf("fusion: drain: insert %s failed, will retry: %v", fd.ID, err)
			return
		}

		if err := c.queue.Delete(ctx, fd.ID); err != nil {
			log.Printf("fusion: drain: %v", err)
		}
		c.broadcastDetection(fd, dto.StatusSyncedFromOfflineQueue)

		point := geo.Point{Lat: fd.Lat, Lon: fd.Lon}
		match, matchFound := c.hazards.Nearest(point, fd.DetectedAt)
		nearHazard := matchFound && match.DistanceMeters <= c.cfg.HazardProximityMeters
		hazardClass := ""
		hazardDist := 0.0
		var hazardMatch *priority.HazardMatch
		if nearHazard {
			hazardClass = match.Class
			hazardDist = match.DistanceMeters
			hazardMatch = &match
		}
		c.hazards.Record(fd.Class, point, fd.DetectedAt)

		ent, _ := c.clusterer.IngestDetection(
			fd.ID,
			fd.Class,
			fd.Confidence,
			point,
			fd.TrackID,
			fd.Modalities,
			fd.DetectedAt,
			hazardClass,
			hazardDist,
		)

		_ = db.UpsertEntity(ctx, c.pool, db.EntityRow{
			ID:                ent.ID,
			Type:              string(ent.Type),
			Class:             ent.Class,
			Lat:               ent.Lat,
			Lon:               ent.Lon,
			UncertaintyRadius: ent.UncertaintyRadius,
			TrackIDs:          ent.TrackIDs,
			ObservationCount:  ent.ObservationCount,
			Confidence:        ent.Confidence,
			Modalities:        ent.Modalities,
			PriorityScore:     ent.PriorityScore,
			Status:            string(ent.Status),
			FirstSeen:         ent.FirstSeen,
			LastSeen:          ent.LastSeen,
		})

		alertObj, action := c.alertMgr.Evaluate(ent, hazardMatch, fd.ID, fd.DetectedAt)
		if action == alerting.ActionCreated || action == alerting.ActionEscalated || action == alerting.ActionRefreshed {
			_, _ = db.UpsertAlertRecord(ctx, c.pool, db.Alert{
				AlertID:        alertObj.ID,
				EntityID:       alertObj.EntityID,
				DetectionID:    alertObj.DetectionID,
				PriorityTier:   alertObj.Tier,
				Message:        alertObj.Message,
				PriorityScore:  alertObj.PriorityScore,
				Status:         string(alertObj.Status),
				LastNotifiedAt: alertObj.LastNotifiedAt,
				CreatedAt:      alertObj.CreatedAt,
			})
			c.broadcastAlert(alertObj)
		}
		c.broadcastEntity(ent)
	}
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
		TrackID:       fd.TrackID,
		Modalities:    fd.Modalities,
	}

	bytes, err := dto.NewWSMessage(dto.WSMessageDetection, record)
	if err != nil {
		log.Printf("fusion: marshal detection ws message: %v", err)
		return
	}
	c.hub.Broadcast(bytes)
}

func (c *Consumer) broadcastEntity(e *entity.Entity) {
	record := dto.EntityRecord{
		ID:                e.ID,
		Type:              string(e.Type),
		Class:             e.Class,
		Lat:               e.Lat,
		Lon:               e.Lon,
		UncertaintyRadius: e.UncertaintyRadius,
		TrackIDs:          e.TrackIDs,
		ObservationCount:  e.ObservationCount,
		Confidence:        e.Confidence,
		Modalities:        e.Modalities,
		PriorityScore:     e.PriorityScore,
		Status:            string(e.Status),
		FirstSeen:         e.FirstSeen,
		LastSeen:          e.LastSeen,
		ResolvedAt:        e.ResolvedAt,
		Notes:             e.Notes,
	}
	bytes, err := dto.NewWSMessage(dto.WSMessageEntityUpdate, record)
	if err != nil {
		log.Printf("fusion: marshal entity ws message: %v", err)
		return
	}
	c.hub.Broadcast(bytes)
}

func (c *Consumer) broadcastAlert(a *alerting.Alert) {
	record := dto.AlertRecord{
		ID:             a.ID,
		EntityID:       a.EntityID,
		DetectionID:    a.DetectionID,
		PriorityTier:   a.Tier,
		Message:        a.Message,
		PriorityScore:  a.PriorityScore,
		Status:         string(a.Status),
		CreatedAt:      a.CreatedAt,
		LastNotifiedAt: a.LastNotifiedAt,
		ResolvedAt:     a.ResolvedAt,
	}

	bytes, err := dto.NewWSMessage(dto.WSMessageAlert, record)
	if err != nil {
		log.Printf("fusion: marshal alert ws message: %v", err)
		return
	}
	c.hub.Broadcast(bytes)
}
