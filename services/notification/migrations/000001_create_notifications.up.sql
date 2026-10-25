CREATE TABLE notifications (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- RabbitMQ delivers at least once, so the event id makes handling idempotent.
    event_id TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    message TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX notifications_user_id_idx ON notifications (user_id, created_at DESC);
