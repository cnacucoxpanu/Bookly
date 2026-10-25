package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"bookly/contracts/events"
	"bookly/services/notification/internal/authclient"
	"bookly/services/notification/internal/repository"
)

type fakeRepo struct {
	saved map[string]repository.Notification
	err   error
}

func (f *fakeRepo) Save(_ context.Context, n repository.Notification) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if _, ok := f.saved[n.EventID]; ok {
		return false, nil
	}
	f.saved[n.EventID] = n
	return true, nil
}

type fakeUsers struct {
	err error
}

func (f fakeUsers) GetUser(_ context.Context, id int64) (*authclient.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &authclient.User{ID: id, Name: "Anna", Email: "anna@example.com"}, nil
}

func newTestService(users fakeUsers) (*NotificationService, *fakeRepo) {
	repo := &fakeRepo{saved: map[string]repository.Notification{}}
	return NewNotificationService(repo, users), repo
}

func eventBody(t *testing.T, mutate func(*events.BookingEvent)) []byte {
	t.Helper()

	event := events.BookingEvent{
		EventID:   "evt-1",
		Type:      events.BookingCreated,
		BookingID: 5,
		UserID:    7,
		SpaceID:   1,
		SpaceName: "Quiet Room",
		StartTime: time.Date(2030, 1, 15, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	if mutate != nil {
		mutate(&event)
	}

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestHandleBookingEventStoresMessage(t *testing.T) {
	svc, repo := newTestService(fakeUsers{})

	if err := svc.HandleBookingEvent(context.Background(), eventBody(t, nil)); err != nil {
		t.Fatal(err)
	}

	saved, ok := repo.saved["evt-1"]
	if !ok {
		t.Fatal("notification was not saved")
	}
	for _, part := range []string{"Anna", "anna@example.com", "#5", "Quiet Room", "is confirmed", "2030-01-15 10:00"} {
		if !strings.Contains(saved.Message, part) {
			t.Errorf("message %q does not contain %q", saved.Message, part)
		}
	}
	if saved.UserID != 7 || saved.EventType != events.BookingCreated {
		t.Fatalf("unexpected notification: %+v", saved)
	}
}

func TestHandleBookingEventCancelled(t *testing.T) {
	svc, repo := newTestService(fakeUsers{})

	body := eventBody(t, func(e *events.BookingEvent) { e.Type = events.BookingCancelled })
	if err := svc.HandleBookingEvent(context.Background(), body); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(repo.saved["evt-1"].Message, "was cancelled") {
		t.Fatalf("unexpected message: %s", repo.saved["evt-1"].Message)
	}
}

func TestHandleBookingEventIsIdempotent(t *testing.T) {
	svc, repo := newTestService(fakeUsers{})
	body := eventBody(t, nil)

	for i := 0; i < 3; i++ {
		if err := svc.HandleBookingEvent(context.Background(), body); err != nil {
			t.Fatalf("delivery %d: %v", i+1, err)
		}
	}
	if len(repo.saved) != 1 {
		t.Fatalf("got %d notifications, want 1", len(repo.saved))
	}
}

func TestHandleBookingEventWorksWithoutUserLookup(t *testing.T) {
	svc, repo := newTestService(fakeUsers{err: errors.New("auth is down")})

	if err := svc.HandleBookingEvent(context.Background(), eventBody(t, nil)); err != nil {
		t.Fatalf("a failed lookup must not drop the notification: %v", err)
	}
	if !strings.Contains(repo.saved["evt-1"].Message, "user #7") {
		t.Fatalf("unexpected message: %s", repo.saved["evt-1"].Message)
	}
}

func TestHandleBookingEventBadMessages(t *testing.T) {
	tests := map[string][]byte{
		"not json":     []byte("hello"),
		"no event id":  eventBody(t, func(e *events.BookingEvent) { e.EventID = "" }),
		"no user":      eventBody(t, func(e *events.BookingEvent) { e.UserID = 0 }),
		"unknown type": eventBody(t, func(e *events.BookingEvent) { e.Type = "booking.exploded" }),
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			svc, repo := newTestService(fakeUsers{})

			err := svc.HandleBookingEvent(context.Background(), body)
			if !errors.Is(err, ErrBadMessage) {
				t.Fatalf("got %v, want ErrBadMessage", err)
			}
			if len(repo.saved) != 0 {
				t.Fatal("nothing must be saved")
			}
		})
	}
}

func TestHandleBookingEventStorageErrorIsRetryable(t *testing.T) {
	svc, repo := newTestService(fakeUsers{})
	repo.err = errors.New("database is down")

	err := svc.HandleBookingEvent(context.Background(), eventBody(t, nil))
	if err == nil || errors.Is(err, ErrBadMessage) {
		t.Fatalf("got %v, want a retryable error", err)
	}
}
