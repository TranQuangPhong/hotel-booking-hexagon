-- saga_instances: one row per saga. Steps, statuses and failure reasons: doc/uc1-create-booking/state-machines.md
-- Rebuild: drop_all.sql, then this file (no ALTER in phase 1). gen_random_uuid() is built in since Postgres 13

CREATE TYPE saga_type AS ENUM ('CREATE_BOOKING'); -- UC2 / UC3 add theirs with ALTER TYPE ... ADD VALUE

CREATE TYPE saga_status AS ENUM ('IN_PROGRESS', 'COMPENSATING', 'COMPLETED', 'FAILED');

-- COMPLETED and FAILED are statuses, never steps: a saga that ends keeps its last step
CREATE TYPE saga_step AS ENUM (
    'RESERVING_ROOM',
    'CREATING_BOOKING',
    'AWAITING_PAYMENT',
    'CONFIRMING_ROOM',
    'CAPTURING_PAYMENT',
    'CONFIRMING_BOOKING',
    'RELEASING_ROOM',
    'EXPIRING_BOOKING'
);

-- Why a saga left the main line: set by the Save of that transition, never cleared
CREATE TYPE saga_failure_reason AS ENUM ('ROOM_REJECTED', 'PHASE_A_ERROR', 'DEADLINE', 'CAPTURE_FAILED');

CREATE TABLE saga_instances (
    saga_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type           saga_type NOT NULL,
    status         saga_status NOT NULL,
    current_step   saga_step NOT NULL,
    user_id        UUID NOT NULL,                      -- owner check on POST .../payment
    -- NULL until the step that creates them succeeds; the repository maps NULL <-> "" (COALESCE / NULLIF)
    reservation_id UUID,
    booking_id     UUID UNIQUE,                        -- one saga per booking (M5 GetByBookingID); NULLs never collide
    payment_id     UUID,
    context        JSONB NOT NULL,                     -- everything the next call needs: user, room, dates, rates, total
    failure_reason saga_failure_reason,                -- NULL while on the main line
    deadline_at    TIMESTAMPTZ NOT NULL,               -- now + HOLD_TTL at start, sent as the hold's expires_at
    version        INT NOT NULL DEFAULT 1,             -- every write: UPDATE ... WHERE version = <as read>, then + 1
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(), -- kept by the trigger below, not by the repository
    -- a reason exactly when the saga left the main line: COMPENSATING and FAILED have one, IN_PROGRESS and COMPLETED don't
    CONSTRAINT check_failure_reason CHECK ((failure_reason IS NULL) = (status IN ('IN_PROGRESS', 'COMPLETED')))
);

-- Deadline worker (M4a GetOverdue): the steps whose hold can still expire; from CONFIRMING_ROOM on the user has paid.
-- GetOverdue repeats this WHERE with literals, not $n: a generic plan can't prove $n matches it, so it skips the index
CREATE INDEX idx_saga_deadline ON saga_instances (deadline_at)
    WHERE status = 'IN_PROGRESS'
      AND current_step IN ('RESERVING_ROOM', 'CREATING_BOOKING', 'AWAITING_PAYMENT');

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
