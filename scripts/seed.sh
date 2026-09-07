#!/usr/bin/env bash
# Seed demo data for local dev — apps, charges, ledger, payouts, webhooks.
# Usage: bash scripts/seed.sh  (or task dev:seed)
# Requires: postgres at DATABASE_URL (or postgres://orcta:orcta@localhost:5432/orcta_pay), API at http://localhost:8080
set -euo pipefail

API=${API_URL:-http://localhost:8080}
DB_URL=${DATABASE_URL:-postgres://orcta:orcta@localhost:5432/orcta_pay?sslmode=disable}

echo "== creating apps via API $API =="
curl -s -X POST "$API/v1/apps" -H "Content-Type: application/json" -d '{"name":"orctago","product":"orctago"}' | cat; echo
curl -s -X POST "$API/v1/apps" -H "Content-Type: application/json" -d '{"name":"orcta-pos","product":"pos"}' | cat; echo
echo "apps: $(curl -s "$API/v1/apps" | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")"

echo "== seeding charges/ledger/payouts/webhooks via psql =="
python3 <<'PY' > /tmp/seed_demo.sql
import random, uuid
def ulid():
    chars="0123456789ABCDEFGHJKMNPQRSTVWXYZ"
    return ''.join(random.choice(chars) for _ in range(26))
def ref(p,g):
    return f"optd-{p}-{g}-{ulid()}"
charges=[
    ("orctago","paystack",1800,"succeeded","233501234567"),
    ("orctago","hubtel",2500,"succeeded","233501234568"),
    ("orctago","paystack",5000,"pending","233501234569"),
    ("orctago","moolre",1200,"failed","233501234570"),
    ("pos","paystack",7500,"succeeded","233501234571"),
    ("pos","hubtel",3000,"pending","233501234572"),
]
refs=[]
for p,g,amt,st,w in charges:
    r=ref(p,g)
    refs.append((r,p,g,amt,st))
    print(f"INSERT INTO payment_intents (ref, product, gateway, amount_pesewas, currency, wallet, idempotency_key, status) VALUES ('{r}','{p}','{g}',{amt},'GHS','{w}','seed-{r[-6:]}','{st}') ON CONFLICT (ref) DO NOTHING;")
for r,p,g,amt,st in refs:
    if st=="succeeded":
        print(f"INSERT INTO ledger_entries (kind, ref, amount_pesewas, currency, value_time, booking_time, settlement_time, product) VALUES ('collection','{r}',{amt},'GHS',now()-interval'2 hours',now()-interval'1 hour',now(),'{p}');")
        print(f"INSERT INTO ledger_entries (kind, ref, amount_pesewas, currency, value_time, booking_time, product) VALUES ('platform_commission','{r}',{amt//10},'GHS',now()-interval'1 hour',now(),'{p}');")
bid=str(uuid.uuid4())
print(f"INSERT INTO payout_batches (id, product, total_pesewas, currency, status, batch_date) VALUES ('{bid}','orctago',10000,'GHS','completed',CURRENT_DATE) ON CONFLICT (id) DO NOTHING;")
for recipient, amt in [("233551112233",4000),("233551112234",3500),("233551112235",2500)]:
    rid=str(uuid.uuid4())
    print(f"INSERT INTO payout_reservations (id, batch_id, amount_pesewas, currency, status, recipient, reference) VALUES ('{rid}','{bid}',{amt},'GHS','settled','{recipient}','payout-ref-{recipient[-4:]}') ON CONFLICT (id) DO NOTHING;")
for gw in ["paystack","hubtel"]:
    agg=f"evt_{uuid.uuid4().hex[:16]}"
    payload=f'{{"event":"charge.succeeded","gateway":"{gw}","ref":"{refs[0][0]}"}}'.replace("'","''")
    print(f"INSERT INTO webhook_inbox (aggregator_event_id, gateway, payload) VALUES ('{agg}','{gw}','{payload}'::jsonb) ON CONFLICT (aggregator_event_id) DO NOTHING;")
PY
psql "$DB_URL" -f /tmp/seed_demo.sql
echo "counts: charges=$(psql "$DB_URL" -t -c "SELECT count(*) FROM payment_intents") ledger=$(psql "$DB_URL" -t -c "SELECT count(*) FROM ledger_entries") payouts=$(psql "$DB_URL" -t -c "SELECT count(*) FROM payout_batches") webhooks=$(psql "$DB_URL" -t -c "SELECT count(*) FROM webhook_inbox")"
echo "== done — verify: curl $API/v1/charges | jq =="
