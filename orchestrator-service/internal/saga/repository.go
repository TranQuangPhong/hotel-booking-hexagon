package saga

// Driven port for saga persistence: Create, Save(saga). Save is a conditional
// update on version (ErrStale when another writer saved first). Implemented by
// adapter/postgres.
