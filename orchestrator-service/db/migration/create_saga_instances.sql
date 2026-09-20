-- Requires pgcrypto or pgcrypto-compatible UUID generation.
-- If using UUIDv7 in application code, generate there and just store as UUID here.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE saga_type AS ENUM (
    'CREATE_BOOKING',
    'CANCEL_BOOKING',
    'REFUND'
);

CREATE TYPE saga_status AS ENUM (
    'IN_PROGRESS',
    'COMPLETED',
    'FAILED',
    'COMPENSATING'
);

CREATE TABLE saga_instances (
    saga_id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "type"              saga_type NOT NULL,
    "status"            saga_status NOT NULL DEFAULT 'IN_PROGRESS',
    current_step        TEXT NOT NULL,

    -- Cross-references to entities this saga touches. Nullable because they
    -- don't exist yet at saga start (e.g. booking_id is null until step 2).
    booking_id          UUID,
    reservation_id      TEXT,
    payment_intent_id   TEXT,

    -- Minimal workflow context needed to build the next outgoing command.
    -- Not a system of record for any entity — just enough to resume/replay.
    context             JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Optimistic concurrency control: every update must check this and increment it.
    version             INT NOT NULL DEFAULT 1,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Recovery sweep: "find sagas stuck mid-flight."
CREATE INDEX idx_saga_instances_status_updated
    ON saga_instances ("status", updated_at)
    WHERE "status" IN ('IN_PROGRESS', 'COMPENSATING');

-- Lookup by the entity a saga is about (e.g. "any active saga on this booking?").
CREATE INDEX idx_saga_instances_booking_id
    ON saga_instances (booking_id)
    WHERE booking_id IS NOT NULL;

-- Fast path for querying/aggregating by workflow type + status.
CREATE INDEX idx_saga_instances_type_status
    ON saga_instances ("type", "status");

-- Optional: index into specific context fields without unpacking the whole blob,
-- e.g. context->>'room_type'. Add only the expression indexes you actually query on.
-- CREATE INDEX idx_saga_instances_context_room_type
--     ON saga_instances ((context->>'room_type'));

-- Keep updated_at accurate on every UPDATE to saga_instances.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_saga_instances_updated_at
    BEFORE UPDATE ON saga_instances
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
