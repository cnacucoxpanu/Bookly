package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Notification struct {
	EventID   string
	EventType string
	UserID    int64
	Message   string
	Payload   []byte
}

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Save stores the notification. It reports false when a notification for the
// same event already exists, which happens when RabbitMQ redelivers a message.
func (r *NotificationRepository) Save(ctx context.Context, n Notification) (bool, error) {
	tag, err := r.db.Exec(ctx, `
	INSERT INTO notifications (event_id, event_type, user_id, message, payload)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (event_id) DO NOTHING
	`, n.EventID, n.EventType, n.UserID, n.Message, n.Payload)
	if err != nil {
		return false, fmt.Errorf("save notification: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}
