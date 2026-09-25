// Package kafka is the driving adapter for payment-service's Kafka commands.
//
// The consumer reads payment.commands (CapturePayment), finds the payment by
// the envelope's sagaId and calls the payment service's Capture.
package kafka
