package saga

// Orchestrator use cases, called by the driving adapters:
//   - StartBooking:  POST /bookings, phase A (ReserveRoom -> CreateBooking)
//   - StartPayment:  POST /bookings/{id}/payment, calls CreatePaymentIntent
//   - OnEvent:       Kafka consumer, applies a participant event via transitions.go
//   - ExpireOverdue: deadline worker, starts compensation for overdue sagas
