package dto

import (
	"encoding/json"
	"fmt"
)

// WSVersion is the current version tag on all live WebSocket broadcast messages.
const WSVersion = "v2"

// WS message types — so a /ws/live client can tell a detection push
// apart from an entity update, alert push, or telemetry update.
const (
	WSMessageDetection    = "detection"
	WSMessageEntityUpdate = "entity_update"
	WSMessageAlert        = "alert"
	WSMessageTelemetry    = "telemetry"
)

// WSMessage is the versioned envelope every /ws/live payload is wrapped in.
type WSMessage struct {
	Version string          `json:"version"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewWSMessage marshals payload and wraps it in a versioned WSMessage envelope.
func NewWSMessage(msgType string, payload any) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", msgType, err)
	}

	envelope, err := json.Marshal(WSMessage{
		Version: WSVersion,
		Type:    msgType,
		Payload: payloadBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal %s envelope: %w", msgType, err)
	}
	return envelope, nil
}
