package saga

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestNewSaga(t *testing.T) {
	now := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	req := StartRequest{
		User:           User{ID: "u1", Name: "An", Email: "an@example.com"},
		RoomID:         "r1",
		CheckInDate:    "2026-12-01",
		CheckOutDate:   "2026-12-03",
		NumberOfGuests: 2,
	}

	s := newSaga(req, now, 15*time.Minute)

	if s.Type != TypeCreateBooking || s.Status != StatusInProgress || s.Step != StepReservingRoom {
		t.Errorf("type / status / step = %s / %s / %s, want CREATE_BOOKING / IN_PROGRESS / RESERVING_ROOM",
			s.Type, s.Status, s.Step)
	}
	if want := now.Add(15 * time.Minute); !s.DeadlineAt.Equal(want) {
		t.Errorf("DeadlineAt = %v, want %v", s.DeadlineAt, want)
	}
	wantCtx := Context{
		User:           req.User,
		Room:           Room{ID: "r1"},
		CheckInDate:    "2026-12-01",
		CheckOutDate:   "2026-12-03",
		NumberOfGuests: 2,
	}
	if !reflect.DeepEqual(s.Context, wantCtx) {
		t.Errorf("Context = %+v, want %+v", s.Context, wantCtx)
	}
	// The DB sets these on insert
	if s.ID != "" || s.Version != 0 || s.BookingID != "" || s.FailureReason != "" {
		t.Errorf("ID / Version / BookingID / FailureReason = %q / %d / %q / %q, want all zero",
			s.ID, s.Version, s.BookingID, s.FailureReason)
	}
}

// The json tags are the stored format of the context column. Go decodes keys case-insensitively, so a missing tag
// survives a round trip: only the exact string catches it
func TestContextJSON(t *testing.T) {
	tests := []struct {
		name string
		ctx  Context
		want string
	}{
		{
			name: "at start (newSaga)",
			ctx: Context{
				User:           User{ID: "u1", Name: "An", Email: "an@example.com"},
				Room:           Room{ID: "r1"},
				CheckInDate:    "2026-12-01",
				CheckOutDate:   "2026-12-03",
				NumberOfGuests: 2,
			},
			want: `{"user":{"id":"u1","name":"An","email":"an@example.com"},"room":{"id":"r1"},` +
				`"check_in_date":"2026-12-01","check_out_date":"2026-12-03","number_of_guests":2}`,
		},
		{
			name: "after RoomReserved",
			ctx: Context{
				User:           User{ID: "u1", Name: "An", Email: "an@example.com", PhoneNumber: "0901234567"},
				Room:           Room{ID: "r1", Number: "101", Type: "DELUXE"},
				CheckInDate:    "2026-12-01",
				CheckOutDate:   "2026-12-03",
				NumberOfGuests: 2,
				ReservationID:  "res1",
				TotalAmount:    25000,
				Currency:       "USD",
				NightlyRates:   []NightlyRate{{"2026-12-01", 12000}, {"2026-12-02", 13000}},
			},
			want: `{"user":{"id":"u1","name":"An","email":"an@example.com","phone_number":"0901234567"},` +
				`"room":{"id":"r1","number":"101","type":"DELUXE"},` +
				`"check_in_date":"2026-12-01","check_out_date":"2026-12-03","number_of_guests":2,` +
				`"reservation_id":"res1","total_amount":25000,"currency":"USD",` +
				`"nightly_rates":[{"date":"2026-12-01","price":12000},{"date":"2026-12-02","price":13000}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.ctx)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != tt.want {
				t.Fatalf("marshal =\n%s\nwant\n%s", b, tt.want)
			}

			var got Context
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(got, tt.ctx) {
				t.Errorf("round trip = %+v, want %+v", got, tt.ctx)
			}
		})
	}
}
