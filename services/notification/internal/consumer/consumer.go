package consumer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"bookly/contracts/events"
	"bookly/services/notification/internal/service"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	connectAttempts = 30
	connectDelay    = 2 * time.Second
	retryDelay      = time.Second
)

type Handler interface {
	HandleBookingEvent(ctx context.Context, body []byte) error
}

// Consumer reads booking events from a durable queue bound to the exchange.
type Consumer struct {
	url     string
	queue   string
	handler Handler
}

func New(url, queue string, handler Handler) *Consumer {
	return &Consumer{url: url, queue: queue, handler: handler}
}

// Run blocks until ctx is cancelled or the connection to the broker is lost.
func (c *Consumer) Run(ctx context.Context) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	deliveries, err := c.setup(ch)
	if err != nil {
		return err
	}
	log.Printf("Notification Service is listening on queue %q", c.queue)

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("rabbitmq delivery channel closed")
			}
			c.handle(ctx, delivery)
		}
	}
}

func (c *Consumer) dial(ctx context.Context) (*amqp.Connection, error) {
	var err error
	for attempt := 1; attempt <= connectAttempts; attempt++ {
		var conn *amqp.Connection
		if conn, err = amqp.Dial(c.url); err == nil {
			return conn, nil
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

func (c *Consumer) setup(ch *amqp.Channel) (<-chan amqp.Delivery, error) {
	err := ch.ExchangeDeclare(events.Exchange, amqp.ExchangeTopic, true, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	if _, err := ch.QueueDeclare(c.queue, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	if err := ch.QueueBind(c.queue, "booking.*", events.Exchange, false, nil); err != nil {
		return nil, fmt.Errorf("bind queue: %w", err)
	}

	// Take a few messages at a time instead of flooding the consumer.
	if err := ch.Qos(10, 0, false); err != nil {
		return nil, fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := ch.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("start consuming: %w", err)
	}
	return deliveries, nil
}

func (c *Consumer) handle(ctx context.Context, delivery amqp.Delivery) {
	err := c.handler.HandleBookingEvent(ctx, delivery.Body)

	switch {
	case err == nil:
		_ = delivery.Ack(false)
	case errors.Is(err, service.ErrBadMessage):
		// Redelivering a broken message would loop forever, drop it.
		log.Printf("dropping message: %v", err)
		_ = delivery.Reject(false)
	default:
		log.Printf("handling message failed, will retry: %v", err)
		time.Sleep(retryDelay)
		_ = delivery.Nack(false, true)
	}
}
