package booking

import (
	"errors"
	"testing"
	"time"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// validBooking returns a 2-night stay that passes validate; each test case changes one thing
func validBooking() *Booking {
	return &Booking{
		UserName:       "Phong",
		UserEmail:      "phong@example.com",
		RoomNumber:     "101",
		RoomType:       "DOUBLE",
		CheckInDate:    date(2026, 12, 31),
		CheckOutDate:   date(2027, 1, 2),
		NumberOfGuests: 2,
		TotalAmount:    300,
		Currency:       "USD",
		NightlyRates:   []NightlyRate{{date(2026, 12, 31), 100}, {date(2027, 1, 1), 200}},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		change  func(b *Booking)
		wantErr error
	}{
		{name: "valid, crossing a year end", change: func(b *Booking) {}},
		{name: "empty phone is allowed", change: func(b *Booking) { b.UserPhoneNumber = "" }},
		{name: "rate date in another location, same instant", change: func(b *Booking) {
			b.NightlyRates[0].Date = b.NightlyRates[0].Date.In(time.FixedZone("ICT", 7*60*60))
		}},

		{name: "check-out = check-in", change: func(b *Booking) { b.CheckOutDate = b.CheckInDate }, wantErr: ErrInvalidInput},
		{name: "check-out before check-in", change: func(b *Booking) { b.CheckOutDate = date(2026, 12, 30) }, wantErr: ErrInvalidInput},
		{name: "zero guests", change: func(b *Booking) { b.NumberOfGuests = 0 }, wantErr: ErrInvalidInput},
		{name: "empty user name", change: func(b *Booking) { b.UserName = "" }, wantErr: ErrInvalidInput},
		{name: "empty user email", change: func(b *Booking) { b.UserEmail = "" }, wantErr: ErrInvalidInput},
		{name: "empty room number", change: func(b *Booking) { b.RoomNumber = "" }, wantErr: ErrInvalidInput},
		{name: "empty room type", change: func(b *Booking) { b.RoomType = "" }, wantErr: ErrInvalidInput},
		{name: "empty currency", change: func(b *Booking) { b.Currency = "" }, wantErr: ErrInvalidInput},
		{name: "lowercase currency", change: func(b *Booking) { b.Currency = "usd" }, wantErr: ErrInvalidInput},
		{name: "4-letter currency", change: func(b *Booking) { b.Currency = "USDT" }, wantErr: ErrInvalidInput},
		{name: "no rates", change: func(b *Booking) { b.NightlyRates = nil }, wantErr: ErrInvalidInput},
		{name: "last night missing", change: func(b *Booking) {
			b.NightlyRates = b.NightlyRates[:1]
			b.TotalAmount = 100
		}, wantErr: ErrInvalidInput},
		{name: "one rate too many", change: func(b *Booking) {
			b.NightlyRates = append(b.NightlyRates, NightlyRate{date(2027, 1, 2), 100})
			b.TotalAmount = 400
		}, wantErr: ErrInvalidInput},
		{name: "rates out of order", change: func(b *Booking) {
			b.NightlyRates[0], b.NightlyRates[1] = b.NightlyRates[1], b.NightlyRates[0]
		}, wantErr: ErrInvalidInput},
		{name: "rate not at midnight", change: func(b *Booking) {
			b.NightlyRates[1].Date = b.NightlyRates[1].Date.Add(time.Hour)
		}, wantErr: ErrInvalidInput},
		{name: "zero price", change: func(b *Booking) {
			b.NightlyRates[1].Price = 0
			b.TotalAmount = 100
		}, wantErr: ErrInvalidInput},
		{name: "negative price", change: func(b *Booking) {
			b.NightlyRates[0].Price = -100
			b.TotalAmount = 100
		}, wantErr: ErrInvalidInput},
		{name: "forgotten total", change: func(b *Booking) { b.TotalAmount = 0 }, wantErr: ErrInvalidInput},
		{name: "prices don't sum to total", change: func(b *Booking) { b.TotalAmount = 299 }, wantErr: ErrInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := validBooking()
			tt.change(b)
			err := b.validate()

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
