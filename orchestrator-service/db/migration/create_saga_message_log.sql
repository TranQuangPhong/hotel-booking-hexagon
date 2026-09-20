CREATE TYPE direction AS ENUM (
    'IN',
    'OUT'
);

CREATE TYPE message_type AS ENUM (
    'CMD_RESERVE_ROOM',
    'EVENT_RESERVE_ROOM_SUCCEEDED'
    -- TODO: add more here
);

CREATE TABLE saga_message_log (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    saga_id         UUID NOT NULL REFERENCES saga_instances (saga_id),
    direction       direction NOT NULL,
    "type"          message_type NOT NULL,
    payload         JSONB NOT NULL,    -- redact/omit sensitive fields before insert

    correlation_id  UUID NOT NULL,
    causation_id    UUID,              -- id of the message that triggered this one

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Reconstruct causal chain for a given saga, in order.
CREATE INDEX idx_saga_message_log_saga_id_created
    ON saga_message_log (saga_id, created_at);

-- Trace across a correlation_id independent of saga_id, if you use it broader than 1:1.
CREATE INDEX idx_saga_message_log_correlation_id
    ON saga_message_log (correlation_id);
