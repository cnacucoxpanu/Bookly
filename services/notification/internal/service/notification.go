package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"bookly/contracts/events"
	"bookly/services/notification/internal/authclient"
	"bookly/services/notification/internal/repository"
)

// ErrBadMessage means the message can never be processed, so there is no point
// in redelivering it.
var ErrBadMessage = errors.New("bad message")

const timeLayout = "2006-01-02 15:04"

type NotificationRepository interface {
	Save(ctx context.Context, n repository.Notification) (bool, error)
}

type UserLookup interface {
	GetUser(ctx context.Context, id int64) (*authclient.User, error)
}

type NotificationService struct {
	notifications NotificationRepository
	users         UserLookup
}

func NewNotificationService(notifications NotificationRepository, users UserLookup) *NotificationService {
	return &NotificationService{notifications: notifications, users: users}
}

// HandleBookingEvent turns a booking event into a stored notification.
func (s *NotificationService) HandleBookingEvent(ctx context.Context, body []byte) error {
	var event events.BookingEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("%w: %v", ErrBadMessage, err)
	}
	if event.EventID == "" || event.UserID == 0 {
		return fmt.Errorf("%w: event_id and user_id are required", ErrBadMessage)
	}

	message, err := s.buildMessage(ctx, event)
	if err != nil {
		return err
	}

	saved, err := s.notifications.Save(ctx, repository.Notification{
		EventID:   event.EventID,
		EventType: event.Type,
		UserID:    event.UserID,
		Message:   message,
		Payload:   body,
	})
	if err != nil {
		return err
	}
	if !saved {
		log.Printf("event %s was already handled, skipping", event.EventID)
		return nil
	}

	log.Printf("notification for user %d: %s", event.UserID, message)
	return nil
}

func (s *NotificationService) buildMessage(ctx context.Context, event events.BookingEvent) (string, error) {
	var action string
	switch event.Type {
	case events.BookingCreated:
		action = "is confirmed"
	case events.BookingCancelled:
		action = "was cancelled"
	default:
		return "", fmt.Errorf("%w: unknown event type %q", ErrBadMessage, event.Type)
	}

	// The name only makes the text friendlier, so a failed lookup must not
	// stop the notification from being stored.
	who := fmt.Sprintf("user #%d", event.UserID)
	if user, err := s.users.GetUser(ctx, event.UserID); err != nil {
		log.Printf("could not load user %d: %v", event.UserID, err)
	} else {
		who = fmt.Sprintf("%s (%s)", user.Name, user.Email)
	}

	return fmt.Sprintf(
		"%s: booking #%d of %q %s, %s - %s UTC",
		who,
		event.BookingID,
		event.SpaceName,
		action,
		event.StartTime.UTC().Format(timeLayout),
		event.EndTime.UTC().Format(timeLayout),
	), nil
}
