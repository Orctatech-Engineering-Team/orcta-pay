-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

-- Allow moolre gateway in existing tables.
ALTER TABLE payment_intents DROP CONSTRAINT payment_intents_gateway_check;
ALTER TABLE payment_intents ADD CONSTRAINT payment_intents_gateway_check CHECK (gateway IN ('hubtel','paystack','moolre'));

ALTER TABLE gateway_events DROP CONSTRAINT gateway_events_gateway_check;
ALTER TABLE gateway_events ADD CONSTRAINT gateway_events_gateway_check CHECK (gateway IN ('hubtel','paystack','moolre'));

ALTER TABLE webhook_inbox DROP CONSTRAINT webhook_inbox_gateway_check;
ALTER TABLE webhook_inbox ADD CONSTRAINT webhook_inbox_gateway_check CHECK (gateway IN ('hubtel','paystack','moolre'));

-- Payout batch idempotency scoped to (vendor_id, batch_date) per payments-design §7.
-- batch_date is the business date of the batch; idempotency_key is product:batch_date or vendor:batch_date.
ALTER TABLE payout_batches ADD COLUMN batch_date DATE NOT NULL DEFAULT CURRENT_DATE;
ALTER TABLE payout_batches ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX idx_payout_batches_product_batch_date ON payout_batches (product, batch_date, idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Vendor-scoped reservation: recipient is vendor phone / momo, vendor_id optional for future FK.
ALTER TABLE payout_reservations ADD COLUMN recipient TEXT;
ALTER TABLE payout_reservations ADD COLUMN reference TEXT;
ALTER TABLE payout_reservations ADD COLUMN gateway TEXT CHECK (gateway IN ('hubtel','paystack','moolre'));
