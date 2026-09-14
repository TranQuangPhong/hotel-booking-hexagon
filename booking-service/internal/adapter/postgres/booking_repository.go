package postgres

import (
	"booking/booking-service/internal/booking"
	"context"
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

func (r *BookingRepository) GetBookingDetailByID(ctx context.Context, id string) (*booking.BookingDetail, error) {
	// Select bookings table
	sqlBooking := `select id,
	 user_id, user_name, user_email, user_phone_number,
	 room_id, room_number, room_type,
	 check_in_date, check_out_date,
	 number_of_guests, total_amount, currency,
	 status, payment_status,
	 created_at, updated_at
	 from bookings
	 where id = $1`

	var b booking.BookingDetail
	if err := r.pool.QueryRow(ctx, sqlBooking, id).Scan(&b.ID,
		&b.UserID, &b.UserName, &b.UserEmail, &b.UserPhoneNumber,
		&b.RoomID, &b.RoomNumber, &b.RoomType,
		&b.CheckInDate, &b.CheckOutDate,
		&b.NumberOfGuests, &b.TotalAmount, &b.Currency,
		&b.Status, &b.PaymentStatus,
		&b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, fmt.Errorf("select booking: %w", err)
	}

	// Select nightly_rates table
	sqlNightlyRates := `select id, booking_id, date, price from nightly_rates where booking_id = $1`
	rows, err := r.pool.Query(ctx, sqlNightlyRates, id)
	if err != nil {
		return nil, fmt.Errorf("select nightly rates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var nr booking.NightlyRate
		if err := rows.Scan(&nr.ID, &nr.BookingID, &nr.Date, &nr.Price); err != nil {
			return nil, fmt.Errorf("scan nightly rate: %w", err)
		}
		b.NightlyRates = append(b.NightlyRates, nr)
	}
	// Check iteration error
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nightly rates: %w", err)
	}

	return &b, nil
}

func (r *BookingRepository) GetBookingDetailByUserID(ctx context.Context, userID string) ([]*booking.BookingDetail, error) {
	// Select bookings table
	sqlBooking := `
	SELECT id,
	 	user_id, user_name, user_email, user_phone_number,
	 	room_id, room_number, room_type,
	 	heck_in_date, check_out_date,
	 	number_of_guests, total_amount, currency,
	 	status, payment_status,
	 	created_at, updated_at
	 FROM bookings
	 WHERE user_id = $1`

	bookingRows, err := r.pool.Query(ctx, sqlBooking, userID)
	if err != nil {
		return nil, fmt.Errorf("select booking: %w", err)
	}
	defer bookingRows.Close()

	var bookings []*booking.BookingDetail
	var bookingIDs []string

	for bookingRows.Next() {
		var b booking.BookingDetail
		if err := bookingRows.Scan(&b.ID,
			&b.UserID, &b.UserName, &b.UserEmail, &b.UserPhoneNumber,
			&b.RoomID, &b.RoomNumber, &b.RoomType,
			&b.CheckInDate, &b.CheckOutDate,
			&b.NumberOfGuests, &b.TotalAmount, &b.Currency,
			&b.Status, &b.PaymentStatus,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		bookings = append(bookings, &b)
		bookingIDs = append(bookingIDs, b.ID)
	}
	if err := bookingRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookings: %w", err)
	}

	// Select nightly_rates table
	sqlNightlyRates := `SELECT id, booking_id, date, price FROM nightly_rates WHERE booking_id = ANY($1)`
	nightlyRateRows, err := r.pool.Query(ctx, sqlNightlyRates, bookingIDs)
	if err != nil {
		return nil, fmt.Errorf("select nightly rates: %w", err)
	}
	defer nightlyRateRows.Close()

	ratesMap := make(map[string][]booking.NightlyRate, len(bookingIDs))
	for nightlyRateRows.Next() {
		var nr booking.NightlyRate
		if err := nightlyRateRows.Scan(&nr.ID, &nr.BookingID, &nr.Date, &nr.Price); err != nil {
			return nil, fmt.Errorf("scan nightly rate: %w", err)
		}
		ratesMap[nr.BookingID] = append(ratesMap[nr.BookingID], nr)
	}
	// Check iteration error
	if err := nightlyRateRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nightly rates: %w", err)
	}

	for _, b := range bookings {
		b.NightlyRates = ratesMap[b.ID]
	}

	return bookings, nil
}

func (r *BookingRepository) CreateBooking(ctx context.Context, booking *booking.BookingDetail) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op if Commit succeeds

	// Insert bookings table
	const sqlBooking = `
		INSERT INTO bookings (
			user_id, user_name, user_email, user_phone_number,
			room_id, room_number, room_type, 
			check_in_date, check_out_date, number_of_guests,
			total_amount, currency,
			status, payment_status
		) VALUES (
			$1, $2, $3, $4,
			$5,$6, $7,
			$8, $9,	$10,
			$11, $12,
			$13, $14
		) RETURNING id`
	if err := tx.QueryRow(ctx, sqlBooking,
		booking.UserID, booking.UserName, booking.UserEmail, booking.UserPhoneNumber,
		booking.RoomID, booking.RoomNumber, booking.RoomType,
		booking.CheckInDate, booking.CheckOutDate, booking.NumberOfGuests,
		booking.TotalAmount, booking.Currency,
		booking.Status, booking.PaymentStatus).Scan(&booking.ID); err != nil {
		return "", fmt.Errorf("insert booking: %w", err)
	}

	// Insert nightly_rates table
	if len(booking.NightlyRates) > 0 {
		batch := &pgx.Batch{}
		const sqlNightlyRates = `INSERT INTO booking_nightly_rates (booking_id, date, price) VALUES ($1, $2, $3)`
		for i := range booking.NightlyRates {
			batch.Queue(sqlNightlyRates, booking.ID, booking.NightlyRates[i].Date, booking.NightlyRates[i].Price)
		}
		br := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return "", fmt.Errorf("insert nightly rate: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return "", fmt.Errorf("close batch: %w", err)
		}
	}
	// Commit tx
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}
	// Final return
	return booking.ID, nil
}

func (r *BookingRepository) UpdateBookingStatus(ctx context.Context, id string, status booking.BookingStatus) error {
	const sql = `UPDATE bookings SET status = $1, updated_at = NOW() WHERE id = $2`

	if _, err := r.pool.Exec(ctx, sql, id, status); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}
