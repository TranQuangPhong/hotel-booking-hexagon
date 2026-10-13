// Package paymentv1 holds payment-service's public contracts: the PaymentService
// gRPC API (CreateCheckoutSession, generated from payment.proto) and the Kafka
// messages on its topics.
//
//   - payment.commands (payment accepts): CapturePayment
//   - payment.events   (payment emits):   PaymentAuthorized, PaymentCaptured, PaymentCaptureFailed
package paymentv1
