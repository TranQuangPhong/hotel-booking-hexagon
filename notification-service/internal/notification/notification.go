// Package notification is the core of notification-service: what to tell whom
// when a booking is confirmed. It depends only on its Sender port.
//
// This file holds BookingConfirmedNotice, built from the BookingConfirmed event
// (a fat event, so no lookups to other services are needed).
package notification
