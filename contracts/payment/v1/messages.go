// Package paymentv1 holds payment-service's public contracts: the PaymentService
// gRPC API (CreatePaymentIntent, generated from payment.proto) and the Kafka
// messages on its topics.
//
//   - payment.commands (payment accepts): CapturePayment
//   - payment.events   (payment emits):   PaymentAuthorized, PaymentCaptured, PaymentCaptureFailed
//
// This file holds the Kafka side: type names, topic names and data structs.
package paymentv1
