package saga

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// fakeRepo plays the Postgres adapter: Create sets what the DB fills, Save bumps the version. saves keeps a copy of
// each saga Save received, in order; saveErr makes every Save fail
type fakeRepo struct {
	saves   []Saga
	saveErr error
}

func (f *fakeRepo) Create(_ context.Context, s *Saga) (*Saga, error) {
	stored := *s
	stored.ID, stored.Version = "saga-1", 1
	return &stored, nil
}

func (f *fakeRepo) Save(_ context.Context, s *Saga) (*Saga, error) {
	f.saves = append(f.saves, *s)
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	stored := *s
	stored.Version++
	return &stored, nil
}

// fakeRoom answers Reserve with res or err and keeps the saga it was called with
type fakeRoom struct {
	res *Reservation
	err error
	got *Saga
}

func (f *fakeRoom) Reserve(_ context.Context, s *Saga) (*Reservation, error) {
	f.got = s
	return f.res, f.err
}

// fakeBooking answers Create with booking-1 / PENDING or err and keeps the saga it was called with
type fakeBooking struct {
	err   error
	calls int
	got   *Saga
}

func (f *fakeBooking) Create(_ context.Context, s *Saga) (string, string, error) {
	f.calls++
	f.got = s
	if f.err != nil {
		return "", "", f.err
	}
	return "booking-1", "PENDING", nil
}

var testReq = StartRequest{
	User:           User{ID: "u1", Name: "An", Email: "an@example.com"},
	RoomID:         "r1",
	CheckInDate:    "2026-12-01",
	CheckOutDate:   "2026-12-03",
	NumberOfGuests: 2,
}

func testReservation() *Reservation {
	return &Reservation{
		ID: "res-1", RoomNumber: "101", RoomType: "DELUXE", Currency: "USD", TotalAmount: 25000,
		NightlyRates: []NightlyRate{{"2026-12-01", 12000}, {"2026-12-02", 13000}},
	}
}

func newTestService(repo *fakeRepo, room *fakeRoom, booking *fakeBooking) *Service {
	return NewService(repo, room, booking, 15*time.Minute)
}

// Happy path: two Saves, each continuing from the stored saga (the version Save receives is the one it returned
// before), and what is checked is what was saved and sent, not only the 201
func TestStartBooking(t *testing.T) {
	repo, room, booking := &fakeRepo{}, &fakeRoom{res: testReservation()}, &fakeBooking{}
	before := time.Now().UTC()

	got, err := newTestService(repo, room, booking).StartBooking(t.Context(), testReq)
	if err != nil {
		t.Fatalf("StartBooking: %v", err)
	}

	// Reserve gets the stored saga: its id, and the deadline_at the booking and the 201 share
	if room.got.ID != "saga-1" || room.got.Version != 1 {
		t.Errorf("Reserve got id / version %q / %d, want saga-1 / 1", room.got.ID, room.got.Version)
	}
	deadline := room.got.DeadlineAt
	if d := deadline.Sub(before); d < 15*time.Minute || d > 16*time.Minute {
		t.Errorf("deadline_at = now + %v, want now + 15m (holdTTL)", d)
	}

	wantCtx := Context{
		User:           testReq.User,
		Room:           Room{ID: "r1", Number: "101", Type: "DELUXE"},
		CheckInDate:    "2026-12-01",
		CheckOutDate:   "2026-12-03",
		NumberOfGuests: 2,
		ReservationID:  "res-1",
		TotalAmount:    25000,
		Currency:       "USD",
		NightlyRates:   []NightlyRate{{"2026-12-01", 12000}, {"2026-12-02", 13000}},
	}

	if len(repo.saves) != 2 {
		t.Fatalf("Save called %d times, want 2", len(repo.saves))
	}
	reserved, created := repo.saves[0], repo.saves[1]
	if reserved.Status != StatusInProgress || reserved.Step != StepCreatingBooking || reserved.Version != 1 {
		t.Errorf("Save 1 = %s / %s, version %d; want IN_PROGRESS / CREATING_BOOKING, version 1",
			reserved.Status, reserved.Step, reserved.Version)
	}
	if !reflect.DeepEqual(reserved.Context, wantCtx) {
		t.Errorf("Save 1 context = %+v, want %+v", reserved.Context, wantCtx)
	}

	// CreateBooking gets the saga as Save 1 stored it
	if booking.got.Version != 2 || !reflect.DeepEqual(booking.got.Context, wantCtx) ||
		!booking.got.DeadlineAt.Equal(deadline) {
		t.Errorf("CreateBooking got version %d, context %+v, deadline %v; want 2, %+v, %v",
			booking.got.Version, booking.got.Context, booking.got.DeadlineAt, wantCtx, deadline)
	}

	if created.Status != StatusInProgress || created.Step != StepAwaitingPayment || created.Version != 2 ||
		created.BookingID != "booking-1" || created.FailureReason != "" {
		t.Errorf("Save 2 = %s / %s, version %d, booking_id %q, reason %q; "+
			"want IN_PROGRESS / AWAITING_PAYMENT, version 2, booking-1, \"\"",
			created.Status, created.Step, created.Version, created.BookingID, created.FailureReason)
	}
	if !reflect.DeepEqual(created.Context, wantCtx) {
		t.Errorf("Save 2 context = %+v, want %+v", created.Context, wantCtx)
	}

	want := &Started{BookingID: "booking-1", BookingStatus: "PENDING", TotalAmount: 25000, Currency: "USD",
		ExpiresAt: deadline}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Started = %+v, want %+v", got, want)
	}
}

// A definite rejection: one Save (FAILED / RESERVING_ROOM / ROOM_REJECTED), no CreateBooking, and the error is
// returned as room sent it. When that Save fails, the rejection is still the answer (nothing is held)
func TestStartBookingRoomRejected(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		saveErr error
	}{
		{name: "not found", err: fmt.Errorf("%w: room r1", ErrRoomNotFound)},
		{name: "invalid input", err: fmt.Errorf("%w: check_in_date: must not be in the past", ErrInvalidInput)},
		{name: "unavailable", err: fmt.Errorf("%w: dates taken", ErrRoomUnavailable)},
		{name: "save fails", err: ErrRoomNotFound, saveErr: errors.New("db down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, booking := &fakeRepo{saveErr: tt.saveErr}, &fakeBooking{}

			got, err := newTestService(repo, &fakeRoom{err: tt.err}, booking).StartBooking(t.Context(), testReq)

			// The same value, not only errors.Is: unwrapped, so the 400 body is room's message
			if got != nil || err != tt.err {
				t.Errorf("StartBooking = %v, %v; want nil, %v", got, err, tt.err)
			}
			if len(repo.saves) != 1 {
				t.Fatalf("Save called %d times, want 1", len(repo.saves))
			}
			if s := repo.saves[0]; s.Status != StatusFailed || s.Step != StepReservingRoom ||
				s.FailureReason != ReasonRoomRejected || s.Version != 1 {
				t.Errorf("Save = %s / %s / %q, version %d; want FAILED / RESERVING_ROOM / ROOM_REJECTED, version 1",
					s.Status, s.Step, s.FailureReason, s.Version)
			}
			if booking.calls != 0 {
				t.Errorf("CreateBooking called %d times, want 0", booking.calls)
			}
		})
	}
}

// An unknown ReserveRoom outcome (timeout, UNAVAILABLE, INTERNAL): a hold may exist, so nothing is saved and the saga
// stays IN_PROGRESS / RESERVING_ROOM for the M4a worker. The error is not a rejection (503)
func TestStartBookingRoomUnknown(t *testing.T) {
	repo, booking := &fakeRepo{}, &fakeBooking{}
	roomErr := fmt.Errorf("ReserveRoom: %w", context.DeadlineExceeded)

	got, err := newTestService(repo, &fakeRoom{err: roomErr}, booking).StartBooking(t.Context(), testReq)

	if got != nil || !errors.Is(err, context.DeadlineExceeded) || isRoomRejection(err) {
		t.Errorf("StartBooking = %v, %v; want nil and a non-rejection wrapping %v", got, err, roomErr)
	}
	if len(repo.saves) != 0 || booking.calls != 0 {
		t.Errorf("Save / CreateBooking called %d / %d times, want 0 / 0", len(repo.saves), booking.calls)
	}
}

// CreateBooking fails: only RoomReserved was saved, so the saga stays at CREATING_BOOKING with the reservation in its
// context (the hold stays for the M4a worker), and the error is a 503
func TestStartBookingBookingError(t *testing.T) {
	repo := &fakeRepo{}
	bookingErr := errors.New("CreateBooking: unavailable")

	got, err := newTestService(repo, &fakeRoom{res: testReservation()}, &fakeBooking{err: bookingErr}).
		StartBooking(t.Context(), testReq)

	if got != nil || !errors.Is(err, bookingErr) || isRoomRejection(err) {
		t.Errorf("StartBooking = %v, %v; want nil and a non-rejection wrapping %v", got, err, bookingErr)
	}
	if len(repo.saves) != 1 {
		t.Fatalf("Save called %d times, want 1", len(repo.saves))
	}
	if s := repo.saves[0]; s.Step != StepCreatingBooking || s.Context.ReservationID != "res-1" {
		t.Errorf("Save = %s, reservation_id %q; want CREATING_BOOKING, res-1", s.Step, s.Context.ReservationID)
	}
}

// A Save error after RoomReserved (ErrStale too: no other writer before M4a) is a 503 and CreateBooking isn't called
func TestStartBookingSaveError(t *testing.T) {
	repo, booking := &fakeRepo{saveErr: ErrStale}, &fakeBooking{}

	got, err := newTestService(repo, &fakeRoom{res: testReservation()}, booking).StartBooking(t.Context(), testReq)

	if got != nil || !errors.Is(err, ErrStale) || isRoomRejection(err) {
		t.Errorf("StartBooking = %v, %v; want nil and a non-rejection wrapping ErrStale", got, err)
	}
	if booking.calls != 0 {
		t.Errorf("CreateBooking called %d times, want 0", booking.calls)
	}
}

// advance with an input that isn't a row for the saga's state: an error, fill and Save are not called
func TestAdvanceNotApplicable(t *testing.T) {
	repo := &fakeRepo{}
	sg := &Saga{ID: "saga-1", Status: StatusFailed, Step: StepReservingRoom}

	got, err := newTestService(repo, &fakeRoom{}, &fakeBooking{}).advance(t.Context(), sg, InputRoomReserved,
		func(*Saga) { t.Error("fill called") })

	if got != nil || err == nil {
		t.Errorf("advance = %v, %v; want nil and an error", got, err)
	}
	if len(repo.saves) != 0 {
		t.Errorf("Save called %d times, want 0", len(repo.saves))
	}
}
