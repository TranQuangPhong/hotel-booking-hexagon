package postgres

import (
	"booking/room-service/internal/inventory"
	"booking/room-service/internal/room"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomRepository struct {
	pool *pgxpool.Pool
}

func NewRoomRepository(ctx context.Context, pool *pgxpool.Pool) *RoomRepository {
	return &RoomRepository{pool: pool}
}

func (r *RoomRepository) GetAll(ctx context.Context) ([]*room.Room, error) {
	sql := `select id, number, type, status, created_at, updated_at from rooms`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*room.Room
	for rows.Next() {
		var u room.Room
		if err := rows.Scan(&u.ID, &u.Number, &u.Type, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, &u)
	}
	return rooms, nil
}

func (r *RoomRepository) GetByID(ctx context.Context, id string) (*room.Room, error) {
	sql := `select id, number, type, status, created_at, updated_at from rooms where id = $1`
	var u room.Room
	if err := r.pool.QueryRow(ctx, sql, id).Scan(&u.ID, &u.Number, &u.Type, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *RoomRepository) Update(ctx context.Context, room *room.Room) (*room.Room, error) {
	sql := `update rooms set number = $1, type = $2, status = $3, updated_at = NOW()
	 where id = $4 RETURNING id, number, type, status, created_at, updated_at`
	if err := r.pool.QueryRow(ctx, sql, room.Number, room.Type, room.Status, room.ID).
		Scan(&room.ID, &room.Number, &room.Type, &room.Status, &room.CreatedAt, &room.UpdatedAt); err != nil {
		return nil, err
	}
	return room, nil
}

// Create room & inventory for next 1 year
func (r *RoomRepository) Create(ctx context.Context, room *room.Room, inventories []*inventory.Inventory) (*room.Room, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback in case of error

	// Create room
	sqlRoom := `insert into rooms (number, type, status) values ($1, $2, $3)
	 returning id, created_at, updated_at`
	if err = tx.QueryRow(ctx, sqlRoom, room.Number, room.Type, room.Status).
		Scan(&room.ID, &room.CreatedAt, &room.UpdatedAt); err != nil {
		return nil, fmt.Errorf("failed to insert room: %w", err)
	}
	// Create inventory
	sqlInventory :=
		`insert into inventories (room_id, year, month, days, currency) values ($1, $2, $3, $4, $5)
	 ON CONFLICT (room_id, year, month) DO NOTHING`
	batch := &pgx.Batch{}
	for _, inv := range inventories {
		inv.RoomID = room.ID // only known after the room insert
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

	return room, nil
}
