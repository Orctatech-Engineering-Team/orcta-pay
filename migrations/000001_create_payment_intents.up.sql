-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

CREATE TABLE payment_intents (
    ref              TEXT PRIMARY KEY,
    product          TEXT NOT NULL,
    gateway          TEXT NOT NULL CHECK (gateway IN ('hubtel','paystack')),
    amount_pesewas   BIGINT NOT NULL CHECK (amount_pesewas > 0),
    currency         TEXT NOT NULL CHECK (currency = 'GHS'),
    wallet           TEXT,
    idempotency_key  TEXT,
    status           TEXT NOT NULL CHECK (status IN ('pending','succeeded','failed')) DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product, idempotency_key)
);
CREATE INDEX idx_payment_intents_product_created ON payment_intents (product, created_at);
