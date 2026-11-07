package booking

import (
	"encoding/json"
	"fmt"
	"time"
)

type NightlyRate struct {
	Date  time.Time `json:"date"` // midnight UTC; JSON "2026-10-25", see MarshalJSON
	Price int64     `json:"price"`
}

// nightlyRateJSON is the wire form of NightlyRate: date as YYYY-MM-DD, like the proto, not RFC 3339
type nightlyRateJSON struct {
	Date  string `json:"date"`
	Price int64  `json:"price"`
}

func (n NightlyRate) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		nightlyRateJSON{
			Date:  n.Date.Format(time.DateOnly),
			Price: n.Price,
		})
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
