package saga

import "context"

// Driven port for saga persistence: Create, Save(saga). Save is a conditional
// update on version (ErrStale when another writer saved first). Implemented by
// adapter/postgres.

// Repository stores sagas in saga_instances. It grows with its callers: GetByID + GetOverdue in M4a, GetByBookingID
// in M5 (signatures: plan.md, Notes, saga core)
type Repository interface {
	// Create inserts s and returns the stored row (saga_id, version 1, created_at come from the DB); s is not modified
	Create(ctx context.Context, s *Saga) (*Saga, error)
	// Save writes s only if the stored version is still s.Version. Returns the stored row (version + 1); s is not
	// modified. ErrStale: another writer saved first, nothing written. M4a adds commands []Command (outbox rows, same tx)
	Save(ctx context.Context, s *Saga) (*Saga, error)
}
