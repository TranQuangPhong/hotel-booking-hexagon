package postgres

import (
	"booking/room-service/internal/room"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomRepository struct {
	pool *pgxpool.Pool
}

// compile-time check: a mismatch with the port fails here, not at the NewService call in main.go
var _ room.Repository = (*RoomRepository)(nil)

func NewRoomRepository(pool *pgxpool.Pool) *RoomRepository {
	return &RoomRepository{pool: pool}
}

func (r *RoomRepository) GetAll(ctx context.Context) ([]*room.Room, error) {
	sql := `select id, number, type, status, created_at, updated_at from rooms`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[room.Room])
}

func (r *RoomRepository) GetByID(ctx context.Context, id string) (*room.Room, error) {
	sql := `select id, number, type, status, created_at, updated_at from rooms where id = $1`
	rows, err := r.pool.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	rm, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[room.Room])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, room.ErrNotFound
	}
	return rm, err
}

func (r *RoomRepository) Update(ctx context.Context, rm *room.Room) (*room.Room, error) {
	sql := `update rooms set number = $1, type = $2, status = $3, updated_at = NOW()
	 where id = $4 RETURNING id, number, type, status, created_at, updated_at`
	if err := r.pool.QueryRow(ctx, sql, rm.Number, rm.Type, rm.Status, rm.ID).
		Scan(&rm.ID, &rm.Number, &rm.Type, &rm.Status, &rm.CreatedAt, &rm.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, room.ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("room number %q: %w", rm.Number, room.ErrNumberTaken)
		}
		return nil, err
	}
	return rm, nil
}

// isUniqueViolation reports a Postgres unique_violation (23505). On rooms the only one a caller can hit is number
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Create room & inventory for next 1 year
func (r *RoomRepository) Create(ctx context.Context, rm *room.Room, inventories []*room.Inventory) (*room.Room, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback in case of error

	// Create room
	sqlRoom := `insert into rooms (number, type, status) values ($1, $2, $3)
	 returning id, created_at, updated_at`
	if err = tx.QueryRow(ctx, sqlRoom, rm.Number, rm.Type, rm.Status).
		Scan(&rm.ID, &rm.CreatedAt, &rm.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("room number %q: %w", rm.Number, room.ErrNumberTaken)
		}
		return nil, fmt.Errorf("failed to insert room: %w", err)
	}
	// Create inventory
	sqlInventory :=
		`insert into inventories (room_id, year, month, days, currency)
		 values ($1, $2, $3, $4, $5)
	 	 ON CONFLICT (room_id, year, month) DO NOTHING`
	batch := &pgx.Batch{}
	for _, inv := range inventories {
		inv.RoomID = rm.ID // only known after the room insert
		batch.Queue(sqlInventory, inv.RoomID, inv.Year, inv.Month, inv.Days, inv.Currency)
	}
	results := tx.SendBatch(ctx, batch)
	// Important: defer for error path,
	// but for txn to commit, still need to explicitly close batch
	defer results.Close()

	for _, inv := range inventories {
		if _, err = results.Exec(); err != nil {
			return nil, fmt.Errorf("failed to insert inventory %d-%02d: %w", inv.Year, inv.Month, err)
		}
	}
	// Important: explicitly close batch for txn to commit
	// Because above "defer results.Close()" runs after txn commits,
	// means conn still in use & fails the commit
	if err = results.Close(); err != nil {
		return nil, fmt.Errorf("failed to close batch results: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return rm, nil
}

// the rows of every month from firstNight to lastNight; a missing month is just absent, not an error
func (r *RoomRepository) GetInventories(ctx context.Context, roomID string, firstNight, lastNight time.Time) ([]*room.Inventory, error) {
	sql := `select id, room_id, year, month, days, currency, created_at, updated_at from inventories
	 where room_id = $1
	 and (year, month) >= ($2, $3)
	 and (year, month) <= ($4, $5)
	 order by year, month`
	rows, err := r.pool.Query(ctx, sql, roomID,
		firstNight.Year(), int(firstNight.Month()),
		lastNight.Year(), int(lastNight.Month()))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[room.Inventory])
}

func (r *RoomRepository) SaveInventories(ctx context.Context, inventories []*room.Inventory) error {
	sql := `update inventories set days = $1, updated_at = now()
	 where id = $2 returning updated_at`

	batch := &pgx.Batch{}
	for _, inv := range inventories {
		batch.Queue(sql, inv.Days, inv.ID)
	}

	results := r.pool.SendBatch(ctx, batch) //implicit txn
	defer results.Close()

	for _, inv := range inventories {
		if err := results.QueryRow().Scan(&inv.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// nothing deletes inventories, so fail loudly. The other rows still commit: an UPDATE of 0 rows
				// is no database error, so the batch's implicit transaction doesn't roll back
				return fmt.Errorf("failed to save inventory %d-%02d: id %d no longer exists", inv.Year, inv.Month, inv.ID)
			}
			return fmt.Errorf("failed to save inventory %d-%02d: %w", inv.Year, inv.Month, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("failed to close inventory batch: %w", err)
	}
	return nil
}

func (r *RoomRepository) HasReservation(ctx context.Context, roomID string, firstNight, lastNight time.Time) (bool, error) {
	var exist bool
	sql := `select exists (
	select 1 from reservations
	where room_id = $1
	and status in ('RESERVED', 'CONFIRMED')
	and daterange(check_in_date, check_out_date, '[)') && daterange($2, $3, '[]')
	)`

	err := r.pool.QueryRow(ctx, sql, roomID, firstNight, lastNight).Scan(&exist)
	if err != nil {
		return false, fmt.Errorf("failed to check reservation exists: %w", err)
	}
	return exist, nil
}

// nil, nil when the saga has no reservation yet: the normal first call, not an error
func (r *RoomRepository) GetReservationBySagaID(ctx context.Context, sagaID string) (*room.Reservation, error) {
	sql := `select id, saga_id, room_id, check_in_date, check_out_date,
	 status, total_amount, currency, nightly_rates, expires_at, created_at, updated_at
	 from reservations where saga_id = $1`
	rows, err := r.pool.Query(ctx, sql, sagaID)
	if err != nil {
		return nil, err
	}
	rs, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[room.Reservation])
	if errors.Is(err, pgx.ErrNoRows) { //It's fine to have no reservation
		return nil, nil
	}
	return rs, err
}

// returns the stored row; ErrRoomUnavailable on an overlap
func (r *RoomRepository) CreateReservation(ctx context.Context, rs *room.Reservation) (*room.Reservation, error) {
	// on conflict with no target: any unique or exclusion conflict (saga_id, reservations_no_overlap) skips the row,
	// after Postgres waits for the other insert to commit. Two inserts at the same moment don't deadlock this way
	sql := `insert into reservations
	 (saga_id, room_id, check_in_date, check_out_date,
	 status, total_amount, currency, nightly_rates, expires_at)
	 values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	 on conflict do nothing
	 returning id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, sql, rs.SagaID, rs.RoomID, rs.CheckInDate, rs.CheckOutDate,
		rs.Status, rs.TotalAmount, rs.Currency, rs.NightlyRates, rs.ExpiresAt).
		Scan(&rs.ID, &rs.CreatedAt, &rs.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// skipped: the row in the way is our own (an identical call committed first) or another saga's overlap
		own, getErr := r.GetReservationBySagaID(ctx, rs.SagaID)
		if getErr != nil {
			return nil, getErr
		}
		if own == nil {
			return nil, fmt.Errorf("dates overlap another reservation: %w", room.ErrRoomUnavailable)
		}
		return own, nil
	}
	if err != nil {
		return nil, err
	}
	return rs, nil
}
