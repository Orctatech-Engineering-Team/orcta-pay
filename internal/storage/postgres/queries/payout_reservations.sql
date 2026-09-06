-- name: CreatePayoutBatch :exec
INSERT INTO payout_batches (id, product, total_pesewas, currency, status, batch_date, idempotency_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now());

-- name: GetPayoutBatch :one
SELECT id, product, total_pesewas, currency, status, batch_date, idempotency_key, created_at
FROM payout_batches WHERE id = $1;

-- name: CreatePayoutReservation :exec
INSERT INTO payout_reservations (id, batch_id, amount_pesewas, currency, status, recipient, reference, gateway, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now());

-- name: SettlePayoutReservation :execrows
UPDATE payout_reservations SET status = 'settled', settled_at = $2 WHERE id = $1 AND status = 'open';

-- name: ReleasePayoutReservation :execrows
UPDATE payout_reservations SET status = 'released' WHERE id = $1 AND status = 'open';

-- name: ListPayoutReservationsByBatch :many
SELECT id, batch_id, amount_pesewas, currency, status, recipient, reference, gateway, settled_at, created_at
FROM payout_reservations WHERE batch_id = $1 ORDER BY created_at;

-- name: GetPayoutReservationByID :one
SELECT id, batch_id, amount_pesewas, currency, status, recipient, reference, gateway, settled_at, created_at
FROM payout_reservations WHERE id = $1;
