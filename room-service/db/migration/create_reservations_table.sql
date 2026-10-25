CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE reservation_status AS ENUM (
    'RESERVED',
    'CONFIRMED', -- payment authorized (capture comes next)
    'RELEASED'
);

CREATE TABLE reservations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id         UUID NOT NULL UNIQUE,
    room_id         UUID NOT NULL REFERENCES rooms(id),
    check_in_date   DATE NOT NULL,
    check_out_date  DATE NOT NULL,          -- exclusive, i.e. last night is check_out_date - 1
    "status"        reservation_status NOT NULL DEFAULT 'RESERVED',
    total_amount    BIGINT NOT NULL,  -- snapshot, minor units
    currency        CHAR(3) NOT NULL, -- snapshot
    nightly_rates   JSONB NOT NULL, -- snapshot, [{"date":"2026-10-25","price":12000}]: date YYYY-MM-DD, price minor units
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reservations_no_overlap
        EXCLUDE USING gist (
            room_id WITH =,
            daterange(check_in_date, check_out_date, '[)') WITH &&
        )
        WHERE ("status" IN ('RESERVED', 'CONFIRMED')), --an overlap on the same room fails with SQLSTATE 23P01
    CONSTRAINT check_dates CHECK (check_in_date < check_out_date)
);
