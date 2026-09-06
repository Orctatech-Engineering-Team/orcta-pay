# Orcta Pay Dashboard

Minimal operator dashboard for Orcta Pay — Charges, Payouts, Ledger, Gateways, Webhooks. Thin views over the same API any Orcta service uses via the TypeScript client.

## Any Orcta service is one client

The dashboard is just one consumer of `clients/ts/orctapay` (`@orctatech/orcta-pay`). Any service (orctago, pos, or a new product) can use the same client by plugging its own per-product API key — provisioned in Vault at `secret/orcta/orcta-pay/keys/{product}` and rendered to `.env` as `ORCTA_PAY_API_KEY`.

```ts
import { OrctaPay } from "@orctatech/orcta-pay";

const pay = new OrctaPay({
  baseUrl: process.env.VITE_ORCTA_PAY_URL, // or ORCTA_PAY_URL
  apiKey: process.env.VITE_ORCTA_PAY_API_KEY!,
});

// Charge — omit idempotency_key to auto-generate optd-{product}-{gateway}-{ulid}
const res = await pay.createCharge({
  product: "orctago",
  amount_pesewas: 1800, // GH₵18.00
  currency: "GHS",
  wallet: "0241234567",
});

const status = await pay.getChargeStatus(res.status !== "failed" ? res.ref : "optd-...");

const batch = await pay.createPayout({
  product: "orctago",
  entries: [{ recipient: "0241111111", amount_pesewas: 5000 }],
});
```

Reference format is `optd-{product}-{gateway}-{ulid}` (Crockford Base32 ULID, same as `internal/gateway/reference.go`). Gateway segment is lower-cased.

## Dashboard config

```bash
cp .env.example .env
# edit VITE_ORCTA_PAY_URL and VITE_ORCTA_PAY_API_KEY
```

| Variable | Description |
|---|---|
| `VITE_ORCTA_PAY_URL` | Orcta Pay base URL (default `http://localhost:8080`) |
| `VITE_ORCTA_PAY_API_KEY` | Per-product Bearer key (Vault `secret/orcta/orcta-pay/keys/{product}`) |
| `VITE_ORCTA_PAY_PRODUCT` | Product hint for display (default `orctago`) |

For demo without `.env`, click **Settings** in the header and paste any product's key — stored in `localStorage` as `orcta_pay_api_key` / `orcta_pay_url` / `orcta_pay_product`. Env is the fallback; localStorage wins.

The header shows a green/red dot by pinging `GET /healthz` (fallback `/readyz`) and the masked active key. When the API is unreachable, every page falls back to mock data but keeps the `OrctaPay.getChargeStatus` wiring live — click a charge to see a typed `OrctaPayError`.

## Pages

- **Charges** — `payment_intents` (ref, product, gateway, amount as GHS, status pending/succeeded/failed, created_at). Search by ref, filter by product/gateway/status. Click row → detail with `GetChargeStatus` and raw gateway event.
- **Payouts** — `payout_batches` (id, batch_date, status running/completed/partially_failed, vendor count, gross/commission/net). Click → per-vendor lines with reservation state `open→settled/released` and age.
- **Ledger** — `vendor_ledger_entries` + `platform_commission_entries` with `value_time`/`booking_time`/`settlement_time` (null until `Verify` or reconciliation confirms). Filter by vendor. Verifies `sum(debits + credits) + net = 0` over the visible slice.
- **Gateways** — Valkey ranking table per `gateway×channel`: rolling success rate, p95 latency, circuit state (closed/open/half_open), cost. Ranking is success-rate floor → cost tiebreak; open circuits excluded. See `PAYMENTS_SERVICE_DESIGN.md:4`.
- **Webhooks** — `webhook_inbox` (aggregator_event_id, kind, payload, received_at, processed_at) with `UNIQUE(aggregator_event_id)` dedup highlight — duplicates return 200 without reprocessing.

## Develop

```bash
npm install
npm run dev      # http://localhost:5173, proxy /v1 and /healthz to VITE_ORCTA_PAY_URL
npm run build    # tsc + vite build
npm run preview  # preview prod build on :5173
npm run lint     # tsc --noEmit
```

`vite.config.ts` proxies `/v1`, `/healthz`, `/readyz` to `VITE_ORCTA_PAY_URL` (or `ORCTA_PAY_URL`) for local dev so the browser avoids CORS.

## Stack

Vite + React 18 + TypeScript (strict) + React Router. Plain CSS — no Tailwind build step. Local path dep `file:../clients/ts/orctapay` → `from "@orctatech/orcta-pay"`.
