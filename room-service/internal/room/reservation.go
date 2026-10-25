package room

import (
	"encoding/json"
	"fmt"
	"time"
)

type Reservation struct {
	ID           string            `json:"id" db:"id"`
	SagaID       string            `json:"saga_id" db:"saga_id"`
	RoomID       string            `json:"room_id" db:"room_id"`
	CheckInDate  time.Time         `json:"check_in_date" db:"check_in_date"`
	CheckOutDate time.Time         `json:"check_out_date" db:"check_out_date"`
	Status       ReservationStatus `json:"status" db:"status"`
	TotalAmount  int64             `json:"total_amount" db:"total_amount"`
	Currency     string            `json:"currency" db:"currency"`
	NightlyRates []NightlyRate     `json:"nightly_rates" db:"nightly_rates"` // json array [{date, price}]. Eg: {"date":"2026-10-20","price":10000}
	ExpiresAt    time.Time         `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at" db:"updated_at"`
}

type ReservationStatus string
type NightlyRate struct {
	Date  time.Time `json:"date"` // midnight UTC; JSON "2026-10-20", see MarshalJSON
	Price int64     `json:"price"`
}

// nightlyRateJSON is the wire form of NightlyRate: date as YYYY-MM-DD, like the proto, not RFC 3339
type nightlyRateJSON struct {
	Date  string `json:"date"`
	Price int64  `json:"price"`
}

func (n NightlyRate) MarshalJSON() ([]byte, error) {
	return json.Marshal(nightlyRateJSON{Date: n.Date.Format(time.DateOnly), Price: n.Price})
}

func (n *NightlyRate) UnmarshalJSON(b []byte) error {
	var raw nightlyRateJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	date, err := time.Parse(time.DateOnly, raw.Date)
	if err != nil {
		return fmt.Errorf("failed to parse nightly rate date %q: %w", raw.Date, err)
	}
	n.Date, n.Price = date, raw.Price
	return nil
}

const (
	Reserved  ReservationStatus = "RESERVED"
	Confirmed ReservationStatus = "CONFIRMED"
	Released  ReservationStatus = "RELEASED"
)

func (s ReservationStatus) IsValid() bool {
	switch s {
	case Reserved, Confirmed, Released:
		return true
	}
	return false
}

// setRates fills NightlyRates, TotalAmount and Currency from the inventory rows covering [CheckInDate, CheckOutDate).
// No I/O. r is left unchanged on error.
func (r *Reservation) setRates(inventories []*Inventory) error {
	if !r.CheckInDate.Before(r.CheckOutDate) {
		return fmt.Errorf("bad input date range: %w", ErrInvalidInput)
	}
	nightlyRates := make([]NightlyRate, 0, int(r.CheckOutDate.Sub(r.CheckInDate).Hours()/24))
	var total int64
	for night := r.CheckInDate; night.Before(r.CheckOutDate); night = night.AddDate(0, 0, 1) {
		rate, err := getNightRate(night, inventories)
		if err != nil {
			return err
		}
		nightlyRates = append(nightlyRates, rate)
		total += rate.Price
	}
	r.NightlyRates = nightlyRates
	r.TotalAmount = total
	r.Currency = inventories[0].Currency
	return nil
}

// getNightRate loops inventories to get nightly rate for specific target reservation night
// Returns NightlyRate if target night found.
// Returns empty NightlyRate & ErrRoomUnavailable if not found or status not available
func getNightRate(night time.Time, inventories []*Inventory) (NightlyRate, error) {
	for _, inv := range inventories {
		if night.Year() == int(inv.Year) && night.Month() == time.Month(inv.Month) {
			day, ok := inv.Days[int8(night.Day())]
			if !ok {
				return NightlyRate{}, fmt.Errorf("no inventory for %s: %w", night.Format(time.DateOnly), ErrRoomUnavailable)
			}
			if day.Status != DayAvailable {
				return NightlyRate{}, fmt.Errorf("%s is in maintenance: %w", night.Format(time.DateOnly), ErrRoomUnavailable)
			}
			rate := NightlyRate{
				Date:  night,
				Price: day.Price,
			}
			return rate, nil
		}
	}
	return NightlyRate{}, fmt.Errorf("no inventory for %s: %w", night.Format(time.DateOnly), ErrRoomUnavailable)
}
