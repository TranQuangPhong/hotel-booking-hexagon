package saga

// Input: what happened to the saga, one per row of the transition table (a gRPC result in M3; events, the deadline
// worker and POST .../payment later). Never stored, so no Postgres ENUM: the values are for logs
type Input string

const (
	InputRoomReserved   Input = "ROOM_RESERVED"
	InputRoomRejected   Input = "ROOM_REJECTED"
	InputBookingCreated Input = "BOOKING_CREATED"
)

// outcome: what transition decides. M4a adds commands []Command
type outcome struct {
	status        Status
	step          Step
	failureReason FailureReason // "" = keep the saga's: set once, by the row that leaves the main line
}

// row: the key of one line of the table (current status, current step, input)
type row struct {
	status Status
	step   Step
	in     Input
}

// transition is state-machines.md's table: no I/O, one case per row. ok = false: in doesn't apply in this state
// (duplicate, late, lost a race). The caller decides: StartBooking returns an error (it made the saga, so it is a
// bug); M4a's consumer logs and acks
func transition(status Status, step Step, in Input) (out outcome, ok bool) {
	switch (row{status, step, in}) {
	case row{StatusInProgress, StepReservingRoom, InputRoomReserved}:
		return outcome{status: StatusInProgress, step: StepCreatingBooking}, true
	case row{StatusInProgress, StepReservingRoom, InputRoomRejected}:
		return outcome{status: StatusFailed, step: StepReservingRoom, failureReason: ReasonRoomRejected}, true
	case row{StatusInProgress, StepCreatingBooking, InputBookingCreated}:
		return outcome{status: StatusInProgress, step: StepAwaitingPayment}, true
	}
	return outcome{}, false
}

// next returns a copy of s with out applied (status, step, failure_reason if set); s is not modified. The copy is
// shallow: Context.NightlyRates is shared, so a caller assigns a new slice and never writes into it
func (s *Saga) next(out outcome) *Saga {
	ns := *s
	ns.Status = out.status
	ns.Step = out.step
	if out.failureReason != "" {
		ns.FailureReason = out.failureReason
	}

	return &ns
}
