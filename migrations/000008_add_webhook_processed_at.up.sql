-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

ALTER TABLE webhook_inbox ADD COLUMN IF NOT EXISTS processed_at TIMESTAMPTZ;
