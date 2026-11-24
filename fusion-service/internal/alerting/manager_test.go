package alerting

import (
	"testing"
	"time"

	"sar-drone-prototype/fusion-service/internal/entity"
)

func TestAlertManager_HysteresisAndCooldown(t *testing.T) {
	cfg := Config{
		TriggerThreshold: 0.70,
		ClearThreshold:   0.60,
		CooldownDuration: 10 * time.Second,
	}
	mgr := NewManager(cfg)
	now := time.Now()

	e := &entity.Entity{
		ID:            "ent-1",
		Class:         "person",
		PriorityScore: 0.75,
		Status:        entity.StatusActive,
	}

	// 1. Initial trigger above threshold (0.75 >= 0.70) -> Created
	a1, action := mgr.Evaluate(e, nil, "det-1", now)
	if action != ActionCreated || a1 == nil {
		t.Fatalf("expected ActionCreated, got %v", action)
	}
	if a1.Status != StatusActive || a1.Tier != "medium" {
		t.Errorf("unexpected alert: %+v", a1)
	}

	// 2. Cooldown check: 2 seconds later at same score -> Suppressed
	_, action = mgr.Evaluate(e, nil, "det-2", now.Add(2*time.Second))
	if action != ActionSuppressed {
		t.Errorf("expected ActionSuppressed during cooldown, got %v", action)
	}

	// 3. Hysteresis band test: score dips to 0.65 (< 0.70 trigger, but >= 0.60 clear)
	// Alert must NOT be cleared!
	e.PriorityScore = 0.65
	aHyst, action := mgr.Evaluate(e, nil, "det-3", now.Add(5*time.Second))
	if action != ActionSuppressed || aHyst.Status != StatusActive {
		t.Errorf("expected alert to remain active in hysteresis band [0.60, 0.70), got action=%v status=%v",
			action, aHyst.Status)
	}

	// 4. Escalation test: score jumps to 0.92 (Critical tier) at t=6s (within cooldown!)
	// Escalation must bypass cooldown!
	e.PriorityScore = 0.92
	aEsc, action := mgr.Evaluate(e, nil, "det-4", now.Add(6*time.Second))
	if action != ActionEscalated {
		t.Fatalf("expected ActionEscalated, got %v", action)
	}
	if aEsc.Tier != "critical" || aEsc.Status != StatusEscalated {
		t.Errorf("expected critical escalated tier, got tier=%s status=%s", aEsc.Tier, aEsc.Status)
	}

	// 5. Clear threshold test: score drops to 0.55 (< 0.60 clear threshold)
	e.PriorityScore = 0.55
	aClr, action := mgr.Evaluate(e, nil, "det-5", now.Add(15*time.Second))
	if action != ActionCleared {
		t.Fatalf("expected ActionCleared below clear threshold, got %v", action)
	}
	if aClr.Status != StatusResolved || aClr.ResolvedAt == nil {
		t.Errorf("expected alert resolved on clear, got %+v", aClr)
	}
}

func TestAlertManager_EntityResolution(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	now := time.Now()

	e := &entity.Entity{
		ID:            "ent-rescue",
		Class:         "person",
		PriorityScore: 0.85,
		Status:        entity.StatusActive,
	}

	// Raise alert
	_, action := mgr.Evaluate(e, nil, "det-1", now)
	if action != ActionCreated {
		t.Fatalf("expected alert created")
	}

	// Entity is confirmed rescued
	e.Status = entity.StatusRescued
	aRes, action := mgr.Evaluate(e, nil, "det-2", now.Add(5*time.Second))
	if action != ActionCleared {
		t.Fatalf("expected alert cleared on entity rescue, got %v", action)
	}
	if aRes.Status != StatusResolved {
		t.Errorf("expected alert resolved")
	}

	// API resolution test
	e2 := &entity.Entity{
		ID:            "ent-api",
		Class:         "fire",
		PriorityScore: 0.85,
		Status:        entity.StatusActive,
	}
	a2, _ := mgr.Evaluate(e2, nil, "det-3", now)
	resolved, ok := mgr.ResolveAlertByID(a2.ID)
	if !ok || resolved.Status != StatusResolved {
		t.Errorf("failed to resolve alert via ID: %+v", resolved)
	}
}
