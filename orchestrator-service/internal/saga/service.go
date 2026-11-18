package saga

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"
)

// Orchestrator use cases, called by the driving adapters:
//   - StartBooking: POST /bookings, phase A (ReserveRoom -> CreateBooking)

type Service struct {
	repository Repository
	room       RoomClient
	booking    BookingClient
	holdTTL    time.Duration
}

func NewService(repo Repository, room RoomClient, booking BookingClient, holdTTL time.Duration) *Service {
	return &Service{repository: repo, room: room, booking: booking, holdTTL: holdTTL}
}

// StartRequest: POST /bookings, filled by the handler
type StartRequest struct {
	User                      User
	RoomID                    string
	CheckInDate, CheckOutDate string // YYYY-MM-DD
	NumberOfGuests            int
}

// Started: the data of the 201
type Started struct {
	BookingID, BookingStatus string // as CreateBooking returned them (PENDING for a new saga)
	TotalAmount              int64
	Currency                 string
	ExpiresAt                time.Time // the saga's deadline_at
}

// StartBooking runs phase A: insert the saga -> ReserveRoom -> Save -> CreateBooking -> Save.
// A definite ReserveRoom rejection (ErrRoomNotFound, ErrInvalidInput, ErrRoomUnavailable) is returned as room sent it,
// and the saga ends FAILED / ROOM_REJECTED. Any other error is a 503: the saga stays at its step and a hold may
// exist, which the M4a deadline worker releases
func (s *Service) StartBooking(ctx context.Context, req StartRequest) (*Started, error) {
	sg, err := s.repository.Create(ctx, newSaga(req, time.Now().UTC(), s.holdTTL))
	if err != nil {
		return nil, fmt.Errorf("failed to create saga: %w", err)
	}

	// sg is the stored row from here on: room, booking and the 201 get the DB's deadline_at
	r, err := s.room.Reserve(ctx, sg)
	if err != nil {
		if !isRoomRejection(err) {
			// Unknown outcome: save nothing, the saga stays at RESERVING_ROOM for the M4a worker
			return nil, fmt.Errorf("failed to reserve room: %w", err)
		}
		// Nothing is held, so the rejection is the client's true answer even when this Save fails
		if _, saveErr := s.advance(ctx, sg, InputRoomRejected, nil); saveErr != nil {
			slog.ErrorContext(ctx, "failed to save the rejected saga", "saga_id", sg.ID, "error", saveErr)
		}
		// Not wrapped: the 400 body is err.Error(), and room's message already names the field
		return nil, err
	}

	sg, err = s.advance(ctx, sg, InputRoomReserved, func(ns *Saga) {
		ns.Context.ReservationID = r.ID
		ns.Context.Room.Number = r.RoomNumber
		ns.Context.Room.Type = r.RoomType
		ns.Context.TotalAmount = r.TotalAmount
		ns.Context.Currency = r.Currency
		ns.Context.NightlyRates = slices.Clone(r.NightlyRates)
	})
	if err != nil {
		return nil, err
	}

	bookingID, bookingStatus, err := s.booking.Create(ctx, sg)
	if err != nil {
		// Save nothing: the saga stays at CREATING_BOOKING and the hold stays until the M4a worker releases it
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	sg, err = s.advance(ctx, sg, InputBookingCreated, func(ns *Saga) {
		ns.BookingID = bookingID
	})
	if err != nil {
		return nil, err
	}

	return &Started{
		BookingID:     sg.BookingID,
		BookingStatus: bookingStatus,
		TotalAmount:   sg.Context.TotalAmount,
		Currency:      sg.Context.Currency,
		ExpiresAt:     sg.DeadlineAt,
	}, nil
}

// advance applies one row of the transition table to sg: transition -> next -> fill (copies the input's data into
// the new saga; nil when there is none) -> Save. Returns the stored saga; sg is not modified. ok = false is an error:
// StartBooking made the saga, so it is a bug (M4a's consumer checks transition itself, to log and ack)
func (s *Service) advance(ctx context.Context, sg *Saga, in Input, fill func(ns *Saga)) (*Saga, error) {
	out, ok := transition(sg.Status, sg.Step, in)
	if !ok {
		return nil, fmt.Errorf("saga %s: no transition for %s at %s / %s", sg.ID, in, sg.Status, sg.Step)
	}

	ns := sg.next(out)
	if fill != nil {
		fill(ns)
	}

	stored, err := s.repository.Save(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to save saga %s at %s: %w", sg.ID, ns.Step, err)
	}
	return stored, nil
}
