-- Drops every orchestrator-service object, so the create_*.sql files can be applied from scratch.
-- Order matters: saga_message_log references saga_instances, enum types go after the tables that use them.
-- Re-create: create_saga_instances.sql only (saga_message_log is not v1; its file goes away in M4a)

DROP TABLE IF EXISTS saga_message_log;
DROP TABLE IF EXISTS saga_instances; -- drops its trigger too

DROP FUNCTION IF EXISTS set_updated_at();

DROP TYPE IF EXISTS saga_failure_reason;
DROP TYPE IF EXISTS saga_step;
DROP TYPE IF EXISTS saga_status;
DROP TYPE IF EXISTS saga_type;
DROP TYPE IF EXISTS direction;    -- saga_message_log's
DROP TYPE IF EXISTS message_type; -- saga_message_log's
