package saga

// Driven port for saga persistence: Create, GetForUpdate, Save(saga, commands).
// Save writes the new saga state and its outgoing commands (saga_message_log,
// the outbox) in one transaction. Implemented by adapter/postgres.
