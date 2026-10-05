package inventory

import "time"

type Inventory struct {
	ID        int64        `json:"id" db:"id"`
	RoomID    string       `json:"room_id" db:"room_id"` // UUID from Room model
	Year      int16        `json:"year" db:"year"`       // YYYY
	Month     int16        `json:"month" db:"month"`     // 1 - 12
	Days      map[int8]Day `json:"days" db:"days"`       // Stored as json for each day
	Currency  string       `json:"currency" db:"currency"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

type Day struct {
	Status DayStatus `json:"status"`
	Price  int64     `json:"price"` // minor-unit value of currency. Eg: USD -> store CENT value
}

// Rate is a nightly price: Price in minor units of Currency (ISO 4217, eg: USD)
type Rate struct {
	Price    int64
	Currency string
}

func (r Rate) IsValid() bool {
	if r.Price <= 0 || len(r.Currency) != 3 {
		return false
	}
	for _, c := range r.Currency {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

type DayStatus string

const (
	StatusAvailable   DayStatus = "AVAILABLE"
	StatusMaintenance DayStatus = "MAINTENANCE"
)

func (s DayStatus) IsValid() bool {
	switch s {
	case StatusAvailable, StatusMaintenance:
		return true
	}
	return false
}

// NewMonths builds inventories for `months` consecutive months, starting at the month of `from` (UTC).
// Every day is AVAILABLE at `rate`. RoomID is left empty: the caller sets it once the room exists.
func NewMonths(from time.Time, months int, rate Rate) []*Inventory {
	from = from.UTC()
	// Normalize to the 1st: AddDate on day 29-31 overflows (Oct 31 + 1 month = Dec 1, November skipped)
	start := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)

	inventories := make([]*Inventory, 0, months)
	for i := range months {
		year, month, _ := start.AddDate(0, i, 0).Date()
		// Day 0 of next month = last day of this month (handles 28/29/30/31)
		daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()

		days := make(map[int8]Day, daysInMonth)
		for d := 1; d <= daysInMonth; d++ {
			days[int8(d)] = Day{Status: StatusAvailable, Price: rate.Price}
		}
		inventories = append(inventories, &Inventory{
			Year:     int16(year),
			Month:    int16(month),
			Days:     days,
			Currency: rate.Currency,
		})
	}
	return inventories
}
