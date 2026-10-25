CREATE TYPE booking_status AS ENUM ('PENDING', 'BOOKED', 'EXPIRED', 'CANCELLED', 'CHECKED_IN', 'CHECKED_OUT', 'NO_SHOW');
CREATE TYPE payment_status AS ENUM ('PENDING', 'COMPLETED', 'FAILED', 'REFUNDED', 'PARTIALLY_REFUNDED');

CREATE TABLE bookings (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id           UUID NOT NULL UNIQUE,  -- idempotent CreateBooking + lookup for commands
    reservation_id    UUID NOT NULL,         -- room-service hold, reference only

    user_id           UUID NOT NULL,
    user_name         TEXT NOT NULL,
    user_email        TEXT NOT NULL,
    user_phone_number TEXT NOT NULL,

    room_id           UUID NOT NULL,
    room_number       TEXT NOT NULL,
    room_type         TEXT NOT NULL,

    check_in_date     DATE NOT NULL,
    check_out_date    DATE NOT NULL,
    number_of_guests  INT  NOT NULL,

    total_amount      BIGINT NOT NULL,       -- minor units
    currency          CHAR(3) NOT NULL,
    nightly_rates     JSONB NOT NULL,        -- snapshot, [{"date":"2026-10-25","price":12000}]: date YYYY-MM-DD, price minor units

    status            booking_status NOT NULL,
    payment_status    payment_status NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,  -- hold deadline: UI countdown while PENDING

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (check_out_date > check_in_date)
);

CREATE INDEX idx_bookings_user_id ON bookings(user_id);
CREATE INDEX idx_bookings_room_id ON bookings(room_id);
CREATE INDEX idx_bookings_dates   ON bookings(check_in_date, check_out_date);