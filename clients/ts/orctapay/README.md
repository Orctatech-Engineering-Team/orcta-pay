# Orcta Pay TypeScript SDK

Thin TypeScript client for the Orcta Pay API (`api/openapi.yaml`). Mirrors `clients/go/orctapay`.

- **Zero runtime dependencies** — native `fetch` + `AbortController`
- **Runtime-agnostic** — Node 18+, Bun, Deno, browsers
- **Typed errors** — throws `OrctaPayError` with `statusCode` / `code`
- **Dual-published** — npm (`@orctatech/orcta-pay`) + JSR (`@orctatech/orcta-pay`)

See `orcta-go-docs/PAYMENTS_SERVICE_DESIGN.md` for reference and idempotency rules. Amounts are whole pesewas in GHS (integer).

## Install

```bash
npm install @orctatech/orcta-pay
# or
npx jsr add @orctatech/orcta-pay
# or
deno add jsr:@orctatech/orcta-pay
```

## Auth

Each product gets its own API key. Provisioned in Vault at `secret/orcta/orcta-pay/keys/{product}` and rendered to `.env` as `ORCTA_PAY_API_KEY`.

```bash
ORCTA_PAY_URL=https://api.pay.orctatech.com
ORCTA_PAY_API_KEY=pay_live_...   # per product: ORCTA_PAY_API_KEY_GO, ORCTA_PAY_API_KEY_POS
```

The client sends `Authorization: Bearer <apiKey>`.

## Usage

```ts
import { OrctaPay } from "@orctatech/orcta-pay";

const pay = new OrctaPay({
  baseUrl: process.env.ORCTA_PAY_URL, // defaults to http://localhost:8080 or ORCTA_PAY_URL env
  apiKey: process.env.ORCTA_PAY_API_KEY!,
  timeout: 10_000, // ms, default 10s like Go
});

// Charge — omit idempotency_key to auto-generate optd-{product}-{gateway}-{ulid}
const res = await pay.createCharge({
  product: "orctago",
  amount_pesewas: 1800, // GH₵18.00
  currency: "GHS",
  wallet: "0241234567",
});

if (res.status === "pending") {
  // res.ref is optd-orctago-hubtel-...
  console.log(res.ref, res.gateway);
} else if (res.status === "succeeded") {
  console.log(res.amount_pesewas);
} else {
  console.error(res.reason);
}

// Authoritative status (calls gateway GetTransactionStatus)
const status = await pay.getChargeStatus(res.status !== "failed" ? res.ref : "optd-...");

// Payout batch
const batch = await pay.createPayout({
  product: "orctago",
  entries: [
    { recipient: "0241111111", amount_pesewas: 5000 },
    { recipient: "0242222222", amount_pesewas: 3000 },
  ],
  idempotency_key: "orctago-2026-08-31", // optional
});
console.log(batch.batch_id, batch.total_pesewas);
```

### POS (Node) — direct use

```ts
import { OrctaPay } from "@orctatech/orcta-pay";
const pay = new OrctaPay({ apiKey: process.env.ORCTA_PAY_API_KEY! });
const res = await pay.createCharge({ product: "pos", amount_pesewas: 1800, currency: "GHS", wallet: "0241234567" });
```

Keep auth, reference, and sealed-result mapping identical to the Go client. The POS frontend can import this SDK directly instead of proxying through a backend.

### Errors

All non-2xx and network failures throw `OrctaPayError`:

```ts
import { OrctaPayError } from "@orctatech/orcta-pay";
try {
  await pay.createCharge({ product: "orctago", amount_pesewas: 100, wallet: "024..." });
} catch (e) {
  if (e instanceof OrctaPayError) {
    console.error(e.statusCode, e.code, e.message); // 401 unauthorized, 404 not_found, 500 internal_error, 408 timeout
  }
}
```

### Reference

Every charge gets `optd-{product}-{gateway}-{ulid}`. Pass `idempotency_key` to reuse the same intent on retry; omit it and the client generates `optd-{product}-hubtel-{ulid}` via Crockford Base32 ULID (same as `internal/gateway/reference.go`). Gateway segment is lower-cased.

```ts
import { generateReference } from "@orctatech/orcta-pay";
generateReference("pos", "hubtel"); // optd-pos-hubtel-01ARZ...
```

No retries — the service outbox handles that. Timeout is 10s; `AbortController` aborts the fetch.

## API

| Method | Endpoint | Description |
|---|---|---|
| `createCharge(req)` | `POST /v1/charges` | Initiate a charge, returns `ChargeResult` (`pending` \| `succeeded` \| `failed`) |
| `getChargeStatus(ref)` | `GET /v1/charges/{ref}/status` | Authoritative gateway truth |
| `createPayout(req)` | `POST /v1/payouts` | Create a payout batch |

## Development

```bash
npm test        # vitest
npm run build   # tsc -> dist/
```
