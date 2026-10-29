package room

import (
	"maps"
	"testing"
	"time"
)

// withDay sets one stored day of row and returns row, so a case can build its own row inline
func withDay(row *Inventory, day int8, d Day) *Inventory {
	row.Days[day] = d
	return row
}

// nightsAt maps every date from first to last, both inclusive, as YYYY-MM-DD, to d
func nightsAt(first, last time.Time, d Day) map[string]Day {
	nights := make(map[string]Day)
	for night := first; !night.After(last); night = night.AddDate(0, 0, 1) {
		nights[night.Format(time.DateOnly)] = d
	}
	return nights
}

func TestDayPatchApply(t *testing.T) {
	maintenance, available := DayMaintenance, DayAvailable
	price := int64(500)

	// every case builds its own rows: apply edits them in place
	tests := []struct {
		name        string
		patch       DayPatch
		firstNight  time.Time
		lastNight   time.Time
		inventories []*Inventory
		want        map[string]Day // the patched nights (YYYY-MM-DD); every other stored day must stay as it was
	}{
		{
			name:        "price only keeps each day's status, both ends inclusive",
			patch:       DayPatch{Price: &price},
			firstNight:  date(2026, 10, 10),
			lastNight:   date(2026, 10, 12),
			inventories: []*Inventory{withDay(monthRow(2026, 10, 100), 11, Day{DayMaintenance, 300})},
			want: map[string]Day{
				"2026-10-10": {DayAvailable, 500},
				"2026-10-11": {DayMaintenance, 500},
				"2026-10-12": {DayAvailable, 500},
			},
		},
		{
			name:        "one night",
			patch:       DayPatch{Price: &price},
			firstNight:  date(2026, 10, 15),
			lastNight:   date(2026, 10, 15),
			inventories: []*Inventory{monthRow(2026, 10, 100)},
			want:        map[string]Day{"2026-10-15": {DayAvailable, 500}},
		},
		{
			name:       "status only keeps each day's price, across a month end",
			patch:      DayPatch{Status: &maintenance},
			firstNight: date(2026, 10, 31),
			lastNight:  date(2026, 11, 1),
			inventories: []*Inventory{
				withDay(monthRow(2026, 10, 100), 31, Day{DayAvailable, 150}),
				monthRow(2026, 11, 200),
			},
			want: map[string]Day{
				"2026-10-31": {DayMaintenance, 150},
				"2026-11-01": {DayMaintenance, 200},
			},
		},
		{
			name:       "status and price across a year end",
			patch:      DayPatch{Status: &available, Price: &price},
			firstNight: date(2026, 12, 31),
			lastNight:  date(2027, 1, 1),
			inventories: []*Inventory{
				withDay(monthRow(2026, 12, 100), 31, Day{DayMaintenance, 100}),
				monthRow(2027, 1, 100),
			},
			want: map[string]Day{
				"2026-12-31": {DayAvailable, 500},
				"2027-01-01": {DayAvailable, 500},
			},
		},
		{
			name:        "far last night: every stored day from the first night",
			patch:       DayPatch{Price: &price},
			firstNight:  date(2026, 10, 30),
			lastNight:   date(2099, 12, 31),
			inventories: []*Inventory{monthRow(2026, 10, 100), monthRow(2026, 11, 100)},
			want:        nightsAt(date(2026, 10, 30), date(2026, 11, 30), Day{DayAvailable, 500}),
		},
		{
			// the row stores only days 1 and 2: apply loops the stored days, not the nights, so days 3-31 must not appear
			name:       "a day missing from a row is not added",
			patch:      DayPatch{Price: &price},
			firstNight: date(2026, 10, 1),
			lastNight:  date(2026, 10, 31),
			inventories: []*Inventory{{Year: 2026, Month: 10, Days: map[int8]Day{
				1: {DayAvailable, 100},
				2: {DayAvailable, 100},
			}}},
			want: map[string]Day{
				"2026-10-01": {DayAvailable, 500},
				"2026-10-02": {DayAvailable, 500},
			},
		},
		{
			name:        "empty patch changes nothing",
			firstNight:  date(2026, 10, 1),
			lastNight:   date(2026, 10, 31),
			inventories: []*Inventory{monthRow(2026, 10, 100)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := make([]map[int8]Day, len(tt.inventories))
			for i, inv := range tt.inventories {
				before[i] = maps.Clone(inv.Days)
			}

			tt.patch.apply(tt.inventories, tt.firstNight, tt.lastNight)

			unseen := maps.Clone(tt.want) // what is left after the loop is in want, but no row stores it
			for i, inv := range tt.inventories {
				if len(inv.Days) != len(before[i]) {
					t.Errorf("%d-%02d has %d days, want %d", inv.Year, inv.Month, len(inv.Days), len(before[i]))
				}
				for d, got := range inv.Days {
					night := date(int(inv.Year), time.Month(inv.Month), int(d)).Format(time.DateOnly)
					want, patched := tt.want[night]
					if !patched {
						want = before[i][d]
					}
					delete(unseen, night)
					if got != want {
						t.Errorf("%s = %+v, want %+v", night, got, want)
					}
				}
			}
			for night := range unseen {
				t.Errorf("%s: want it patched, but no row stores it", night)
			}
		})
	}
}
