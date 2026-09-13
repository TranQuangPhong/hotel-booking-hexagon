package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepository struct {
	pool *pgxpool.Pool
}

func NewBookingRepository(ctx context.Context, pool *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{pool: pool}
}
