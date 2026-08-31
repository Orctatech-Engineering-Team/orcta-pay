-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

CREATE TABLE payout_batches (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product        TEXT NOT NULL,
    total_pesewas  BIGINT NOT NULL CHECK (total_pesewas > 0),
    currency       TEXT NOT NULL DEFAULT 'GHS',
    status         TEXT NOT NULL CHECK (status IN ('pending','processing','completed','failed')) DEFAULT 'pending',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payout_reservations (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id       UUID NOT NULL REFERENCES payout_batches(id),
    amount_pesewas BIGINT NOT NULL CHECK (amount_pesewas > 0),
    currency       TEXT NOT NULL DEFAULT 'GHS',
    status         TEXT NOT NULL CHECK (status IN ('open','settled','released')) DEFAULT 'open',
    settled_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payout_reservations_batch ON payout_reservations (batch_id);
CREATE INDEX idx_payout_reservations_open ON payout_reservations (created_at) WHERE status = 'open';
