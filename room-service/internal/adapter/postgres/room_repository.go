package postgres

import (
	"booking/room-service/internal/room"
	"context"

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

func (r *RoomRepository) Create(ctx context.Context, room *room.Room) (*room.Room, error) {
	sql := `insert into rooms (number, type, status) values ($1, $2, $3) returning id, created_at, updated_at`
	if err := r.pool.QueryRow(ctx, sql, room.Number, room.Type, room.Status).Scan(&room.ID, &room.CreatedAt, &room.UpdatedAt); err != nil {
		return nil, err
	}
	return room, nil
}

func (r *RoomRepository) Update(ctx context.Context, room *room.Room) (*room.Room, error) {
	sql := `update rooms set number = $1, type = $2, status = $3, updated_at = NOW() where id = $4 RETURNING id, number, type, status, created_at, updated_at`
	if err := r.pool.QueryRow(ctx, sql, room.Number, room.Type, room.Status, room.ID).
		Scan(&room.ID, &room.Number, &room.Type, &room.Status, &room.CreatedAt, &room.UpdatedAt); err != nil {
		return nil, err
	}
	return room, nil
}
