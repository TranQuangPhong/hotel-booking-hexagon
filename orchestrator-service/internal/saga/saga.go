package saga

import "time"

// Saga maps to the saga_instances table (db tags = column names). booking_id and failure_reason are NULL in the
// table and "" here; the repository maps between them
type Saga struct {
	ID            string        `db:"saga_id"`
	Type          Type          `db:"type"`
	Status        Status        `db:"status"`
	Step          Step          `db:"current_step"`
	BookingID     string        `db:"booking_id"` // "" until CreateBooking succeeds
	Context       Context       `db:"context"`
	FailureReason FailureReason `db:"failure_reason"` // "" while on the main line
	DeadlineAt    time.Time     `db:"deadline_at"`    // = ReserveRoom's and CreateBooking's expires_at, the 201's expires_at
	Version       int           `db:"version"`
	CreatedAt     time.Time     `db:"created_at"`
	UpdatedAt     time.Time     `db:"updated_at"`
}

// Context is the context JSONB: everything the next call needs. The json tags are the stored format, so renaming
// one changes what is stored
type Context struct {
	User           User          `json:"user"`
	Room           Room          `json:"room"`           // ID from the request; Number, Type from ReserveRoom
	CheckInDate    string        `json:"check_in_date"`  // YYYY-MM-DD, as sent: room svc checks them
	CheckOutDate   string        `json:"check_out_date"` // YYYY-MM-DD, as sent: room svc checks them
	NumberOfGuests int           `json:"number_of_guests"`
	ReservationID  string        `json:"reservation_id,omitempty"` // from here on: set from ReserveRoom's response
	TotalAmount    int64         `json:"total_amount,omitempty"`   // minor units
	Currency       string        `json:"currency,omitempty"`
	NightlyRates   []NightlyRate `json:"nightly_rates,omitempty"`
}

type User struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

type Room struct {
	ID     string `json:"id,omitempty"`
	Number string `json:"number,omitempty"`
	Type   string `json:"type,omitempty"`
}

type NightlyRate struct {
	Date  string `json:"date"`  // YYYY-MM-DD
	Price int64  `json:"price"` // minor units
}

// The enums below have the same values as their Postgres ENUM in create_saga_instances.sql. No IsValid(): no value
// comes from outside, and the ENUM rejects a wrong one at the write

// Type: saga_type
type Type string

const TypeCreateBooking Type = "CREATE_BOOKING"

// Status: saga_status
type Status string

const (
	StatusInProgress   Status = "IN_PROGRESS"
	StatusCompensating Status = "COMPENSATING"
	StatusCompleted    Status = "COMPLETED"
	StatusFailed       Status = "FAILED"
)

// Step: saga_step. A saga that ends keeps its last step (COMPLETED and FAILED are statuses)
type Step string

const (
	StepReservingRoom     Step = "RESERVING_ROOM"
	StepCreatingBooking   Step = "CREATING_BOOKING"
	StepAwaitingPayment   Step = "AWAITING_PAYMENT"
	StepConfirmingRoom    Step = "CONFIRMING_ROOM"
	StepCapturingPayment  Step = "CAPTURING_PAYMENT"
	StepConfirmingBooking Step = "CONFIRMING_BOOKING"
	StepReleasingRoom     Step = "RELEASING_ROOM"
	StepExpiringBooking   Step = "EXPIRING_BOOKING"
)

// FailureReason: saga_failure_reason. Set by the Save that leaves the main line, never cleared
type FailureReason string

const (
	ReasonRoomRejected  FailureReason = "ROOM_REJECTED"
	ReasonPhaseAError   FailureReason = "PHASE_A_ERROR"
	ReasonDeadline      FailureReason = "DEADLINE"
	ReasonCaptureFailed FailureReason = "CAPTURE_FAILED"
)

// newSaga builds the row StartBooking inserts: CREATE_BOOKING, IN_PROGRESS, RESERVING_ROOM,
// context from req, deadline_at = now + holdTTL. now is a parameter so the test can fix it
func newSaga(req StartRequest, now time.Time, holdTTL time.Duration) *Saga {
	return &Saga{
		Type:   TypeCreateBooking,
		Status: StatusInProgress,
		Step:   StepReservingRoom,
		Context: Context{
			User:           req.User,
			Room:           Room{ID: req.RoomID}, // Number, Type: set from ReserveRoom's response
			CheckInDate:    req.CheckInDate,
			CheckOutDate:   req.CheckOutDate,
			NumberOfGuests: req.NumberOfGuests,
		},
		DeadlineAt: now.Add(holdTTL),
	}
}
