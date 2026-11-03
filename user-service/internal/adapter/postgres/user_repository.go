package postgres

import (
	"booking/user-service/internal/user"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

// compile-time check: a mismatch with the port fails here, not at the NewService call in main.go
var _ user.Repository = (*UserRepository)(nil)

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) GetAll(ctx context.Context) ([]*user.User, error) {
	sql := `select id, name, email, role, created_at, updated_at from users`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[user.User])
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	sql := `select id, name, email, role, created_at, updated_at from users where id = $1`
	rows, err := r.pool.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[user.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, user.ErrNotFound
	}
	return u, err
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) (*user.User, error) {
	sql := `insert into users (name, email, role) values ($1, $2, $3) returning id, created_at, updated_at`
	if err := r.pool.QueryRow(ctx, sql, u.Name, u.Email, u.Role).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("user email %q: %w", u.Email, user.ErrEmailTaken)
		}
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) Update(ctx context.Context, u *user.User) (*user.User, error) {
	sql := `update users set name = $1, email = $2, role = $3, updated_at = NOW()
	 where id = $4 returning created_at, updated_at`
	if err := r.pool.QueryRow(ctx, sql, u.Name, u.Email, u.Role, u.ID).Scan(&u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("user email %q: %w", u.Email, user.ErrEmailTaken)
		}
		return nil, err
	}
	return u, nil
}

// isUniqueViolation reports a Postgres unique_violation (23505). On users the only one a caller can hit is email
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
