CREATE TYPE reservation_status AS ENUM (
    'RESERVED', -- room reserved temporarily, same status value as inventory
    'CONFIRMED', -- room reserved and paid
    'RELEASED', -- return to inventory (timeout, cancel, payment fails...)
    'EXPIRED'
);

CREATE TABLE reservations (
    reservation_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id         UUID NOT NULL REFERENCES rooms(id),
    check_in        DATE NOT NULL,
    check_out       DATE NOT NULL,          -- exclusive, i.e. last night is check_out - 1
    "status"        reservation_status NOT NULL DEFAULT 'RESERVED',
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sweep query: find expired reservations to release.
CREATE INDEX idx_reservations_active_expiry
    ON reservations (expires_at)
    WHERE "status" = 'RESERVED';

-- Lookup: does this room already have an active reservation overlapping a date range?
CREATE INDEX idx_reservations_room_status
    ON reservations (room_id, "status");
