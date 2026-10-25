package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"bookly/contracts/events"
	"bookly/services/booking/internal/model"
	"bookly/services/booking/internal/repository"
)

const maxBookingDuration = 12 * time.Hour

type BookingRepository interface {
	Create(ctx context.Context, booking *model.Booking) error
	GetByID(ctx context.Context, id int64) (*model.Booking, error)
	ListByUser(ctx context.Context, userID int64) ([]model.Booking, error)
	Cancel(ctx context.Context, id int64) error
}

type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, event events.BookingEvent) error
}

type BookingService struct {
	bookings BookingRepository
	spaces   SpaceRepository
	events   EventPublisher
	now      func() time.Time
}

func NewBookingService(
	bookings BookingRepository,
	spaces SpaceRepository,
	events EventPublisher,
) *BookingService {
	return &BookingService{
		bookings: bookings,
		spaces:   spaces,
		events:   events,
		now:      time.Now,
	}
}

func (s *BookingService) Create(
	ctx context.Context,
	userID int64,
	spaceID int64,
	start time.Time,
	end time.Time,
) (*model.Booking, error) {
	start, end = start.UTC(), end.UTC()

	if spaceID <= 0 {
		return nil, fmt.Errorf("%w: space_id is required", ErrInvalidInput)
	}
	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("%w: start_time and end_time are required", ErrInvalidInput)
	}
	if !end.After(start) {
		return nil, fmt.Errorf("%w: end_time must be after start_time", ErrInvalidInput)
	}
	if start.Before(s.now()) {
		return nil, fmt.Errorf("%w: start_time must be in the future", ErrInvalidInput)
	}
	if end.Sub(start) > maxBookingDuration {
		return nil, fmt.Errorf("%w: a booking can last at most %s", ErrInvalidInput, maxBookingDuration)
	}

	space, err := s.spaces.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, repository.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}

	booking := &model.Booking{
		SpaceID:   space.ID,
		SpaceName: space.Name,
		UserID:    userID,
		StartTime: start,
		EndTime:   end,
	}

	if err := s.bookings.Create(ctx, booking); err != nil {
		switch {
		case errors.Is(err, repository.ErrSlotTaken):
			return nil, ErrSlotTaken
		case errors.Is(err, repository.ErrSpaceNotFound):
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}

	s.publish(ctx, events.BookingCreated, booking)

	return booking, nil
}

func (s *BookingService) ListMine(ctx context.Context, userID int64) ([]model.Booking, error) {
	return s.bookings.ListByUser(ctx, userID)
}

func (s *BookingService) Get(
	ctx context.Context,
	userID int64,
	role string,
	id int64,
) (*model.Booking, error) {
	booking, err := s.find(ctx, id)
	if err != nil {
		return nil, err
	}
	if booking.UserID != userID && role != RoleAdmin {
		return nil, ErrForbidden
	}
	return booking, nil
}

func (s *BookingService) Cancel(
	ctx context.Context,
	userID int64,
	role string,
	id int64,
) (*model.Booking, error) {
	booking, err := s.Get(ctx, userID, role, id)
	if err != nil {
		return nil, err
	}
	if booking.Status != model.StatusActive {
		return nil, ErrAlreadyCancelled
	}

	if err := s.bookings.Cancel(ctx, id); err != nil {
		if errors.Is(err, repository.ErrBookingNotActive) {
			return nil, ErrAlreadyCancelled
		}
		return nil, err
	}
	booking.Status = model.StatusCancelled

	s.publish(ctx, events.BookingCancelled, booking)

	return booking, nil
}

func (s *BookingService) find(ctx context.Context, id int64) (*model.Booking, error) {
	booking, err := s.bookings.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}
	return booking, nil
}

// publish sends the event after the booking is already saved. A broken broker
// must not turn a successful booking into an error, so failures are only logged.
func (s *BookingService) publish(ctx context.Context, eventType string, booking *model.Booking) {
	event := events.BookingEvent{
		EventID:    newEventID(),
		Type:       eventType,
		BookingID:  booking.ID,
		UserID:     booking.UserID,
		SpaceID:    booking.SpaceID,
		SpaceName:  booking.SpaceName,
		StartTime:  booking.StartTime,
		EndTime:    booking.EndTime,
		OccurredAt: s.now().UTC(),
	}

	if err := s.events.Publish(ctx, eventType, event); err != nil {
		log.Printf("publish %s for booking %d: %v", eventType, booking.ID, err)
	}
}

func newEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
