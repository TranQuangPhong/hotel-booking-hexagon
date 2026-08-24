CREATE TYPE room_type AS ENUM (
    'STANDARD',
    'DELUXE',
    'SUITE'
);

CREATE TYPE room_status AS ENUM (
    'ACTIVE',
    'INACTIVE',
    'ARCHIVED'
);

CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "number" TEXT NOT NULL UNIQUE,
    "type" room_type NOT NULL,
    "status" room_status NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
