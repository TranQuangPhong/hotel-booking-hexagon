// Package paymentv1 holds payment-service's public contracts: the PaymentService
// gRPC API (CreateCheckoutSession, generated from payment.proto) and the Kafka
// messages on its topics.
//
//   - payment.commands (payment accepts): CapturePayment
//   - payment.events   (payment emits):   PaymentAuthorized, PaymentCaptured, PaymentCaptureFailed
//
// This file holds the Kafka side: type names, topic names and data structs.
package paymentv1

// topics
const (
	TopicCommands = "payment.commands"
	TopicEvents   = "payment.events"
)

// message types
const (
	//event
	TypePaymentAuthorized    = "PaymentAuthorized"
	TypePaymentCaptured      = "PaymentCaptured"
	TypePaymentCaptureFailed = "PaymentCaptureFailed"
	//cmd
	TypeCapturePayment = "CapturePayment"
)

// message structures
type PaymentAuthorized struct {
	PaymentID string `json:"payment_id"`
	BookingID string `json:"booking_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

type PaymentCaptured struct {
	PaymentID string `json:"payment_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

type PaymentCaptureFailed struct {
	PaymentID string `json:"payment_id"`
	Reason    string `json:"reason"`
}

// cmd payment svc to capture payment.
// No data: the payment is found by the envelope's saga_id
type CapturePayment struct{}
