package postgres

import (
	"booking/booking-service/internal/booking"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepository struct {
	pool *pgxpool.Pool
}

func NewBookingRepository(ctx context.Context, pool *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{pool: pool}
}

func (r *BookingRepository) GetByID(ctx context.Context, id string) (*booking.Booking, error) {
	sql := `select id, saga_id, reservation_id,
	 user_id, user_name, user_email, user_phone_number,
	 room_id, room_number, room_type,
	 check_in_date, check_out_date,
	 number_of_guests, total_amount, currency, nightly_rates,
	 status, payment_status, expires_at,
	 created_at, updated_at
	 from bookings
	 where id = $1`

	rows, err := r.pool.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	b, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[booking.Booking])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, booking.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get booking by ID: %w", err)
	}

	return b, nil
}

func (r *BookingRepository) GetByUserID(ctx context.Context, userID string) ([]*booking.Booking, error) {
	sql := `select id, saga_id, reservation_id,
	 user_id, user_name, user_email, user_phone_number,
	 room_id, room_number, room_type,
	 check_in_date, check_out_date,
	 number_of_guests, total_amount, currency, nightly_rates,
	 status, payment_status, expires_at,
	 created_at, updated_at
	 from bookings
	 where user_id = $1`

	rows, err := r.pool.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[booking.Booking])
}

func (r *BookingRepository) GetBySagaID(ctx context.Context, sagaID string) (*booking.Booking, error) {
	sql := `select id, saga_id, reservation_id,
	 user_id, user_name, user_email, user_phone_number,
	 room_id, room_number, room_type,
	 check_in_date, check_out_date,
	 number_of_guests, total_amount, currency, nightly_rates,
	 status, payment_status, expires_at,
	 created_at, updated_at
	 from bookings
	 where saga_id = $1`

	rows, err := r.pool.Query(ctx, sql, sagaID)
	if err != nil {
		return nil, err
	}
	b, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[booking.Booking])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, booking.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get booking by saga ID: %w", err)
	}
	return b, nil
}

// Insert bookings table
func (r *BookingRepository) Create(ctx context.Context, booking *booking.Booking) (string, error) {
	const sqlBooking = `
		INSERT INTO bookings (
			saga_id, reservation_id,
			user_id, user_name, user_email, user_phone_number,
			room_id, room_number, room_type, 
			check_in_date, check_out_date, number_of_guests,
			total_amount, currency, nightly_rates,
			status, payment_status, expires_at
		) VALUES (
			$1, $2,
			$3, $4, $5,$6,
			$7,	$8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18
		) RETURNING id`
	if err := r.pool.QueryRow(ctx, sqlBooking,
		booking.SagaID, booking.ReservationID,
		booking.UserID, booking.UserName, booking.UserEmail, booking.UserPhoneNumber,
		booking.RoomID, booking.RoomNumber, booking.RoomType,
		booking.CheckInDate, booking.CheckOutDate, booking.NumberOfGuests,
		booking.TotalAmount, booking.Currency, booking.NightlyRates,
		booking.Status, booking.PaymentStatus, booking.ExpiresAt).Scan(&booking.ID); err != nil {
		return "", fmt.Errorf("failed to insert booking: %w", err)
	}
	return booking.ID, nil
}

func (r *BookingRepository) UpdateStatus(ctx context.Context, id string, status booking.Status) error {
	const sql = `UPDATE bookings SET status = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, sql, id, status); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}
