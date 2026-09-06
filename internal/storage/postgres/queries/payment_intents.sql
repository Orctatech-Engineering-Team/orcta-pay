-- name: CreatePaymentIntent :exec
INSERT INTO payment_intents (ref, product, gateway, amount_pesewas, currency, wallet, idempotency_key, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now());

-- name: GetPaymentIntentByRef :one
SELECT ref, product, gateway, amount_pesewas, currency, wallet, idempotency_key, status, created_at
FROM payment_intents WHERE ref = $1;

-- name: GetPaymentIntentByIdempotencyKey :one
SELECT ref FROM payment_intents WHERE product = $1 AND idempotency_key = $2;

-- name: UpdatePaymentIntentStatus :exec
UPDATE payment_intents SET status = $2 WHERE ref = $1;

-- name: GetPaymentIntentStatus :one
SELECT status FROM payment_intents WHERE ref = $1;

-- name: GetPaymentIntentForWebhook :one
SELECT ref, gateway, status, product FROM payment_intents WHERE ref = $1;
