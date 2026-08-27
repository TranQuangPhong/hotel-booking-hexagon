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
