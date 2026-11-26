package postgres

import (
	"booking/orchestrator-service/internal/saga"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SagaRepository struct {
	pool *pgxpool.Pool
}

func NewSagaRepository(p *pgxpool.Pool) *SagaRepository {
	return &SagaRepository{pool: p}
}

var _ saga.Repository = (*SagaRepository)(nil)

// sagaColumns: the column list of every RETURNING / SELECT, one per saga.Saga field (RowToStructByName needs an exact
// match). booking_id and failure_reason are NULL in the table and "" in Go: COALESCE here, NULLIF in the writes.
// ::text first: without it COALESCE reads the empty string as a saga_failure_reason value and fails
const sagaColumns = `saga_id, type, status, current_step,
	coalesce(booking_id::text, '') as booking_id, context,
	coalesce(failure_reason::text, '') as failure_reason,
	deadline_at, version, created_at, updated_at`

func (r *SagaRepository) Create(ctx context.Context, s *saga.Saga) (*saga.Saga, error) {
	// booking_id and failure_reason start NULL; saga_id, version, created_at and updated_at come from the defaults
	const sql = `insert into saga_instances (type, status, current_step, context, deadline_at)
	values ($1, $2, $3, $4, $5)
	returning ` + sagaColumns

	// One message for both checks: a server error (constraint, enum value) arrives while the rows are read, so in
	// CollectOneRow, not in Query
	rows, err := r.pool.Query(ctx, sql, s.Type, s.Status, s.Step, s.Context, s.DeadlineAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create saga instance: %w", err)
	}
	sg, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[saga.Saga])
	if err != nil {
		return nil, fmt.Errorf("failed to create saga instance: %w", err)
	}

	return sg, nil
}

func (r *SagaRepository) Save(ctx context.Context, s *saga.Saga) (*saga.Saga, error) {
	// type and deadline_at are written once by Create; updated_at by the trigger. NULLIF: "" -> NULL ($3 and $5 are
	// inferred as text, so pgx sends the Go string and the cast happens in SQL)
	const sql = `update saga_instances
	set status = $1, current_step = $2, booking_id = nullif($3, '')::uuid,
	context = $4, failure_reason = nullif($5, '')::saga_failure_reason, version = version + 1
	where saga_id = $6 and version = $7
	returning ` + sagaColumns

	rows, err := r.pool.Query(ctx, sql, s.Status, s.Step, s.BookingID, s.Context, s.FailureReason, s.ID, s.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to update saga instance: %w", err)
	}
	sg, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[saga.Saga])
	if err != nil {
		// 0 rows: the version moved (another writer saved first), nothing was written
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, saga.ErrStale
		}
		return nil, fmt.Errorf("failed to update saga instance: %w", err)
	}

	return sg, nil
}
