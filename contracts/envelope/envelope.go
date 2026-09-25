// Package envelope defines the v1 Kafka message envelope shared by all services:
// messageId, type, sagaId, producer, occurredAt, plus the message-specific data.
//
// Definitions only: no Kafka or database code. Field meanings are in
// doc/uc1-create-booking/contracts.md (section 5).
package envelope
