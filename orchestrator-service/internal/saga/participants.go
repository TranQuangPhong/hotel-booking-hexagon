package saga

// Driven ports for the synchronous calls made while the user is waiting
// (phase A and B): RoomClient, BookingClient and PaymentClient. Each wraps one
// RPC (ReserveRoom, CreateBooking, CreateCheckoutSession); the port methods drop
// the entity like every core method (Reserve, Create, CreateCheckout).
// Implemented by adapter/grpcclient.
