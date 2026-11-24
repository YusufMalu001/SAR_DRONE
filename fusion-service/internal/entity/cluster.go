// Package entity manages geo-clustering of continuous detections across tracks and passes
// into unified Survivor and Hazard entities with running position estimates.
package entity

import (
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	"sar-drone-prototype/fusion-service/internal/geo"
	"sar-drone-prototype/fusion-service/internal/priority"
)

type EntityType string

const (
	TypeSurvivor EntityType = "survivor"
	TypeHazard   EntityType = "hazard"
)

type EntityStatus string

const (
	StatusActive   EntityStatus = "active"
	StatusRescued  EntityStatus = "rescued"
	StatusCleared  EntityStatus = "cleared"
)

// Entity represents a real-world person survivor or hazard zone.
type Entity struct {
	ID                string       `json:"id"`
	Type              EntityType   `json:"type"`  // survivor | hazard
	Class             string       `json:"class"` // person, fire, flood, debris, structural_damage
	Lat               float64      `json:"lat"`
	Lon               float64      `json:"lon"`
	UncertaintyRadius float64      `json:"uncertainty_radius"` // meters
	TrackIDs          []string     `json:"track_ids"`
	ObservationCount  int          `json:"observation_count"`
	Confidence        float64      `json:"confidence"`
	Modalities        []string     `json:"modalities"`
	PriorityScore     float64      `json:"priority_score"`
	Status            EntityStatus `json:"status"` // active | rescued | cleared
	FirstSeen         time.Time    `json:"first_seen"`
	LastSeen          time.Time    `json:"last_seen"`
	ResolvedAt        *time.Time   `json:"resolved_at,omitempty"`
	Notes             string       `json:"notes,omitempty"`
}

// ClusterConfig specifies association distance thresholds for grouping detections.
type ClusterConfig struct {
	SurvivorThresholdMeters float64 // Spatial clustering radius for survivors (~15m)
	HazardThresholdMeters   float64 // Spatial clustering radius for hazards (~35m)
	BaseUncertaintyMeters   float64 // Initial position uncertainty (default 6.0m)
	MinUncertaintyMeters    float64 // Minimum bound on position uncertainty (default 1.5m)
}

func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		SurvivorThresholdMeters: 5.0,
		HazardThresholdMeters:   35.0,
		BaseUncertaintyMeters:   6.0,
		MinUncertaintyMeters:    1.5,
	}
}

// Clusterer maintains the active set of entities and merges incoming detections.
type Clusterer struct {
	mu          sync.RWMutex
	cfg         ClusterConfig
	entities    map[string]*Entity // entity_id -> entity
	trackToID   map[string]string  // track_id -> entity_id
}

func NewClusterer(cfg ClusterConfig) *Clusterer {
	return &Clusterer{
		cfg:       cfg,
		entities:  make(map[string]*Entity),
		trackToID: make(map[string]string),
	}
}

// IngestDetection merges a detection into an existing Entity (via track_id or spatial proximity)
// or mints a new Entity. Returns the updated/created entity and a boolean indicating if it was newly minted.
func (c *Clusterer) IngestDetection(
	detID string,
	class string,
	conf float64,
	pt geo.Point,
	trackID string,
	modalities []string,
	at time.Time,
	nearHazardClass string,
	hazardDistM float64,
) (*Entity, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var matched *Entity

	// 1. First priority: match by continuous tracker track_id
	if trackID != "" {
		if entityID, exists := c.trackToID[trackID]; exists {
			if e, ok := c.entities[entityID]; ok && e.Status == StatusActive {
				matched = e
			}
		}
	}

	// 2. Second priority: cross-pass spatial clustering by class & proximity
	if matched == nil {
		threshold := c.cfg.SurvivorThresholdMeters
		if class != "person" {
			threshold = c.cfg.HazardThresholdMeters
		}

		bestDist := math.MaxFloat64
		for _, e := range c.entities {
			if e.Status != StatusActive || e.Class != class {
				continue
			}

			// If both have track IDs and were seen concurrently (within 10s),
			// they are distinct objects in the same frame/pass!
			if trackID != "" && len(e.TrackIDs) > 0 && !hasString(e.TrackIDs, trackID) {
				if at.Sub(e.LastSeen) < 10*time.Second {
					continue
				}
			}

			d := geo.HaversineMeters(pt, geo.Point{Lat: e.Lat, Lon: e.Lon})
			if d <= threshold && d < bestDist {
				bestDist = d
				matched = e
			}
		}
	}

	// 3. Existing entity update
	if matched != nil {
		wOld := math.Min(10.0, float64(matched.ObservationCount))
		wNew := math.Max(0.5, conf)

		// Running position update (recursive weighted average)
		matched.Lat = (matched.Lat*wOld + pt.Lat*wNew) / (wOld + wNew)
		matched.Lon = (matched.Lon*wOld + pt.Lon*wNew) / (wOld + wNew)

		matched.ObservationCount++
		matched.LastSeen = at
		matched.Confidence = math.Max(matched.Confidence, conf)

		// Shrink position uncertainty with more observations
		matched.UncertaintyRadius = math.Max(
			c.cfg.MinUncertaintyMeters,
			c.cfg.BaseUncertaintyMeters/math.Sqrt(float64(matched.ObservationCount)),
		)

		// Modality union
		matched.Modalities = mergeModalities(matched.Modalities, modalities)

		// Associate new track_id if applicable
		if trackID != "" {
			if !hasString(matched.TrackIDs, trackID) {
				matched.TrackIDs = append(matched.TrackIDs, trackID)
			}
			c.trackToID[trackID] = matched.ID
		}

		// Update priority score
		matched.PriorityScore = priority.Score(priority.ScoringParams{
			Class:            matched.Class,
			Confidence:       matched.Confidence,
			Modalities:       matched.Modalities,
			ObservationCount: matched.ObservationCount,
			NearHazardClass:  nearHazardClass,
			HazardDistanceM:  hazardDistM,
		})

		cp := *matched
		return &cp, false
	}

	// 4. Create new Entity
	entityType := TypeSurvivor
	if class != "person" {
		entityType = TypeHazard
	}

	newID := uuid.New().String()
	trackIDs := []string{}
	if trackID != "" {
		trackIDs = append(trackIDs, trackID)
	}

	mods := make([]string, len(modalities))
	copy(mods, modalities)
	if len(mods) == 0 {
		mods = []string{"rgb"}
	}

	pScore := priority.Score(priority.ScoringParams{
		Class:            class,
		Confidence:       conf,
		Modalities:       mods,
		ObservationCount: 1,
		NearHazardClass:  nearHazardClass,
		HazardDistanceM:  hazardDistM,
	})

	newEntity := &Entity{
		ID:                newID,
		Type:              entityType,
		Class:             class,
		Lat:               pt.Lat,
		Lon:               pt.Lon,
		UncertaintyRadius: c.cfg.BaseUncertaintyMeters,
		TrackIDs:          trackIDs,
		ObservationCount:  1,
		Confidence:        conf,
		Modalities:        mods,
		PriorityScore:     pScore,
		Status:            StatusActive,
		FirstSeen:         at,
		LastSeen:          at,
	}

	c.entities[newID] = newEntity
	if trackID != "" {
		c.trackToID[trackID] = newID
	}

	cp := *newEntity
	return &cp, true
}

// ResolveEntity marks an entity as rescued or cleared.
func (c *Clusterer) ResolveEntity(id string, status EntityStatus, notes string) (*Entity, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entities[id]
	if !ok {
		return nil, false
	}

	now := time.Now().UTC()
	e.Status = status
	e.ResolvedAt = &now
	e.Notes = notes

	cp := *e
	return &cp, true
}

// ListEntities returns a snapshot copy of all entities matching optional type or status filters.
func (c *Clusterer) ListEntities(entityType EntityType, status EntityStatus) []Entity {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Entity, 0, len(c.entities))
	for _, e := range c.entities {
		if entityType != "" && e.Type != entityType {
			continue
		}
		if status != "" && e.Status != status {
			continue
		}
		out = append(out, *e)
	}
	return out
}

// GetEntity returns a copy of the entity by ID.
func (c *Clusterer) GetEntity(id string) (*Entity, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entities[id]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

func mergeModalities(a, b []string) []string {
	set := make(map[string]struct{})
	for _, m := range a {
		set[m] = struct{}{}
	}
	for _, m := range b {
		set[m] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	return out
}

func hasString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
