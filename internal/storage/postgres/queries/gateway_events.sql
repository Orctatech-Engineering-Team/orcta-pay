-- name: InsertGatewayEvent :exec
INSERT INTO gateway_events (id, ref, gateway, status, raw_request, raw_response, created_at)
VALUES ($1, $2, $3, $4, $5, $6, now());

-- name: ListGatewayEvents :many
SELECT id, ref, gateway, status, raw_request, raw_response, created_at FROM gateway_events WHERE ref = $1 ORDER BY created_at;

-- name: ClaimGatewayEventsForDispatch :many
SELECT id, ref, gateway, status, raw_request, raw_response, created_at FROM gateway_events
WHERE status = 'pending' ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED;
