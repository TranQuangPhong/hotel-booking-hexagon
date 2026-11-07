package booking

import (
	"encoding/json"
	"testing"
)

func TestNightlyRateJSON(t *testing.T) {
	rates := []NightlyRate{{date(2026, 10, 25), 12000}, {date(2026, 10, 26), 15000}}

	b, err := json.Marshal(rates)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `[{"date":"2026-10-25","price":12000},{"date":"2026-10-26","price":15000}]`
	if string(b) != want {
		t.Fatalf("marshal = %s, want %s", b, want)
	}

	var got []NightlyRate
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != len(rates) {
		t.Fatalf("got %d rates, want %d", len(got), len(rates))
	}
	for i := range rates {
		if !got[i].Date.Equal(rates[i].Date) || got[i].Price != rates[i].Price {
			t.Errorf("rate %d = %+v, want %+v", i, got[i], rates[i])
		}
	}
}

func TestNightlyRateJSONBadDate(t *testing.T) {
	var n NightlyRate
	if err := json.Unmarshal([]byte(`{"date":"2026-10-25T00:00:00Z","price":100}`), &n); err == nil {
		t.Fatal("RFC 3339 date accepted, want an error: the stored form is YYYY-MM-DD")
	}
}
