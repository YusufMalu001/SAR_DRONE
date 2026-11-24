// Package alerting manages the lifecycle, hysteresis, cooldown, and escalation
// of alerts generated from high-priority entities and detections.
package alerting

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"sar-drone-prototype/fusion-service/internal/entity"
	"sar-drone-prototype/fusion-service/internal/priority"
)

type AlertStatus string

const (
	StatusActive    AlertStatus = "active"
	StatusEscalated AlertStatus = "escalated"
	StatusResolved  AlertStatus = "resolved"
)

type AlertAction string

const (
	ActionCreated    AlertAction = "created"
	ActionEscalated  AlertAction = "escalated"
	ActionRefreshed  AlertAction = "refreshed"
	ActionSuppressed AlertAction = "suppressed"
	ActionCleared    AlertAction = "cleared"
	ActionNoop       AlertAction = "noop"
)

type Alert struct {
	ID             string      `json:"id"`
	EntityID       string      `json:"entity_id"`
	DetectionID    string      `json:"detection_id,omitempty"`
	Tier           string      `json:"tier"` // critical | high | medium
	Message        string      `json:"message"`
	PriorityScore  float64     `json:"priority_score"`
	Status         AlertStatus `json:"status"`
	LastNotifiedAt time.Time   `json:"last_notified_at"`
	CreatedAt      time.Time   `json:"created_at"`
	ResolvedAt     *time.Time  `json:"resolved_at,omitempty"`
}

type Config struct {
	TriggerThreshold float64       // Score to trigger new alert (default 0.70)
	ClearThreshold   float64       // Score below which active alert is cleared (default 0.60)
	CooldownDuration time.Duration // Cooldown to suppress duplicate notifications (default 10s)
}

func DefaultConfig() Config {
	return Config{
		TriggerThreshold: 0.70,
		ClearThreshold:   0.60,
		CooldownDuration: 10 * time.Second,
	}
}

// Manager maintains alert state, enforcing hysteresis, cooldowns, and escalation.
type Manager struct {
	mu           sync.RWMutex
	cfg          Config
	activeByEnt  map[string]*Alert // entity_id -> active/escalated alert
	alertsByID   map[string]*Alert // alert_id -> alert
}

func NewManager(cfg Config) *Manager {
	return &Manager{
		cfg:         cfg,
		activeByEnt: make(map[string]*Alert),
		alertsByID:  make(map[string]*Alert),
	}
}

// Evaluate applies hysteresis, cooldown, and escalation logic to an entity finding.
func (m *Manager) Evaluate(
	e *entity.Entity,
	hazard *priority.HazardMatch,
	detID string,
	at time.Time,
) (*Alert, AlertAction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. If entity has been rescued or cleared, resolve any associated alert
	if e.Status == entity.StatusRescued || e.Status == entity.StatusCleared {
		if a, ok := m.activeByEnt[e.ID]; ok {
			a.Status = StatusResolved
			a.ResolvedAt = &at
			delete(m.activeByEnt, e.ID)
			cp := *a
			return &cp, ActionCleared
		}
		return nil, ActionNoop
	}

	existing, hasActive := m.activeByEnt[e.ID]

	// 2. Score >= TriggerThreshold: Alert active or escalating
	if e.PriorityScore >= m.cfg.TriggerThreshold {
		newTier := Tier(e.PriorityScore)
		msg := Message(e.Class, hazard)

		if hasActive {
			// Check for Escalation: higher tier OR significant score jump
			if tierWeight(newTier) > tierWeight(existing.Tier) || (e.PriorityScore-existing.PriorityScore >= 0.10) {
				existing.Tier = newTier
				existing.PriorityScore = e.PriorityScore
				existing.Message = msg
				existing.Status = StatusEscalated
				existing.LastNotifiedAt = at
				if detID != "" {
					existing.DetectionID = detID
				}
				cp := *existing
				return &cp, ActionEscalated
			}

			// Cooldown check for identical tier
			if at.Sub(existing.LastNotifiedAt) < m.cfg.CooldownDuration {
				cp := *existing
				return &cp, ActionSuppressed
			}

			// Cooldown expired: refresh notification
			existing.LastNotifiedAt = at
			existing.PriorityScore = e.PriorityScore
			existing.Message = msg
			if detID != "" {
				existing.DetectionID = detID
			}
			cp := *existing
			return &cp, ActionRefreshed
		}

		// Mint new Alert
		alertID := uuid.New().String()
		newAlert := &Alert{
			ID:             alertID,
			EntityID:       e.ID,
			DetectionID:    detID,
			Tier:           newTier,
			Message:        msg,
			PriorityScore:  e.PriorityScore,
			Status:         StatusActive,
			LastNotifiedAt: at,
			CreatedAt:      at,
		}
		m.activeByEnt[e.ID] = newAlert
		m.alertsByID[alertID] = newAlert
		cp := *newAlert
		return &cp, ActionCreated
	}

	// 3. Score < ClearThreshold: Clear active alert
	if e.PriorityScore < m.cfg.ClearThreshold {
		if hasActive {
			existing.Status = StatusResolved
			existing.ResolvedAt = &at
			delete(m.activeByEnt, e.ID)
			cp := *existing
			return &cp, ActionCleared
		}
		return nil, ActionNoop
	}

	// 4. In Hysteresis Band [ClearThreshold, TriggerThreshold):
	// Maintain current state to prevent flickering / thrashing.
	if hasActive {
		cp := *existing
		return &cp, ActionSuppressed
	}

	return nil, ActionNoop
}

// ResolveAlertByID explicitly resolves an alert by its ID (e.g. via REST API).
func (m *Manager) ResolveAlertByID(alertID string) (*Alert, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.alertsByID[alertID]
	if !ok {
		return nil, false
	}

	now := time.Now().UTC()
	a.Status = StatusResolved
	a.ResolvedAt = &now
	delete(m.activeByEnt, a.EntityID)

	cp := *a
	return &cp, true
}

// ListAlerts returns a snapshot of all alerts, optionally filtered by status.
func (m *Manager) ListAlerts(status AlertStatus) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Alert, 0, len(m.alertsByID))
	for _, a := range m.alertsByID {
		if status != "" && a.Status != status {
			continue
		}
		out = append(out, *a)
	}
	return out
}

// GetAlert returns a copy of an alert by ID.
func (m *Manager) GetAlert(id string) (*Alert, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.alertsByID[id]
	if !ok {
		return nil, false
	}
	cp := *a
	return &cp, true
}

func tierWeight(tier string) int {
	switch tier {
	case "critical":
		return 3
	case "high":
		return 2
	case "medium":
		return 1
	default:
		return 0
	}
}
