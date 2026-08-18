package postgres

import (
	"booking/user-service/internal/user"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(ctx context.Context, pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) GetUsers(ctx context.Context) ([]*user.User, error) {
	sql := `select id, name, email, role from users`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		var u user.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	sql := `select id, name, email, role from users where id = $1`
	var u user.User
	if err := r.pool.QueryRow(ctx, sql, id).Scan(&u.ID, &u.Name, &u.Email, &u.Role); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, user *user.User) (*user.User, error) {
	sql := `insert into users (name, email, role) values ($1, $2, $3) returning id`
	if err := r.pool.QueryRow(ctx, sql, user.Name, user.Email, user.Role).Scan(&user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, user *user.User) (*user.User, error) {
	sql := `update users set name = $1, email = $2, role = $3 where id = $4`
	if _, err := r.pool.Exec(ctx, sql, &user.Name, &user.Email, &user.Role, &user.ID); err != nil {
		return nil, err
	}
	return user, nil
}
