# Gateway Setup Guide

How to add production credentials for Paystack, Hubtel, and Moolre to the
running orcta-pay service.

## Current State

The service is live at `https://api.pay.orctatech.com/`. The dashboard and API
are served from the same image. Three gateways are supported:

| Gateway   | Status        | Key variable(s)                    |
|-----------|---------------|-------------------------------------|
| Paystack  | Not configured | `PAYSTACK_SECRET_KEY`              |
| Hubtel    | Not configured | `HUBTEL_CLIENT_ID` + `HUBTEL_CLIENT_SECRET` |
| Moolre    | Not configured | `MOOLRE_API_KEY`                   |

Infrastructure (Postgres, Valkey, API key, primary gateway) is already running.

## How to Add Keys

All changes happen on the VPS. The `.env` file is at:

```
/srv/apps/orcta-pay/deploy/app/.env
```

### Step 1: SSH into the VPS

```bash
ssh orcta_vps
```

### Step 2: Edit the .env file

```bash
sudo nano /srv/apps/orcta-pay/deploy/app/.env
```

#### Paystack

Set your Paystack secret key (starts with `sk_live_`):

```
PAYSTACK_SECRET_KEY=sk_live_...
```

This single key enables both API calls and webhook verification. Paystack
signs webhooks with HMAC-SHA512 using the same account secret key. The code
handles this automatically — no separate webhook secret is needed unless you
want one (see optional override below).

**Optional:** If you generate a separate webhook secret in the Paystack
dashboard, set it explicitly:

```
PAYSTACK_WEBHOOK_SECRET=whsec_...
```

Without this, the service falls back to `PAYSTACK_SECRET_KEY` for webhook
verification.

**Where to get the key:** [Paystack Dashboard](https://dashboard.paystack.com/) → Settings → API Keys & Webhooks → Live keys.

#### Hubtel

Set both the client ID and secret:

```
HUBTEL_CLIENT_ID=your_client_id
HUBTEL_CLIENT_SECRET=your_client_secret
```

**Where to get the keys:** Hubtel Merchant Dashboard → My Apps → select your
app → Credentials.

The base URL is already set to `https://payproxyapi.hubtel.com` (production).

#### Moolre

Set the API key:

```
MOOLRE_API_KEY=your_api_key
```

**Where to get the key:** Moolre dashboard → Settings → API Key.

The base URL is already set to `https://api.moolre.com` (production).

### Step 3: Restart the containers

After saving the `.env` file, restart the web and worker containers:

```bash
cd /srv/apps/orcta-pay/deploy/app
docker compose restart web worker
```

Or if you prefer a full recreation:

```bash
docker compose up -d web worker
```

### Step 4: Verify

Check that the gateway is reachable:

```bash
# From the VPS
curl -s http://localhost:8085/v1/gateways/health \
  -H "Authorization: Bearer $(grep ORCTA_PAY_API_KEY .env | cut -d= -f2)"
```

You should see the configured gateway marked as `ok` (or `unknown` if no
attempts have been made yet — that's normal).

## Webhook Configuration

Once the gateway key is set, configure the webhook URL in the gateway's
dashboard to point to:

```
https://api.pay.orctatech.com/webhooks/paystack
https://api.pay.orctatech.com/webhooks/hubtel
https://api.pay.orctatech.com/webhooks/moolre
```

The service deduplicates webhooks by event ID and verifies HMAC signatures
before processing. Duplicate or unknown-ref webhooks return `200` so the
gateway stops retrying.

## Primary Gateway

`PAYMENTS_PRIMARY` controls which gateway is used by default for charges.
Currently set to `paystack`. To change it:

```
PAYMENTS_PRIMARY=hubtel
```

Valid values: `paystack`, `hubtel`, `moolre`.

## Troubleshooting

**Gateway shows `down` or `error`:**
- Check the logs: `docker compose logs web --tail 50`
- Verify the key is set: `grep PAYSTACK_SECRET_KEY .env`
- Confirm the key is valid by testing the gateway's API directly

**Webhooks not processing:**
- Check logs for signature verification errors
- Ensure the webhook URL matches exactly (including path)
- For Paystack: confirm the secret key matches what's in the Paystack dashboard

**Dashboard not loading:**
- The dashboard is served at `https://api.pay.orctatech.com/`
- API endpoints are at `https://api.pay.orctatech.com/v1/...`
- Login uses the `ORCTA_PAY_API_KEY` value from `.env`

## Architecture Note

The dashboard and API are served from the same Go binary. The router at
`internal/api/router.go` handles:

- `/` → dashboard (static files from `dashboard/dist`)
- `/auth/*` → login/session/logout (cookie-based)
- `/v1/*` → API endpoints (bearer auth required)
- `/webhooks/*` → gateway webhooks (HMAC verified, no auth required)
- `/healthz`, `/readyz` → health checks

When accessed from the same origin, the dashboard uses relative URLs for API
calls. No CORS issues in production.
