package saga

// Driven ports for the synchronous calls made while the user is waiting
// (phase A and B): RoomClient (ReserveRoom), BookingClient (CreateBooking)
// and PaymentClient (CreatePaymentIntent). Implemented by adapter/grpcclient.
