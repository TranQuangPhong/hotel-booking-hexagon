package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryRepository struct {
	pool *pgxpool.Pool
}

func NewInventoryRepository(ctx context.Context, pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

// func (r *InventoryRepository) Create(ctx context.Context, inventories []*inventory.Inventory) error {
// 	sql := `insert into inventories (room_id, year, month, days, currency) values ($1, $2, $3, $4, $5) ON CONFLICT (room_id, year, month) DO NOTHING`

// 	batch := &pgx.Batch{}
// 	for _, inventory := range inventories {
// 		batch.Queue(sql, inventory.RoomID, inventory.Year, inventory.Month, inventory.Days, inventory.Currency)
// 	}

// 	if err := r.pool.SendBatch(ctx, batch); err != nil {
// 		return fmt.Errorf("failed to insert inventory: %w", err)
// 	}

// 	return nil
// }
