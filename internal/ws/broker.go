package ws

import (
	"context"

	"github.com/bytedance/sonic"
)

// Event is the wire envelope sent to clients: {"type": "...", "payload": {...}}.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Broker fans an event out to target users. v1 = in-process (single node).
// Multi-node later = swap RedisBroker; usecase/hub unchanged.
type Broker interface {
	Publish(ctx context.Context, targetUserIds []string, ev Event) error
}

type InProcessBroker struct {
	hub *Hub
}

func NewInProcessBroker(hub *Hub) *InProcessBroker {
	return &InProcessBroker{hub: hub}
}

func (broker *InProcessBroker) Publish(_ context.Context, targetUserIds []string, ev Event) error {
	payload, err := sonic.Marshal(ev)
	if err != nil {
		return err
	}
	for _, uid := range targetUserIds {
		broker.hub.deliver(uid, payload)
	}
	return nil
}
