package dto

import (
	"encoding/json"
	"fmt"
)

// WS message types — so a /ws/live client can tell a fused-detection push
// apart from an alert push without guessing at payload shape.
const (
	WSMessageDetection = "detection"
	WSMessageAlert     = "alert"
)

// WSMessage is the envelope every /ws/live payload is wrapped in.
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewWSMessage marshals payload and wraps it in a WSMessage envelope of
// the given type, ready to send over the WebSocket.
func NewWSMessage(msgType string, payload any) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", msgType, err)
	}

	envelope, err := json.Marshal(WSMessage{Type: msgType, Payload: payloadBytes})
	if err != nil {
		return nil, fmt.Errorf("marshal %s envelope: %w", msgType, err)
	}
	return envelope, nil
}
