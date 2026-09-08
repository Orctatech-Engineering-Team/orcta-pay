-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

ALTER TABLE payment_intents ADD COLUMN IF NOT EXISTS authorization_url TEXT;
