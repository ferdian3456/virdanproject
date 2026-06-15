package model

import "time"

// XenditWebhookEvent is a row in xendit_webhook_events. status: PENDING/PROCESSED/FAILED.
// Used for webhook idempotency (event_id unique) and audit of received callbacks.
type XenditWebhookEvent struct {
	Id          string
	EventId     string
	EventType   string
	ReferenceId string
	Payload     []byte
	Status      string
	ReceivedAt  time.Time
	ProcessedAt *time.Time
}
