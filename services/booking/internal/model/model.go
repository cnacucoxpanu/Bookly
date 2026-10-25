package model

import "time"

const (
	StatusActive    = "active"
	StatusCancelled = "cancelled"
)

type Space struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Capacity    int       `json:"capacity"`
	CreatedAt   time.Time `json:"created_at"`
}

type Booking struct {
	ID        int64     `json:"id"`
	SpaceID   int64     `json:"space_id"`
	SpaceName string    `json:"space_name"`
	UserID    int64     `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
