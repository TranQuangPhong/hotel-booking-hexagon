package saga

// Driven ports for the synchronous calls made while the user is waiting
// (phase A): RoomClient and BookingClient. Each wraps one RPC (ReserveRoom,
// CreateBooking); the port methods drop the entity like every core method
// (Reserve, Create). Implemented by adapter/grpcclient.
