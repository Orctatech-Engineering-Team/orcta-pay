# Orcta Pay Dashboard

Minimal operator dashboard for Orcta Pay — Charges, Payouts, Ledger, Gateways, Apps, Webhooks. Thin views over the same API any Orcta service uses via the TypeScript client. TanStack Query for data fetching with mock fallback when the API is unreachable.

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

For demo without `.env`, click **Settings** in the header and paste any product's key — stored in `localStorage` as `orcta_pay_api_key` / `orcta_pay_url` / `orcta_pay_product`. Env is the fallback; localStorage wins. The Settings modal now also shows the masked active app key and writes through to TanStack Query's client via `makeClient()` (reload picks up new key/baseUrl).

The header shows a green/red dot via TanStack Query (`useQuery(["health", baseUrl])` polling `GET /healthz` fallback `/readyz` every 15s) and the masked active key. When the API is unreachable, every page falls back to mock data but keeps the `OrctaPay` wiring live — click a charge to see a typed `OrctaPayError`.

## Apps — API keys

Orcta Pay runs on the VPS. The dashboard fetches API keys from Vault (`secret/orcta/orcta-pay/keys/{product}`) so each service can create an app:

- **Create an app**: Dashboard → **Apps** → **Create app** → enter `name` (e.g. `orctago`, `pos`) and `product` → `POST /v1/apps` via `client.createApp({name, product})` → response `{id, name, product, api_key, prefix}`. The `api_key` (`pay_live_…`) is shown once in a copyable code block — **Copy now — shown once** — with `pnpm add @orctatech/orcta-pay` snippet and `ORCTA_PAY_API_KEY=pay_live_…` for the service to plug into its library.
- **List**: `GET /v1/apps` via `client.listApps()` → table of `name`, `product`, `prefix`, `created_at`, `last_used_at`, `revoked`. TanStack Query `["apps"]` with staleTime 30s, retry 1, fallback to `mockApps` and banner “Live API unreachable — showing mock data” when offline.
- **Rotate / Revoke**: Per-row **Rotate** (`POST /v1/apps/{id}/keys/rotate` → `client.rotateAppKey(appId)` → new `api_key` shown once) and **Revoke** (`DELETE /v1/apps/{id}` → `client.revokeApp(appId)`). Mutations invalidate `["apps"]`.

Before the service lands, the dashboard runs against `mockApps` so it builds without a running API.

Vault → env: `secret/orcta/orcta-pay/keys/{product}` → rendered to `.env` as `ORCTA_PAY_API_KEY` (or pasted in Settings → localStorage).

## Pages

- **Charges** — `payment_intents` (ref, product, gateway, amount as GHS, status pending/succeeded/failed, created_at). Search by ref, filter by product/gateway/status via TanStack Query `["charges", {q, product, gateway, status}]`. `useQuery` tries `GET /v1/charges` then falls back to `mockCharges`; detail uses `useQuery(["chargeStatus", ref], () => client.getChargeStatus(ref))`. Banner “Live API unreachable — showing mock data” on query error.
- **Payouts** — `payout_batches` via `useQuery(["payouts"])` → `GET /v1/payouts`, fallback to `mockPayoutBatches` with banner. Click → per-vendor lines with reservation state `open→settled/released` and age.
- **Ledger** — `vendor_ledger_entries` + `platform_commission_entries` via `useQuery(["ledger", {vendor, kind}])` → `GET /v1/ledger`, fallback to `mockLedger`. Filter by vendor. Verifies `sum(debits + credits) + net = 0` over the visible slice.
- **Gateways** — Valkey ranking via `useQuery(["gateways"])` → `GET /v1/gateways/health`, fallback to `mockGateways`. Ranking is success-rate floor → cost tiebreak; open circuits excluded. See `PAYMENTS_SERVICE_DESIGN.md:4`.
- **Apps** — API keys (`POST /v1/apps`, `GET /v1/apps`, `POST /v1/apps/{id}/keys/rotate`, `DELETE /v1/apps/{id}`) via `createApp`/`listApps`/`rotateAppKey`/`revokeApp`. TanStack `["apps"]` with mock fallback.
- **Webhooks** — `webhook_inbox` via `useQuery(["webhooks", {gateway}])` → `GET /v1/webhooks`, fallback to `mockWebhooks` with banner and `UNIQUE(aggregator_event_id)` dedup highlight.

## Develop

```bash
pnpm install
pnpm run dev      # http://localhost:5173, proxy /v1 and /healthz to VITE_ORCTA_PAY_URL
pnpm run build    # tsc && vite build
pnpm run preview  # preview prod build on :5173
pnpm run lint     # tsc --noEmit
```

`vite.config.ts` proxies `/v1`, `/healthz`, `/readyz` to `VITE_ORCTA_PAY_URL` (or `ORCTA_PAY_URL`) for local dev so the browser avoids CORS. Requires `pnpm` only — `packageManager: pnpm@10.29.1`, no `package-lock.json`.

## Stack

Vite 6 + React 18 + TypeScript (strict) + TanStack Router (code-based `createRouter` + `createRootRoute` in `src/router.tsx`) + TanStack Query 5 (staleTime 30s, retry 1) + TanStack Form + Zod + Base UI (unstyled primitives) + pnpm only. Plain CSS + `src/styles/baseui.css` (focus rings, dialog, toast) — no Tailwind build step. Local path dep `file:../clients/ts/orctapay` → `from "@orctatech/orcta-pay"`. `QueryClientProvider` wraps `RouterProvider` in `src/main.tsx`; routes (`/`, `/charges`, `/payouts`, `/ledger`, `/gateways`, `/apps`, `/webhooks`) are defined in `src/router.tsx` with `Layout` as root and `Outlet`. Forms use `useForm` from `@tanstack/react-form` with `zod` schemas (`createAppSchema`, `settingsSchema` in `src/lib/validators.ts`) and Base UI `Field`/`Dialog`/`Select`/`Input`/`Button`/`Tabs`/`Toast`. Deps installed via `pnpm add @tanstack/react-router @tanstack/react-form zod @base-ui/react` and `pnpm add -D @tanstack/router-plugin`. Server state = TanStack Query, client UI state = React `useState` inside route components (q, product, gateway filters, selected row, settings open) — add Zustand (`src/lib/store.ts`) only when cross-route client state grows, per the guide's "prefer boring" rule.
