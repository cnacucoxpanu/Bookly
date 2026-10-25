// Package events describes the messages services publish to RabbitMQ.
package events

import "time"

const (
	// Exchange is the topic exchange all booking events go through.
	Exchange = "bookly.events"

	BookingCreated   = "booking.created"
	BookingCancelled = "booking.cancelled"
)

// BookingEvent is the payload of booking.created and booking.cancelled.
// The routing key of the message equals Type.
type BookingEvent struct {
	EventID    string    `json:"event_id"`
	Type       string    `json:"type"`
	BookingID  int64     `json:"booking_id"`
	UserID     int64     `json:"user_id"`
	SpaceID    int64     `json:"space_id"`
	SpaceName  string    `json:"space_name"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	OccurredAt time.Time `json:"occurred_at"`
}
