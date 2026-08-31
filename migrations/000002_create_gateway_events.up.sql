-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

CREATE TABLE gateway_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ref         TEXT NOT NULL REFERENCES payment_intents(ref),
    gateway     TEXT NOT NULL CHECK (gateway IN ('hubtel','paystack')),
    status      TEXT NOT NULL CHECK (status IN ('pending','succeeded','failed')),
    raw_request  JSONB,
    raw_response JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_gateway_events_ref ON gateway_events (ref);
CREATE INDEX idx_gateway_events_pending ON gateway_events (created_at) WHERE status = 'pending';
