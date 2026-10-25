package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"bookly/contracts/events"
	"bookly/services/booking/internal/model"
	"bookly/services/booking/internal/repository"
)

var now = time.Date(2030, 1, 15, 9, 0, 0, 0, time.UTC)

type fakeSpaces struct {
	spaces map[int64]*model.Space
}

func (f *fakeSpaces) Create(_ context.Context, space *model.Space) error {
	for _, s := range f.spaces {
		if s.Name == space.Name {
			return repository.ErrSpaceExists
		}
	}
	space.ID = int64(len(f.spaces) + 1)
	f.spaces[space.ID] = space
	return nil
}

func (f *fakeSpaces) List(context.Context) ([]model.Space, error) {
	list := []model.Space{}
	for _, s := range f.spaces {
		list = append(list, *s)
	}
	return list, nil
}

func (f *fakeSpaces) GetByID(_ context.Context, id int64) (*model.Space, error) {
	space, ok := f.spaces[id]
	if !ok {
		return nil, repository.ErrSpaceNotFound
	}
	return space, nil
}

type fakeBookings struct {
	bookings map[int64]*model.Booking
}

func (f *fakeBookings) Create(_ context.Context, b *model.Booking) error {
	for _, other := range f.bookings {
		overlaps := other.SpaceID == b.SpaceID &&
			other.Status == model.StatusActive &&
			other.StartTime.Before(b.EndTime) &&
			other.EndTime.After(b.StartTime)
		if overlaps {
			return repository.ErrSlotTaken
		}
	}
	b.ID = int64(len(f.bookings) + 1)
	b.Status = model.StatusActive
	stored := *b
	f.bookings[b.ID] = &stored
	return nil
}

func (f *fakeBookings) GetByID(_ context.Context, id int64) (*model.Booking, error) {
	b, ok := f.bookings[id]
	if !ok {
		return nil, repository.ErrBookingNotFound
	}
	copied := *b
	return &copied, nil
}

func (f *fakeBookings) ListByUser(_ context.Context, userID int64) ([]model.Booking, error) {
	list := []model.Booking{}
	for _, b := range f.bookings {
		if b.UserID == userID {
			list = append(list, *b)
		}
	}
	return list, nil
}

func (f *fakeBookings) Cancel(_ context.Context, id int64) error {
	b, ok := f.bookings[id]
	if !ok || b.Status != model.StatusActive {
		return repository.ErrBookingNotActive
	}
	b.Status = model.StatusCancelled
	return nil
}

type fakePublisher struct {
	published []events.BookingEvent
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, routingKey string, event events.BookingEvent) error {
	if f.err != nil {
		return f.err
	}
	if routingKey != event.Type {
		return errors.New("routing key must equal event type")
	}
	f.published = append(f.published, event)
	return nil
}

func newTestBookingService() (*BookingService, *fakePublisher) {
	spaces := &fakeSpaces{spaces: map[int64]*model.Space{
		1: {ID: 1, Name: "Quiet Room", Capacity: 4},
	}}
	bookings := &fakeBookings{bookings: map[int64]*model.Booking{}}
	publisher := &fakePublisher{}

	svc := NewBookingService(bookings, spaces, publisher)
	svc.now = func() time.Time { return now }

	return svc, publisher
}

func TestCreateBookingValidatesInput(t *testing.T) {
	tests := []struct {
		name    string
		spaceID int64
		start   time.Time
		end     time.Time
	}{
		{"no space", 0, now.Add(time.Hour), now.Add(2 * time.Hour)},
		{"no times", 1, time.Time{}, time.Time{}},
		{"end before start", 1, now.Add(2 * time.Hour), now.Add(time.Hour)},
		{"zero length", 1, now.Add(time.Hour), now.Add(time.Hour)},
		{"starts in the past", 1, now.Add(-time.Hour), now.Add(time.Hour)},
		{"longer than 12 hours", 1, now.Add(time.Hour), now.Add(14 * time.Hour)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, publisher := newTestBookingService()

			_, err := svc.Create(context.Background(), 7, tt.spaceID, tt.start, tt.end)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("got %v, want ErrInvalidInput", err)
			}
			if len(publisher.published) != 0 {
				t.Fatal("no event must be published")
			}
		})
	}
}

func TestCreateBookingPublishesEvent(t *testing.T) {
	svc, publisher := newTestBookingService()
	start, end := now.Add(time.Hour), now.Add(3*time.Hour)

	booking, err := svc.Create(context.Background(), 7, 1, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if booking.ID == 0 || booking.Status != model.StatusActive || booking.SpaceName != "Quiet Room" {
		t.Fatalf("unexpected booking: %+v", booking)
	}

	if len(publisher.published) != 1 {
		t.Fatalf("got %d events, want 1", len(publisher.published))
	}
	event := publisher.published[0]
	if event.Type != events.BookingCreated || event.BookingID != booking.ID ||
		event.UserID != 7 || event.SpaceName != "Quiet Room" || event.EventID == "" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestCreateBookingConflicts(t *testing.T) {
	svc, publisher := newTestBookingService()
	ctx := context.Background()
	start, end := now.Add(time.Hour), now.Add(3*time.Hour)

	if _, err := svc.Create(ctx, 7, 1, start, end); err != nil {
		t.Fatal(err)
	}

	overlapping := []struct {
		name       string
		start, end time.Time
	}{
		{"same period", start, end},
		{"inside", start.Add(30 * time.Minute), end.Add(-30 * time.Minute)},
		{"overlaps the start", start.Add(-time.Hour), start.Add(time.Hour)},
		{"overlaps the end", end.Add(-time.Hour), end.Add(time.Hour)},
	}
	for _, tt := range overlapping {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Create(ctx, 8, 1, tt.start, tt.end); !errors.Is(err, ErrSlotTaken) {
				t.Fatalf("got %v, want ErrSlotTaken", err)
			}
		})
	}

	t.Run("back to back is fine", func(t *testing.T) {
		if _, err := svc.Create(ctx, 8, 1, end, end.Add(time.Hour)); err != nil {
			t.Fatalf("a booking that starts when the other ends must be allowed: %v", err)
		}
	})

	// Only the two successful bookings may produce events.
	if len(publisher.published) != 2 {
		t.Fatalf("got %d events, want 2", len(publisher.published))
	}
}

func TestCreateBookingUnknownSpace(t *testing.T) {
	svc, _ := newTestBookingService()

	_, err := svc.Create(context.Background(), 7, 99, now.Add(time.Hour), now.Add(2*time.Hour))
	if !errors.Is(err, ErrSpaceNotFound) {
		t.Fatalf("got %v, want ErrSpaceNotFound", err)
	}
}

func TestCreateBookingSurvivesBrokenPublisher(t *testing.T) {
	svc, publisher := newTestBookingService()
	publisher.err = errors.New("broker is down")

	_, err := svc.Create(context.Background(), 7, 1, now.Add(time.Hour), now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("a saved booking must not fail because of the broker: %v", err)
	}
}

func TestCancelBooking(t *testing.T) {
	ctx := context.Background()

	create := func(t *testing.T) (*BookingService, *fakePublisher, *model.Booking) {
		svc, publisher := newTestBookingService()
		booking, err := svc.Create(ctx, 7, 1, now.Add(time.Hour), now.Add(2*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		return svc, publisher, booking
	}

	t.Run("owner", func(t *testing.T) {
		svc, publisher, booking := create(t)

		cancelled, err := svc.Cancel(ctx, 7, "user", booking.ID)
		if err != nil {
			t.Fatal(err)
		}
		if cancelled.Status != model.StatusCancelled {
			t.Fatalf("status is %s", cancelled.Status)
		}

		last := publisher.published[len(publisher.published)-1]
		if last.Type != events.BookingCancelled || last.BookingID != booking.ID {
			t.Fatalf("unexpected event: %+v", last)
		}
	})

	t.Run("admin", func(t *testing.T) {
		svc, _, booking := create(t)

		if _, err := svc.Cancel(ctx, 1, RoleAdmin, booking.ID); err != nil {
			t.Fatalf("admin must be able to cancel: %v", err)
		}
	})

	t.Run("someone else", func(t *testing.T) {
		svc, publisher, booking := create(t)

		if _, err := svc.Cancel(ctx, 8, "user", booking.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
		if len(publisher.published) != 1 {
			t.Fatal("a refused cancel must not publish an event")
		}
	})

	t.Run("twice", func(t *testing.T) {
		svc, _, booking := create(t)

		if _, err := svc.Cancel(ctx, 7, "user", booking.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Cancel(ctx, 7, "user", booking.ID); !errors.Is(err, ErrAlreadyCancelled) {
			t.Fatalf("got %v, want ErrAlreadyCancelled", err)
		}
	})

	t.Run("unknown booking", func(t *testing.T) {
		svc, _, _ := create(t)

		if _, err := svc.Cancel(ctx, 7, "user", 999); !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("got %v, want ErrBookingNotFound", err)
		}
	})

	t.Run("freed slot can be booked again", func(t *testing.T) {
		svc, _, booking := create(t)

		if _, err := svc.Cancel(ctx, 7, "user", booking.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Create(ctx, 8, 1, booking.StartTime, booking.EndTime); err != nil {
			t.Fatalf("cancelled booking must free the slot: %v", err)
		}
	})
}

func TestGetBookingAccess(t *testing.T) {
	svc, _ := newTestBookingService()
	ctx := context.Background()

	booking, err := svc.Create(ctx, 7, 1, now.Add(time.Hour), now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Get(ctx, 7, "user", booking.ID); err != nil {
		t.Fatalf("owner: %v", err)
	}
	if _, err := svc.Get(ctx, 1, RoleAdmin, booking.ID); err != nil {
		t.Fatalf("admin: %v", err)
	}
	if _, err := svc.Get(ctx, 8, "user", booking.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("stranger got %v, want ErrForbidden", err)
	}
}

func TestCreateSpace(t *testing.T) {
	newService := func() *SpaceService {
		return NewSpaceService(&fakeSpaces{spaces: map[int64]*model.Space{}})
	}
	ctx := context.Background()

	t.Run("admin", func(t *testing.T) {
		space, err := newService().Create(ctx, RoleAdmin, " Phone Booth ", "", 1)
		if err != nil || space.Name != "Phone Booth" {
			t.Fatalf("got %+v, %v", space, err)
		}
	})

	t.Run("regular user", func(t *testing.T) {
		if _, err := newService().Create(ctx, "user", "Phone Booth", "", 1); !errors.Is(err, ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		svc := newService()
		if _, err := svc.Create(ctx, RoleAdmin, "  ", "", 1); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("empty name: got %v", err)
		}
		if _, err := svc.Create(ctx, RoleAdmin, "Room", "", 0); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("zero capacity: got %v", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		svc := newService()
		if _, err := svc.Create(ctx, RoleAdmin, "Room", "", 2); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Create(ctx, RoleAdmin, "Room", "", 2); !errors.Is(err, ErrSpaceExists) {
			t.Fatalf("got %v, want ErrSpaceExists", err)
		}
	})
}
