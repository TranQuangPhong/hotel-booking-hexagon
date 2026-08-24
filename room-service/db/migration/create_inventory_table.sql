CREATE TABLE inventory (
    id BIGSERIAL PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES room(id),
    "year" SMALLINT NOT NULL,
    "month" SMALLINT NOT NULL,
    "days" JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (room_id, year, month),

    CHECK (month BETWEEN 1 AND 12),
    CHECK (year >= 2000)
);
