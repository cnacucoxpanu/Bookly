package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"bookly/contracts/events"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	connectAttempts = 30
	connectDelay    = 2 * time.Second
)

// Publisher sends booking events to the topic exchange. Messages are
// persistent and the broker confirms every one of them.
type Publisher struct {
	url string

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewPublisher connects to RabbitMQ. The broker may still be starting when the
// service boots, so the connection is retried for a while.
func NewPublisher(ctx context.Context, url string) (*Publisher, error) {
	p := &Publisher{url: url}

	var err error
	for attempt := 1; attempt <= connectAttempts; attempt++ {
		if err = p.connect(); err == nil {
			return p, nil
		}
		log.Printf("RabbitMQ is not ready (attempt %d/%d): %v", attempt, connectAttempts, err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(connectDelay):
		}
	}

	return nil, fmt.Errorf("connect to rabbitmq: %w", err)
}

func (p *Publisher) connect() error {
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	err = ch.ExchangeDeclare(events.Exchange, amqp.ExchangeTopic, true, false, false, false, nil)
	if err == nil {
		err = ch.Confirm(false)
	}
	if err != nil {
		_ = conn.Close()
		return err
	}

	p.conn, p.ch = conn, ch
	return nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, event events.BookingEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil || p.conn.IsClosed() {
		if err := p.connect(); err != nil {
			return fmt.Errorf("reconnect to rabbitmq: %w", err)
		}
	}

	confirmation, err := p.ch.PublishWithDeferredConfirmWithContext(
		ctx,
		events.Exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    event.EventID,
			Timestamp:    event.OccurredAt,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for broker confirmation: %w", err)
	}
	if !acked {
		return errors.New("broker rejected the message")
	}
	return nil
}

func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil || p.conn.IsClosed() {
		return nil
	}
	return p.conn.Close()
}
