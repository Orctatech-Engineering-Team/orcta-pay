-- LOCAL so the setting cannot leak into later migrations on this connection.
SET LOCAL lock_timeout = '3s';

CREATE TABLE apps (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL UNIQUE,
    product        TEXT NOT NULL,
    api_key_hash   TEXT NOT NULL UNIQUE,
    api_key_prefix TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at   TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    created_by     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_apps_product ON apps (product);
