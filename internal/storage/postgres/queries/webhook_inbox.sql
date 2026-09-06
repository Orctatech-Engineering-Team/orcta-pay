-- name: InsertWebhookInbox :exec
INSERT INTO webhook_inbox (aggregator_event_id, gateway, payload, received_at)
VALUES ($1, $2, $3, now());

-- name: ExistsWebhookInbox :one
SELECT EXISTS(SELECT 1 FROM webhook_inbox WHERE aggregator_event_id = $1);

-- name: InsertWebhookInboxDedup :execrows
INSERT INTO webhook_inbox (aggregator_event_id, gateway, payload, received_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (aggregator_event_id) DO NOTHING;

-- name: DeleteWebhookInbox :execrows
DELETE FROM webhook_inbox WHERE aggregator_event_id = $1;
