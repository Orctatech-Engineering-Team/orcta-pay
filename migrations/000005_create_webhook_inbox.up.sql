-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

CREATE TABLE webhook_inbox (
    aggregator_event_id TEXT PRIMARY KEY,
    gateway             TEXT NOT NULL CHECK (gateway IN ('hubtel','paystack')),
    payload             JSONB NOT NULL,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
