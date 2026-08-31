-- name: CreatePayoutReservation :exec
INSERT INTO payout_reservations (id, batch_id, amount_pesewas, currency, status, created_at)
VALUES ($1, $2, $3, $4, $5, now());

-- name: SettlePayoutReservation :exec
UPDATE payout_reservations SET status = 'settled', settled_at = $2 WHERE id = $1 AND status = 'open';

-- name: ReleasePayoutReservation :exec
UPDATE payout_reservations SET status = 'released' WHERE id = $1 AND status = 'open';

-- name: CreatePayoutBatch :exec
INSERT INTO payout_batches (id, product, total_pesewas, currency, status, created_at)
VALUES ($1, $2, $3, $4, $5, now());
