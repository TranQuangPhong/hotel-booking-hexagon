package room

import (
	"errors"
	"testing"
	"time"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// monthRow returns one month of inventory, every day AVAILABLE at price (USD)
func monthRow(year int, month time.Month, price int64) *Inventory {
	return NewInventories(date(year, month, 1), 1, Rate{Price: price, Currency: "USD"})[0]
}

func TestSetRates(t *testing.T) {
	maintenance := monthRow(2026, 11, 200)
	maintenance.Days[11] = Day{Status: DayMaintenance, Price: 200}

	tests := []struct {
		name        string
		checkIn     time.Time
		checkOut    time.Time
		inventories []*Inventory
		wantRates   []NightlyRate
		wantErr     error
	}{
		{
			name:        "stay crossing a month end",
			checkIn:     date(2026, 10, 30),
			checkOut:    date(2026, 11, 2),
			inventories: []*Inventory{monthRow(2026, 10, 100), monthRow(2026, 11, 200)},
			wantRates:   []NightlyRate{{date(2026, 10, 30), 100}, {date(2026, 10, 31), 100}, {date(2026, 11, 1), 200}},
		},
		{
			name:        "stay crossing a year end",
			checkIn:     date(2026, 12, 31),
			checkOut:    date(2027, 1, 2),
			inventories: []*Inventory{monthRow(2026, 12, 300), monthRow(2027, 1, 400)},
			wantRates:   []NightlyRate{{date(2026, 12, 31), 300}, {date(2027, 1, 1), 400}},
		},
		{
			name:        "check-out on the 1st needs no row for that month",
			checkIn:     date(2026, 11, 29),
			checkOut:    date(2026, 12, 1),
			inventories: []*Inventory{monthRow(2026, 11, 200)},
			wantRates:   []NightlyRate{{date(2026, 11, 29), 200}, {date(2026, 11, 30), 200}},
		},
		{
			name:        "maintenance night",
			checkIn:     date(2026, 11, 10),
			checkOut:    date(2026, 11, 12),
			inventories: []*Inventory{maintenance},
			wantErr:     ErrRoomUnavailable,
		},
		{
			name:        "missing month row",
			checkIn:     date(2026, 11, 30),
			checkOut:    date(2026, 12, 2),
			inventories: []*Inventory{monthRow(2026, 11, 200)},
			wantErr:     ErrRoomUnavailable,
		},
		{
			name:     "zero-night stay",
			checkIn:  date(2026, 11, 10),
			checkOut: date(2026, 11, 10),
			wantErr:  ErrInvalidInput,
		},
		{
			name:     "check-out before check-in",
			checkIn:  date(2026, 11, 12),
			checkOut: date(2026, 11, 10),
			wantErr:  ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reservation{CheckInDate: tt.checkIn, CheckOutDate: tt.checkOut}
			err := r.setRates(tt.inventories)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if r.NightlyRates != nil || r.TotalAmount != 0 || r.Currency != "" {
					t.Errorf("reservation changed on error: %+v", r)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if len(r.NightlyRates) != len(tt.wantRates) {
				t.Fatalf("got %d nightly rates, want %d", len(r.NightlyRates), len(tt.wantRates))
			}
			var wantTotal int64
			for i, want := range tt.wantRates {
				got := r.NightlyRates[i]
				if !got.Date.Equal(want.Date) || got.Price != want.Price {
					t.Errorf("night %d = {%s %d}, want {%s %d}", i,
						got.Date.Format(time.DateOnly), got.Price, want.Date.Format(time.DateOnly), want.Price)
				}
				wantTotal += want.Price
			}
			if r.TotalAmount != wantTotal {
				t.Errorf("TotalAmount = %d, want %d", r.TotalAmount, wantTotal)
			}
			if r.Currency != "USD" {
				t.Errorf("Currency = %q, want %q", r.Currency, "USD")
			}
		})
	}
}
