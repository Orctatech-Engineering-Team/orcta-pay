-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

ALTER TABLE payment_intents DROP COLUMN IF EXISTS authorization_url;
