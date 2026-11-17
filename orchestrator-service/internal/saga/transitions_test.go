package saga

import (
	"reflect"
	"testing"
	"time"
)

// wantRows is state-machines.md's table, phase A rows: (status, step, input) -> outcome. Each milestone adds its rows
// here and its new values to the all* lists below
var wantRows = map[row]outcome{
	{StatusInProgress, StepReservingRoom, InputRoomReserved}: {status: StatusInProgress, step: StepCreatingBooking},
	{StatusInProgress, StepReservingRoom, InputRoomRejected}: {status: StatusFailed, step: StepReservingRoom,
		failureReason: ReasonRoomRejected},
	{StatusInProgress, StepCreatingBooking, InputBookingCreated}: {status: StatusInProgress, step: StepAwaitingPayment},
}

// Every value of each enum (Go can't list a type's constants): keep in sync with saga.go and transitions.go.
// TestTransitionNotApplicable fails when a row of wantRows uses a value missing here
var (
	allStatuses = []Status{StatusInProgress, StatusCompensating, StatusCompleted, StatusFailed}
	allSteps    = []Step{
		StepReservingRoom, StepCreatingBooking, StepAwaitingPayment, StepConfirmingRoom,
		StepCapturingPayment, StepConfirmingBooking, StepReleasingRoom, StepExpiringBooking,
	}
	allInputs = []Input{InputRoomReserved, InputRoomRejected, InputBookingCreated}
)

// The whole outcome is compared, so a main-line row also proves failureReason stays "" (check_failure_reason).
// DeepEqual, not ==: M4a adds outcome.commands, a slice
func TestTransition(t *testing.T) {
	for r, want := range wantRows {
		got, ok := transition(r.status, r.step, r.in)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("transition(%s, %s, %s) = %+v, %t; want %+v, true", r.status, r.step, r.in, got, ok, want)
		}
	}
}

// Every (status, step, input) that isn't a row gives ok = false: a duplicate, late or lost-race input changes nothing
// (M4a's consumer logs and acks it). Covers the finished statuses too
func TestTransitionNotApplicable(t *testing.T) {
	rowsSeen := 0
	for _, status := range allStatuses {
		for _, step := range allSteps {
			for _, in := range allInputs {
				if _, isRow := wantRows[row{status, step, in}]; isRow {
					rowsSeen++
					continue
				}
				if got, ok := transition(status, step, in); ok {
					t.Errorf("transition(%s, %s, %s) = %+v, true; want ok = false", status, step, in, got)
				}
			}
		}
	}
	if rowsSeen != len(wantRows) {
		t.Errorf("the all* lists reach %d of the %d rows: add the new status, step or input there",
			rowsSeen, len(wantRows))
	}
}

// atReservingRoom: the saga as Create returns it (newSaga + saga_id, version 1 and timestamps from the DB)
func atReservingRoom() *Saga {
	created := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	return &Saga{
		ID:     "s1",
		Type:   TypeCreateBooking,
		Status: StatusInProgress,
		Step:   StepReservingRoom,
		Context: Context{
			User:           User{ID: "u1", Name: "An", Email: "an@example.com"},
			Room:           Room{ID: "r1"},
			CheckInDate:    "2026-12-01",
			CheckOutDate:   "2026-12-03",
			NumberOfGuests: 2,
		},
		DeadlineAt: created.Add(15 * time.Minute),
		Version:    1,
		CreatedAt:  created,
		UpdatedAt:  created,
	}
}

// releasingRoom: a saga whose deadline passed at AWAITING_PAYMENT (M4a's worker): every field set, reason DEADLINE
func releasingRoom() *Saga {
	s := atReservingRoom()
	s.Status, s.Step, s.FailureReason = StatusCompensating, StepReleasingRoom, ReasonDeadline
	s.BookingID = "b1"
	s.Context.Room.Number, s.Context.Room.Type = "101", "DELUXE"
	s.Context.ReservationID = "res1"
	s.Context.TotalAmount, s.Context.Currency = 25000, "USD"
	s.Context.NightlyRates = []NightlyRate{{"2026-12-01", 12000}, {"2026-12-02", 13000}}
	s.Version = 4
	s.UpdatedAt = s.DeadlineAt.Add(10 * time.Second)
	return s
}

func TestNext(t *testing.T) {
	tests := []struct {
		name       string
		from       *Saga
		out        outcome
		wantReason FailureReason
	}{
		{
			name: "main line: no reason before or after",
			from: atReservingRoom(),
			out:  outcome{status: StatusInProgress, step: StepCreatingBooking},
		},
		{
			name:       "leaving the main line sets the reason",
			from:       atReservingRoom(),
			out:        outcome{status: StatusFailed, step: StepReservingRoom, failureReason: ReasonRoomRejected},
			wantReason: ReasonRoomRejected,
		},
		{
			name:       "an outcome without a reason keeps the saga's",
			from:       releasingRoom(),
			out:        outcome{status: StatusCompensating, step: StepExpiringBooking},
			wantReason: ReasonDeadline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := *tt.from
			got := tt.from.next(tt.out)

			// next sets status, step and the reason; every other field (Version too: Save checks it) is copied
			want := before
			want.Status, want.Step, want.FailureReason = tt.out.status, tt.out.step, tt.wantReason
			if !reflect.DeepEqual(*got, want) {
				t.Errorf("next = %+v,\nwant %+v", *got, want)
			}
			if !reflect.DeepEqual(*tt.from, before) {
				t.Errorf("next modified s: %+v,\nwant %+v", *tt.from, before)
			}
		})
	}
}
