-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

CREATE TABLE ledger_entries (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind             TEXT NOT NULL CHECK (kind IN ('collection','platform_commission','vendor_payout')),
    ref              TEXT NOT NULL,
    amount_pesewas   BIGINT NOT NULL,
    currency         TEXT NOT NULL CHECK (currency = 'GHS'),
    value_time       TIMESTAMPTZ NOT NULL,
    booking_time     TIMESTAMPTZ NOT NULL,
    settlement_time  TIMESTAMPTZ,
    product          TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ledger_entries_ref ON ledger_entries (ref);
CREATE INDEX idx_ledger_entries_product_booking ON ledger_entries (product, booking_time);

CREATE TABLE vendor_ledger_entries (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id        UUID,
    ref              TEXT NOT NULL,
    amount_pesewas   BIGINT NOT NULL,
    currency         TEXT NOT NULL DEFAULT 'GHS',
    value_time       TIMESTAMPTZ NOT NULL,
    booking_time     TIMESTAMPTZ NOT NULL,
    settlement_time  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE platform_commission_entries (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ref              TEXT NOT NULL,
    amount_pesewas   BIGINT NOT NULL,
    currency         TEXT NOT NULL DEFAULT 'GHS',
    value_time       TIMESTAMPTZ NOT NULL,
    booking_time     TIMESTAMPTZ NOT NULL,
    settlement_time  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
