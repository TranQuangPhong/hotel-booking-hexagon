// Package worker holds time-triggered driving adapters.
//
// The deadline worker runs every DEADLINE_POLL and calls service.ExpireOverdue,
// which moves sagas past deadline_at (RESERVING_ROOM, CREATING_BOOKING,
// AWAITING_PAYMENT) into compensation.
package worker
