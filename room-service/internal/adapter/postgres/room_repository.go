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

func (r *RoomRepository) GetRooms(ctx context.Context) ([]*room.Room, error) {
	sql := `select id, number, type, status from rooms`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*room.Room
	for rows.Next() {
		var u room.Room
		if err := rows.Scan(&u.ID, &u.Number, &u.Type, &u.Status); err != nil {
			return nil, err
		}
		rooms = append(rooms, &u)
	}
	return rooms, nil
}

func (r *RoomRepository) GetRoomByID(ctx context.Context, id string) (*room.Room, error) {
	sql := `select id, number, type, status from rooms where id = $1`
	var u room.Room
	if err := r.pool.QueryRow(ctx, sql, id).Scan(&u.ID, &u.Number, &u.Type, &u.Status); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *RoomRepository) CreateRoom(ctx context.Context, room *room.Room) (*room.Room, error) {
	sql := `insert into rooms (number, type, status) values ($1, $2, $3) returning id`
	if err := r.pool.QueryRow(ctx, sql, room.Number, room.Type, room.Status).Scan(&room.ID); err != nil {
		return nil, err
	}
	return room, nil
}

func (r *RoomRepository) UpdateRoom(ctx context.Context, room *room.Room) (*room.Room, error) {
	sql := `update rooms set number = $1, type = $2, status = $3 where id = $4`
	if _, err := r.pool.Exec(ctx, sql, &room.Number, &room.Type, &room.Status, &room.ID); err != nil {
		return nil, err
	}
	return room, nil
}
