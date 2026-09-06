# Orcta Pay Clients

Thin clients for the Orcta Pay API (`api/openapi.yaml`).
The service owns retries, outbox, and circuit breakers.
Clients only handle transport, auth, and parsing.

## Auth

Each product gets its own API key.
The key is provisioned in Vault at `secret/orcta/orcta-pay/keys/{product}` and rendered to `.env` as `ORCTA_PAY_API_KEY`.

```bash
ORCTA_PAY_URL=https://api.pay.orctatech.com
ORCTA_PAY_API_KEY=pay_live_...   # per product: ORCTA_PAY_API_KEY_GO, ORCTA_PAY_API_KEY_POS in Vault
```

The client sends `Authorization: Bearer <apiKey>`.

## Go

Import path: `github.com/orctatech/orcta-pay/clients/go/orctapay`.

```go
import (
    "context"
    "os"

    "github.com/orctatech/orcta-pay/clients/go/orctapay"
    "github.com/orctatech/orcta-pay/internal/money"
)

client := orctapay.NewClient(os.Getenv("ORCTA_PAY_URL"), os.Getenv("ORCTA_PAY_API_KEY"))

res, err := client.CreateCharge(ctx, orctapay.CreateChargeRequest{
    Product: "orctago",
    Amount:  money.New(1800, money.GHS),
    Wallet:  "0241234567",
})
if err != nil {
    // 5xx wraps orctapay.ErrRequestFailed.
    // Retry with the same idempotency_key to avoid double charge.
}

switch v := res.(type) {
case orctapay.ChargePending:
    // v.Ref is optd-{product}-{gateway}-{ulid}
case orctapay.ChargeSucceeded:
    // settled
case orctapay.ChargeFailed:
    // v.Reason
}

status, err := client.GetChargeStatus(ctx, ref)

batch, err := client.CreatePayout(ctx, orctapay.CreatePayoutRequest{
    Product: "orctago",
    Entries: []orctapay.PayoutEntry{
        {Recipient: "0241111111", Amount: money.New(5000, money.GHS)},
    },
    IdempotencyKey: "orctago-2026-08-31",
})
```

Wiring example for OrctaGo — swap the Hubtel provider for Orcta Pay:

```go
// internal/platform/platform.go
import "github.com/orctatech/orcta-pay/clients/go/orctapay"

payClient := orctapay.NewClient(os.Getenv("ORCTA_PAY_URL"), os.Getenv("ORCTA_PAY_API_KEY"))

// Where payments.NewService previously took WithProcessors(hubtelProcessor, paystackProcessor):
// payments.WithOrctaPay(payClient) // adapter implements payments.Processor via Charger interface
```

The adapter translates `ChargeResult` sealed types one to one.

Client options:

```go
orctapay.NewClient("", apiKey) // baseURL defaults to ORCTA_PAY_URL or http://localhost:8080
orctapay.NewClient(url, apiKey, orctapay.WithHTTPClient(custom))
orctapay.NewClient("", apiKey, orctapay.WithBaseURL(testServer.URL))
```

Timeout is 10s.
Context cancellation is respected.
No retries — the service outbox handles that.

## Reference

Every charge gets `optd-{product}-{gateway}-{ulid}`.
Pass `IdempotencyKey` to reuse the same intent on retry.
Omit it and the client generates `optd-{product}-{ulid}`.

## Webhooks

Webhooks are service side only.
One endpoint per gateway (`/webhooks/hubtel`, `/webhooks/paystack`, `/webhooks/moolre`).
Attribution is via the `optd-` reference, not gateway routing.
Clients do not verify webhooks — see `internal/api/webhooks.go` for HMAC.

## Non-Go (POS, TypeScript)

POS may be Node.
A TypeScript client mirrors this shape:

```ts
const client = new OrctaPayClient(process.env.ORCTA_PAY_URL!, process.env.ORCTA_PAY_API_KEY!);
const res = await client.createCharge({ product: "pos", amountPesewas: 1800, currency: "GHS", wallet: "0241234567" });
// res.ref is optd-pos-...
const status = await client.getChargeStatus(res.ref);
const batch = await client.createPayout({ product: "pos", entries: [{ recipient: "024...", amountPesewas: 5000 }] });
```

Keep auth, reference, and sealed result mapping identical to the Go client.
