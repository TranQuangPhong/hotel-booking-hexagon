package booking

import "time"

type NightlyRate struct {
	ID        int64     `db:"id" json:"id"`
	BookingID string    `db:"booking_id" json:"booking_id"`
	Date      time.Time `db:"date" json:"date"`
	Price     int64     `db:"price" json:"price"`
}
