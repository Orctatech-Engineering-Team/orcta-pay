-- name: InsertLedgerEntry :exec
INSERT INTO ledger_entries (id, kind, ref, amount_pesewas, currency, value_time, booking_time, settlement_time, product, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now());

-- name: ListLedgerEntriesByRef :many
SELECT id, kind, ref, amount_pesewas, currency, value_time, booking_time, settlement_time, product, created_at
FROM ledger_entries WHERE ref = $1 ORDER BY booking_time;

-- name: StampLedgerSettlement :execrows
UPDATE ledger_entries SET settlement_time = $2
WHERE ref = $1 AND settlement_time IS NULL;

-- name: UpdateLedgerEntrySettlement :execrows
UPDATE ledger_entries SET settlement_time = $3 WHERE id = $1 AND settlement_time IS NULL AND $2::text IS NOT NULL;
