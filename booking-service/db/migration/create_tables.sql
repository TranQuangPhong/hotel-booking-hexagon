CREATE TYPE booking_status AS ENUM ('PENDING', 'RESERVED', 'RESERVATION_FAILED', 'PAYMENT_FAILED', 'BOOKED', 'CANCELLED', 'CHECKED_IN', 'CHECKED_OUT', 'NO_SHOW');
CREATE TYPE payment_status AS ENUM ('PENDING', 'COMPLETED', 'FAILED', 'REFUNDED', 'PARTIALLY_REFUNDED');

CREATE TABLE bookings (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id           TEXT NOT NULL,
    user_name         TEXT NOT NULL,
    user_email        TEXT NOT NULL,
    user_phone_number TEXT NOT NULL,

    room_id           TEXT NOT NULL,
    room_number       TEXT NOT NULL,
    room_type         TEXT NOT NULL,

    check_in_date     DATE NOT NULL,
    check_out_date    DATE NOT NULL,
    number_of_guests  INT  NOT NULL,

    total_amount      BIGINT NOT NULL DEFAULT 0,  -- minor units
    currency          CHAR(3) NOT NULL,

    status            booking_status NOT NULL,
    payment_status    payment_status NOT NULL,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (check_out_date > check_in_date)
);

CREATE INDEX idx_bookings_user_id ON bookings(user_id);
CREATE INDEX idx_bookings_room_id ON bookings(room_id);
CREATE INDEX idx_bookings_dates   ON bookings(check_in_date, check_out_date);

CREATE TABLE booking_nightly_rates (
    id         BIGSERIAL PRIMARY KEY,
    booking_id UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    date       DATE NOT NULL,
    price      BIGINT NOT NULL,   -- minor units, same currency as bookings.currency

    UNIQUE (booking_id, date)
);

CREATE INDEX idx_nightly_rates_booking_id ON booking_nightly_rates(booking_id);