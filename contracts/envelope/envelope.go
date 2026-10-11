// Package envelope defines the v1 Kafka message envelope shared by all services:
// message_id, type, saga_id, producer, occurred_at, plus the message-specific data.
//
// Definitions only: no Kafka or database code. Field meanings are in
// doc/uc1-create-booking/contracts.md (section 5).
package envelope

type Envelope[T any] struct {
	MessageID  string `json:"message_id"`
	Type       string `json:"type"`
	SagaID     string `json:"saga_id"`
	Producer   string `json:"producer"`
	OccurredAt string `json:"occurred_at"`
	Data       T      `json:"data"`
}
