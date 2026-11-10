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

var _ booking.Repository = (*BookingRepository)(nil)

func NewBookingRepository(pool *pgxpool.Pool) *BookingRepository {
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
		return nil, fmt.Errorf("failed to get booking by ID: %w", err)
	}
	b, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[booking.Booking])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, booking.ErrNotFound
		}
		return nil, fmt.Errorf("failed to collect booking row by ID: %w", err)
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
		return nil, fmt.Errorf("failed to get booking by user ID: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[booking.Booking])
}

func (r *BookingRepository) getBySagaID(ctx context.Context, sagaID string) (*booking.Booking, error) {
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
		return nil, fmt.Errorf("failed to get booking by saga ID: %w", err)
	}
	b, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[booking.Booking])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, booking.ErrNotFound
		}
		return nil, fmt.Errorf("failed to collect booking row by saga ID: %w", err)
	}
	return b, nil
}

// Create inserts b and returns the stored row as a new value; b is not modified. Idempotent by saga_id:
// a repeated call returns the existing row as stored, even if the rest of b differs.
func (r *BookingRepository) Create(ctx context.Context, b *booking.Booking) (*booking.Booking, error) {
	const sql = `
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
		)
		ON CONFLICT (saga_id) DO NOTHING
		RETURNING id, created_at, updated_at`

	// Copy, so the caller's b keeps what it sent. The rest of the row is what was inserted; the NightlyRates
	// slice is shared with b, which is fine: nobody writes to it after this point
	stored := *b
	err := r.pool.QueryRow(ctx, sql,
		b.SagaID, b.ReservationID,
		b.UserID, b.UserName, b.UserEmail, b.UserPhoneNumber,
		b.RoomID, b.RoomNumber, b.RoomType,
		b.CheckInDate, b.CheckOutDate, b.NumberOfGuests,
		b.TotalAmount, b.Currency, b.NightlyRates,
		b.Status, b.PaymentStatus, b.ExpiresAt).Scan(&stored.ID, &stored.CreatedAt, &stored.UpdatedAt)
	if err != nil {
		// No row returned = the insert was skipped: a call with this saga_id committed first
		if errors.Is(err, pgx.ErrNoRows) {
			existing, getErr := r.getBySagaID(ctx, b.SagaID)
			if getErr != nil {
				return nil, fmt.Errorf("failed to read booking after saga_id conflict: %w", getErr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("failed to insert booking: %w", err)
	}
	return &stored, nil
}
