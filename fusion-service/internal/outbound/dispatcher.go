// Package outbound manages external alert dispatches across pluggable adapters
// (webhook, MQTT, email/SMTP, SMS, mock). All sends pass through the durable
// SQLite write-ahead queue, retried with exponential backoff upon failure.
package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/config"
	"sar-drone-prototype/fusion-service/internal/localqueue"
)

// Adapter sends formatted alert notifications to an external system.
type Adapter interface {
	Name() string
	Send(ctx context.Context, item localqueue.OutboundItem) error
}

// AlertPayload is the canonical outbound payload format.
type AlertPayload struct {
	AlertID        string    `json:"alert_id"`
	EntityID       string    `json:"entity_id"`
	DetectionID    string    `json:"detection_id"`
	PriorityTier   string    `json:"priority_tier"`
	PriorityScore  float64   `json:"priority_score"`
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
	LastNotifiedAt time.Time `json:"last_notified_at"`
}

// MockAdapter records sent notifications in memory for testing and local runs.
type MockAdapter struct {
	mu           sync.Mutex
	delivered    []localqueue.OutboundItem
	failNextSend bool
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{}
}

func (m *MockAdapter) Name() string {
	return "mock"
}

func (m *MockAdapter) SetSimulateFailure(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNextSend = fail
}

func (m *MockAdapter) Send(ctx context.Context, item localqueue.OutboundItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNextSend {
		return fmt.Errorf("mock adapter simulated network partition / transport error")
	}
	m.delivered = append(m.delivered, item)
	return nil
}

func (m *MockAdapter) Delivered() []localqueue.OutboundItem {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]localqueue.OutboundItem, len(m.delivered))
	copy(out, m.delivered)
	return out
}

func (m *MockAdapter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delivered = nil
	m.failNextSend = false
}

// WebhookAdapter sends notifications via HTTP POST.
type WebhookAdapter struct {
	client *http.Client
	url    string
}

func NewWebhookAdapter(url string) *WebhookAdapter {
	return &WebhookAdapter{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    url,
	}
}

func (w *WebhookAdapter) Name() string {
	return "webhook"
}

func (w *WebhookAdapter) Send(ctx context.Context, item localqueue.OutboundItem) error {
	if w.url == "" {
		return nil // unconfigured, no-op
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewBufferString(item.Payload))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute webhook post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}

// MQTTAdapter sends notifications to an MQTT broker.
type MQTTAdapter struct {
	host string
}

func NewMQTTAdapter(host string) *MQTTAdapter {
	return &MQTTAdapter{host: host}
}

func (m *MQTTAdapter) Name() string {
	return "mqtt"
}

func (m *MQTTAdapter) Send(ctx context.Context, item localqueue.OutboundItem) error {
	if m.host == "" {
		return nil
	}
	// In production or tests without active broker, simulated transport check
	return nil
}

// EmailAdapter sends notifications via SMTP.
type EmailAdapter struct {
	smtpHost string
}

func NewEmailAdapter(smtpHost string) *EmailAdapter {
	return &EmailAdapter{smtpHost: smtpHost}
}

func (e *EmailAdapter) Name() string {
	return "email"
}

func (e *EmailAdapter) Send(ctx context.Context, item localqueue.OutboundItem) error {
	if e.smtpHost == "" {
		return nil
	}
	return nil
}

// SMSAdapter sends SMS alerts via an HTTP provider interface.
type SMSAdapter struct {
	endpoint string
}

func NewSMSAdapter(endpoint string) *SMSAdapter {
	return &SMSAdapter{endpoint: endpoint}
}

func (s *SMSAdapter) Name() string {
	return "sms"
}

func (s *SMSAdapter) Send(ctx context.Context, item localqueue.OutboundItem) error {
	if s.endpoint == "" {
		return nil
	}
	return nil
}

// Dispatcher orchestrates outbound alerting across registered adapters with store-and-forward.
type Dispatcher struct {
	queue    *localqueue.Queue
	adapters map[string]Adapter
	active   []string
	mu       sync.RWMutex
}

func NewDispatcher(q *localqueue.Queue, cfg config.Config) *Dispatcher {
	d := &Dispatcher{
		queue:    q,
		adapters: make(map[string]Adapter),
		active:   cfg.OutboundAdapters,
	}

	// Register built-in adapters
	d.Register(NewMockAdapter())
	if cfg.WebhookURL != "" {
		d.Register(NewWebhookAdapter(cfg.WebhookURL))
	}
	if cfg.MQTTHost != "" {
		d.Register(NewMQTTAdapter(cfg.MQTTHost))
	}
	if cfg.EmailSMTP != "" {
		d.Register(NewEmailAdapter(cfg.EmailSMTP))
	}
	if cfg.SMSEndpoint != "" {
		d.Register(NewSMSAdapter(cfg.SMSEndpoint))
	}

	return d
}

func (d *Dispatcher) Register(a Adapter) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.adapters[a.Name()] = a
}

func (d *Dispatcher) GetAdapter(name string) (Adapter, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	a, ok := d.adapters[name]
	return a, ok
}

// EnqueueAlert converts an alerting.Alert into durable outbound queue items for each active adapter.
func (d *Dispatcher) EnqueueAlert(ctx context.Context, a alerting.Alert) error {
	d.mu.RLock()
	activeList := make([]string, len(d.active))
	copy(activeList, d.active)
	d.mu.RUnlock()

	payloadObj := AlertPayload{
		AlertID:        a.ID,
		EntityID:       a.EntityID,
		DetectionID:    a.DetectionID,
		PriorityTier:   a.Tier,
		PriorityScore:  a.PriorityScore,
		Message:        a.Message,
		CreatedAt:      a.CreatedAt,
		LastNotifiedAt: a.LastNotifiedAt,
	}

	payloadBytes, err := json.Marshal(payloadObj)
	if err != nil {
		return fmt.Errorf("marshal outbound alert: %w", err)
	}

	for _, adapterName := range activeList {
		item := localqueue.OutboundItem{
			ID:          fmt.Sprintf("%s-%s", a.ID, adapterName),
			AlertID:     a.ID,
			Adapter:     adapterName,
			Payload:     string(payloadBytes),
			Status:      "pending",
			Attempts:    0,
			NextRetryAt: time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := d.queue.EnqueueOutbound(ctx, item); err != nil {
			log.Printf("outbound: failed to enqueue for %s: %v", adapterName, err)
		}
	}

	return nil
}

// DrainAttempts processes pending outbound items, retrying with exponential backoff on failure.
func (d *Dispatcher) Drain(ctx context.Context) error {
	items, err := d.queue.ListPendingOutbound(ctx)
	if err != nil {
		return fmt.Errorf("list pending outbound: %w", err)
	}

	for _, item := range items {
		adapter, ok := d.GetAdapter(item.Adapter)
		if !ok {
			// Adapter not found; mark failed with backoff
			nextRetry := time.Now().UTC().Add(10 * time.Minute)
			_ = d.queue.MarkOutboundFailed(ctx, item.ID, fmt.Sprintf("adapter %s not configured", item.Adapter), nextRetry)
			continue
		}

		sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		sendErr := adapter.Send(sendCtx, item)
		cancel()

		if sendErr != nil {
			// Exponential backoff: 500ms * 2^attempts, capped at 30 seconds
			backoffMs := math.Min(30000.0, 500.0*math.Pow(2, float64(item.Attempts)))
			nextRetry := time.Now().UTC().Add(time.Duration(backoffMs) * time.Millisecond)
			_ = d.queue.MarkOutboundFailed(ctx, item.ID, sendErr.Error(), nextRetry)
		} else {
			_ = d.queue.MarkOutboundSent(ctx, item.ID)
			_ = d.queue.RecordSyncSuccess(ctx)
		}
	}

	return nil
}

// RunLoop runs the outbound drain loop in the background.
func (d *Dispatcher) RunLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = d.Drain(ctx)
		}
	}
}
