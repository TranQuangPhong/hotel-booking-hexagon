// Package handler is payment-service's REST adapter (Gin).
//
// POST /payments/api/v1/webhooks/stripe: reads the raw body, verifies the
// Stripe-Signature header with STRIPE_WEBHOOK_SECRET (400 on a bad signature),
// then calls the payment service's MarkAuthorized.
package handler
