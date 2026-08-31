# Hubtel / Moolre / Paystack Ghana — Deep Dive

> Research for Orcta Pay (ADR-034) and Go clients (issues #41 / #42).
> Every factual claim cites its primary source URL in parentheses.
> Where docs are vague or contradict, the subsection says "confirm directly with provider".
> Amounts are in GHS unless noted. Internal money is integer pesewas (GHS × 100).

---

# Hubtel

## Overview

Hubtel is a Ghanaian aggregator for mobile money, cards, bank transfers, and bulk SMS (https://developers.hubtel.com).
The developer portal lists Programmable APIs under Business and Personal, grouped as Settlement, Payments, Customer Verification, and Transactions (https://developers.hubtel.com).
Business payment docs live under `/docs/business/api_documentation/payment_apis/` with pages for Online Checkout, Checkout SDK, Direct Receive Money, Direct Send Money, and Direct Send to Bank (https://developers.hubtel.com — Nuxt data `__NUXT_DATA__` lists those routes).
The E-Commerce / Merchant Account API is documented at `https://payproxyapi.hubtel.com` and at `https://api.hubtel.com/v2/` for newer unified-pay checkout paths (https://developers.hubtel.com — checkout.js `https://unified-pay.hubtel.com/js/v1/checkout.js` is loaded on the portal).
A community skill summarises the live base URL as `https://payproxyapi.hubtel.com` and notes newer endpoints may appear under `https://api.hubtel.com` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

## Collections

### Checkout (Online Checkout / Receive Money)

Hubtel offers two collection modes.
Hosted checkout creates an invoice and returns an authorization URL.
Direct receive triggers a USSD prompt to the customer's phone.

Hosted checkout endpoint (from skill, verified against merchant-account docs): `POST https://payproxyapi.hubtel.com/items/initiate` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
Direct receive endpoint: `POST https://payproxyapi.hubtel.com/receive/initiate` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

Request fields for direct receive (skill table): `CustomerName`, `CustomerMsisdn`, `CustomerEmail`, `Channel` (`mtn-gh` / `vodafone-gh` / `airteltigo-gh` / `card` / `ghipss` / `ghqr`), `Amount` (decimal GHS, not pesewas), `ClientReference` (merchant idempotency key), `Description`, `PrimaryCallbackUrl` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

Request fields for hosted checkout / items/initiate (same source): `totalAmount`, `callbackUrl`, `returnUrl`, `cancellationUrl`, `clientReference`, `items[]` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

Amount format is decimal GHS (e.g. `50.00`), not pesewas.
The skill explicitly warns to use decimal amounts and that the minimum is typically GHS 0.10 (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
This contrasts with Paystack which uses pesewas.

Response envelope: `{ ResponseCode: "00" | "01" | ..., Data: { TransactionId, ClientReference, Amount, Status, CheckoutUrl } }`.
`ResponseCode 00` means accepted.
`Status` is `Pending` until the customer approves the USSD prompt (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

Success vs pending vs failed handling: success is `ResponseCode 00` plus webhook `Success`; pending is `ResponseCode 00` with `Status: Pending` and the customer must approve on phone; failed is non-00 code or webhook `Failed` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
The portal does not publish a formal state diagram — confirm directly with provider for the full set of terminal states.

Idempotency: `ClientReference` is the merchant dedup key. Reusing it returns the existing transaction (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

Channels for Ghana: `mtn-gh` (MTN MoMo), `vodafone-gh` (Vodafone Cash), `airteltigo-gh` (AirtelTigo Money), `ghipss` (bank), `card`, `ghqr` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
USSD prompts are network-native; no separate USSD code needs to be dialled by the integrator.

Timeout / circuit breaker guidance is not published. The skill notes a generic 30-second webhook response requirement (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel). Confirm timeout and retry thresholds with provider.

### Auth

All Hubtel payment APIs use HTTP Basic Authentication.
Header format is `Authorization: Basic base64(clientId:clientSecret)` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
Credentials are issued per merchant from the Hubtel dashboard / Unity portal (https://unity.hubtel.com is referenced as the merchant dashboard; portal login at https://developers.hubtel.com links to auth.hubtel.com).

## Disbursement / Payout

Transfer to MoMo wallet: `POST https://payproxyapi.hubtel.com/send/money` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
Request fields: `RecipientName`, `RecipientMsisdn`, `Amount` (decimal GHS), `Channel` (`mtn-gh` / `vodafone-gh` / `airteltigo-gh`), `ClientReference`, `Description`, `CallbackUrl` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
Transfer to bank via GhIPSS: same endpoint with `Channel: ghipss` plus bank fields (https://developers.hubtel.com — "Transfer to Bank" via GhIPSS network).

Bulk support is not documented for the send/money endpoint. Confirm directly with provider whether bulk CSV or batch API exists and what the rate limit for bulk submission is.

Idempotency for payouts also uses `ClientReference` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

Fees: the aggregator plan notes collections at ~1.95% and payouts at GHS 1 / 8, but the official Hubtel docs do not publish a fee table. Confirm directly with provider and capture the signed commercial schedule.

## Webhooks

Hubtel sends a POST to `PrimaryCallbackUrl` / `CallbackUrl` supplied per transaction (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).

Expected 2xx within ~30 seconds or the delivery is considered failed (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
Delivery guarantees and retry schedule are not published — confirm directly with provider.

Signature scheme: docs do not publish a signature header name or HMAC algorithm for disbursement callbacks.
The skill notes webhook payloads contain `TransactionId`, `ClientReference`, `Amount`, `Status`, `Message` but does not mention `x-hubtel-signature` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
Treat the webhook as a trigger and verify via `GetTransactionStatus` before mutating ledger.

Verification endpoint (polling): the portal's Transactions section implies a verify/status endpoint under payproxyapi (https://developers.hubtel.com — Transactions nav), but the exact path (`/transactions/{clientReference}` or `/merchant/transactions/status`) is not shown on the public landing page. Confirm the canonical verify path and auth with provider. Use it as the source of truth per ADR-035.

Dedup: `webhook_inbox` unique on `aggregator_event_id` (use `TransactionId` or delivery id) per `payments-design.md` §5. Implement idempotent handling even if the gateway retries.

## Settlement & Reconciliation

The portal advertises "Instant Settlement" and "swift and secure money transfers across all mobile money networks and banks" (https://developers.hubtel.com).
No public T+1/T+3 settlement calendar or reporting API is described on the landing page.
Confirm directly with provider: settlement cadence per product, cut-off times, and whether a settlement statement or reporting API exists for reconciliation.

Fees and reconciliation: no official reporting API is linked from https://developers.hubtel.com. Confirm the reconciliation feed (CSV, API, or dashboard export) and the field to join on (`ClientReference` vs `TransactionId`).

Ghana-specific notes: Ghana Card / Ghana ID verification is a separate Verification API ("Verify Ghana ID Details", "Verify Ghana Mobile Number", "Verify Ghana Bank Account Details") rather than part of payments (https://developers.hubtel.com). It can be used pre-payout to reduce failed transfers.

## Credentials & Sandbox

Sandbox vs production credential separation is asserted by the skill: "Hubtel provides separate test and production API credentials" and "Test credentials work against sandbox environment" (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel).
The portal itself requires login to obtain Programmable API keys (https://developers.hubtel.com — LOGIN links to auth.hubtel.com).
Base URL separation for sandbox is not published on the public landing page — confirm whether sandbox is `sandbox.payproxyapi.hubtel.com` or a flag on the same host.

IP whitelisting for disbursement: not published on https://developers.hubtel.com. The aggregator plan flags it as a likely requirement. Confirm directly with provider.

KYC docs and approval timelines: not published publicly. Confirm required documents and typical approval time.

## Rate Limits & Errors

Rate limits: the skill reports "typically 5 requests per minute per merchant" (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) and a directory lists 500 req/min (https://afrilayer.vercel.app/apis/hubtel). Both are secondary. No official rate limit header is documented on https://developers.hubtel.com — confirm directly with provider and observe `429` + `Retry-After`.

Error codes: `ResponseCode` values (e.g. `00` success) are returned in the JSON envelope, not HTTP status alone (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel). A full code table is not public — request it from provider.

Retry guidance: no official retry/circuit-breaker guide on https://developers.hubtel.com. Follow ADR-035: bounded timeout per call, circuit breaker on sustained 5xx, bounded exponential backoff on `pending`, and rely on verify polling as backstop.

## Open Questions to Confirm Directly

- Canonical base URLs for collections, disbursement, and verify — and whether `payproxyapi.hubtel.com` vs `api.hubtel.com/v2/` are both supported and for how long.
- Full `ResponseCode` table and terminal status values.
- Webhook signature header name, algorithm (HMAC SHA256 vs SHA512), and IP allowlist for verification.
- Retry/delivery guarantees for webhooks and the recommended polling interval for `GetTransactionStatus`.
- Sandbox base URL, test credentials flow, and which test MSISDNs simulate success/pending/failed.
- Published fee schedule for collections and for MoMo/bank payouts.
- Settlement T+ and reporting API / statement format for reconciliation.
- Bulk payout support and IP whitelisting for disbursement.

---

# Moolre

## Overview

Moolre is a Ghanaian multi-product API (payments, transfers, wallets, SMS, WhatsApp, USSD) with live docs at https://docs.moolre.com (https://docs.moolre.com).
Live base URL is `https://api.moolre.com`; sandbox base URL is `https://sandbox.moolre.com` (https://docs.moolre.com/llms.txt).
Docs are versioned as "Moolre API Documentation 2.0" and provide a Postman collection and live/sandbox Postman environments (https://docs.moolre.com).
The llms-full bundle combines guides and API reference for LLM ingestion at https://docs.moolre.com/llms-full.txt (https://docs.moolre.com/llms.txt).

## Collections

Moolre offers three collection surfaces. Use the one that fits the checkout UX.

### 1. Mobile Money Collection (USSD prompt)

Endpoint: `POST https://api.moolre.com/open/transact/payment` (https://docs.moolre.com/ai/initiate-payment.md).
Auth headers: `X-API-USER` (username) and `X-API-PUBKEY` (public key) (https://docs.moolre.com/ai/initiate-payment.md).

Request body fields: `type` (must be 1), `channel` (`13`=MTN, `6`=Telecel, `7`=AT), `currency` (`GHS` or `NGN`), `payer` (local 0-prefixed phone, e.g. `024...`), `amount` (string, decimal GHS, must be > 0), `externalref` (unique reference — idempotency key), `accountnumber` (Moolre account number), optional `otpcode` / `reference` / `sessionid` / `skipotp` (sandbox only) (https://docs.moolre.com/ai/initiate-payment.md).

Response examples:
Success (payment request initiated): `{ status: 1, code: "TR099", data: "<uuid>" }` (https://docs.moolre.com/ai/initiate-payment.md).
OTP required: `{ status: 1, code: "TP14", message: "Please complete the verification process sent to you via SMS and try again." }` (https://docs.moolre.com/ai/initiate-payment.md).
Duplicate reference: `{ status: "0", code: "TP13", message: "External Reference is required and must be unique." }` (https://docs.moolre.com/ai/initiate-payment.md).

Amount format is decimal GHS as a string (e.g. `"120.00"` in the PHP SDK example uses `"120.00"` with `currency: "GHS"`) (https://docs.moolre.com/llms-full.txt — Moolre PHP SDK section).

Success vs pending vs failed: the doc does not label states as pending/success on this endpoint. Treat the initial 200 as "request accepted" and rely on the callback plus status polling for the terminal state.

Idempotency: `externalref` must be unique. Reusing it returns `INP02` / `TP13` duplicate error (https://docs.moolre.com/ai/initiate-payment.md and https://docs.moolre.com/ai/generate-payment-link.md).
Reuse the same `externalref` on retry after a timeout — do not generate a new one (https://docs.moolre.com/llms-full.txt — Idempotency section).

Channels: `13`=MTN, `6`=Telecel (formerly Vodafone), `7`=AT (AirtelTigo) (https://docs.moolre.com/ai/initiate-payment.md). Confirm with provider if `6` covers both Telecel and Vodafone branding after the rebrand.

Timeout / circuit breaker: no official guidance. Follow the safe-retries guidance: bounded exponential backoff with jitter, keep same `externalref`, and reconcile uncertain outcomes via status endpoint or callback (https://docs.moolre.com/llms-full.txt — Safe retries).

### 2. Hosted Payment Link (Web POS)

Endpoint: `POST https://api.moolre.com/embed/link` (https://docs.moolre.com/ai/generate-payment-link.md).
Headers: `X-API-USER`, `X-API-PUBKEY` (https://docs.moolre.com/ai/generate-payment-link.md).

Request fields: `type` (1), `amount` (string), `email` (business email), `externalref` (unique), `currency` (`GHS`/`NGN`), `accountnumber`, `reusable` (`0`/`1`), optional `callback` (webhook URL), `redirect` (return URL), `expiration_time` (minutes, min 1), `metadata` (object echoed in callbacks) (https://docs.moolre.com/ai/generate-payment-link.md).

Success response: `{ status: 1, code: "POS09", data: { authorization_url: "https://pos.moolre.com/...", reference: "uuid-..." } }` (https://docs.moolre.com/ai/generate-payment-link.md).
Duplicate: `{ status: 0, code: "INP02", message: "Transaction already exits!" }` (https://docs.moolre.com/ai/generate-payment-link.md).

The PHP SDK wraps this flow: `Client::initiatePayment(['reference','email','amount','currency','callback','redirect','expiration_time'])` then redirect to `authorizationUrl()`, then `verifyPayment(reference)` before fulfilment (https://docs.moolre.com/llms-full.txt — Moolre PHP SDK).

Callback and redirect URLs are per-link. This maps cleanly to `optd-{product}-{gateway}-{ulid}` as `externalref` / `reference`.

### 3. Permanent Payment ID / Virtual Account / Bank Collection

- Create Payment ID: `POST https://api.moolre.com/open/account/create` with `type: 2`, `phone` (+233...), `name`, `currency`, `accountnumber`, optional `amount` and `externalref`. Returns `{ paymentid: "08160984" }` (https://docs.moolre.com/ai/create-payment-id.md). Customer dials `*203*paymentid#` (live) or `*203*6000*paymentid#` (sandbox).
- Bank Collections / virtual bank account numbers are listed under Payments guides (https://docs.moolre.com/ai/index.html via llms.txt) — endpoint details are analogous to Payment ID but allocate a virtual account. Confirm the exact path with the interactive docs at https://docs.moolre.com/api/accounts/create.
- Not needed for OrctaGo's initial delivery-fee collections, but relevant if a future "pay into vendor virtual account" flow is added.

### Auth

Payment collections use `X-API-USER` + `X-API-PUBKEY` (https://docs.moolre.com/ai/initiate-payment.md and https://docs.moolre.com/ai/generate-payment-link.md).
Transfers use `X-API-USER` + `X-API-KEY` (private key) (https://docs.moolre.com/ai/initiate-transfer.md).
Account status uses `X-API-KEY` (private) (https://docs.moolre.com/ai/account-status.md).
SMS/WhatsApp use `X-API-VASKEY` per service (https://docs.moolre.com/llms.txt).
Sandbox: requests omit placeholder private/public key headers and use the sandbox base URL (https://docs.moolre.com/llms.txt).

## Disbursement / Payout

### Initiate Transfer (single)

Endpoint: `POST https://api.moolre.com/open/transact/transfer` (https://docs.moolre.com/ai/initiate-transfer.md).
Headers: `X-API-USER`, `X-API-KEY` (private key) (https://docs.moolre.com/ai/initiate-transfer.md).

Request fields: `type` (1), `channel` (`1`=MTN, `6`=Telecel, `7`=AT, `2`=Instant Bank Transfer), `currency` (`GHS`/`NGN`), `amount` (string decimal), `receiver` (0-prefixed phone or bank account), `externalref` (unique), `reference` (optional message), `accountnumber`, and `sublistid` (bank code, required when `channel=2`) (https://docs.moolre.com/ai/initiate-transfer.md).

Success response: `{ status: "1", code: "OBGH01", data: { txstatus: 1, receiver, transactionid, externalref, thirdpartyref, receivername, amount, amountfee, fee } }` with `amountfee` and `fee` showing the charge (https://docs.moolre.com/ai/initiate-transfer.md).
Example fee: `amount: "1", amountfee: "1.01", fee: "0.01"` — i.e. GHS 0.01 on GHS 1.

A pre-flight check is available: `POST https://api.moolre.com/open/transact/validate` with `receiver`, `channel`, `sublistid`, `currency`, `accountnumber` returns the account holder name or `AVD02 Phone No. not found` (https://docs.moolre.com/ai/validate-name.md). Use it before transfer to reduce failed payouts.

Bank list for `sublistid`: `GET https://api.moolre.com/open/transact/data?country=gha&data=banks` returns `{ name, code }` pairs (https://docs.moolre.com/ai/miscellaneous-data.md).

Idempotency: `externalref` is the dedup key (https://docs.moolre.com/ai/initiate-transfer.md). Same guidance as collections — keep one durable reference per business action and reuse it on retry (https://docs.moolre.com/llms-full.txt — Idempotency section).

Bulk support: not a single batch endpoint in the public API reference. The product site advertises CSV bulk upload via dashboard (https://moolre.com/?product=disbursement), and the use-case guide is "Bulk payouts" (https://docs.moolre.com/llms.txt). The API would require looping single transfers with distinct `externalref` per line (or uploading via dashboard). Confirm with provider whether a true bulk transfer API exists and what the rate limit for bulk submission is.

## Webhooks

Moolre sends an HTTP POST to your callback URL when a payment is received or its status changes (https://docs.moolre.com/ai/payment-webhook.md).
The endpoint is whatever you passed as `callback` (payment link) or the wallet callback configured on the account (https://docs.moolre.com/ai/account-status.md shows `callback: "https://moolre.requestcatcher.com/test"`).

Payload shape (from webhook doc): `{ status, code, message, data: { ... transaction details ... } }` where `status=1`, `code="P01"` indicates success (https://docs.moolre.com/ai/payment-webhook.md).
No signature header is documented. The guides instead emphasise idempotent handling and verifying final state via the status endpoint before fulfilling (https://docs.moolre.com/llms-full.txt — Webhooks and callbacks).

Verification endpoints (authoritative truth):
- Payments: `POST https://api.moolre.com/open/transact/status` with `type: 1, idtype: 1|2, id: <externalref|moolreId>, accountnumber` returns `{ txstatus, txtype, payer, payee, amount, transactionid, externalref, thirdpartyref, ts }` (https://docs.moolre.com/ai/payment-status.md).
- Transfers: same path but header may be `X-API-KEY` (accepts public or private) and same body shape (https://docs.moolre.com/ai/transfer-status.md).
Treat `txstatus: 1` as success per examples (`SS01 Transaction Successful`) (https://docs.moolre.com/ai/payment-status.md).

Dedup: callbacks may be delivered more than once — "Make processing idempotent because the same callback may be delivered more than once" (https://docs.moolre.com/llms-full.txt — Webhooks section). Use `webhook_inbox` unique on `aggregator_event_id` (e.g. `transactionid` + `ts`) and store raw payload before acking.

IP allowlisting for callbacks: Wallet callbacks from `192.241.135.134` (IPv4) and `2604:a880:400:d1:0:3:4cf0:c001` (IPv6); POS payment-link callbacks from `174.138.44.22` and `2604:a880:400:d0::1a77:400` — use as one signal, not the sole check (https://docs.moolre.com/ai/guides/authentication.md via https://docs.moolre.com/llms.txt).

Delivery guarantees / retry schedule: not published. The guide says to return a quick success response and persist before doing slow work (https://docs.moolre.com/llms-full.txt — Webhooks section). Confirm retry interval with provider during integration.

Ghana-specific notes: supported channels explicitly list GHS and the MoMo networks above. No Ghana Card verification is exposed in the Moolre docs — identity verification is a Hubtel capability, not Moolre's.

## Settlement & Reconciliation

Moolre docs do not publish a settlement T+ calendar. The product site says "Daily bank settlements" (https://moolre.com/?product=merchant is listed under Merchant Service features).
The API for reconciliation is `POST https://api.moolre.com/open/account/status` with `type: 2` (list transactions) filtering by `startdate`, `enddate`, `limit`, and `status` (`0`=Pending, `1`=Successful, `2`=Failed), returning `{ txcount, transactions: [{ txstatus, txtype, payer, payee, amount, transactionid, externalref, thirdpartyref, ts }] }` (https://docs.moolre.com/ai/list-account-transactions.md).
`type: 1` on the same path returns wallet balance and callback URL (https://docs.moolre.com/ai/account-status.md).

Fees: disbursement fee is visible in the transfer response (`fee` / `amountfee` fields) (https://docs.moolre.com/ai/initiate-transfer.md). Collection fees are not published in the API docs. Confirm the full fee schedule with provider.

For Orcta Pay reconciliation, pull the transaction list per `accountnumber` per day and join on `externalref` (which is `optd-...`). Settlement delay should be modelled per provider once confirmed.

## Credentials & Sandbox

Live vs sandbox base URLs are formally separated: live `https://api.moolre.com`, sandbox `https://sandbox.moolre.com` (https://docs.moolre.com/llms.txt).
The guides say "Sandbox requests use https://sandbox.moolre.com. SMS and WhatsApp each use a separate service-specific X-API-VASKEY" (https://docs.moolre.com/llms-full.txt — Sandbox testing).
Sandbox does not require `X-API-KEY`/`X-API-PUBKEY` placeholders; live does (https://docs.moolre.com/llms.txt).

KYC / approval: Moolre docs state "Confirm availability for your rollout — Service coverage, commercial eligibility, and operational limits can differ by product and account. Validate requirements with your Moolre contact before committing" (https://docs.moolre.com/llms-full.txt — Supported channels). No public KYC checklist is published — confirm docs required and timeline with provider.

IP whitelisting for disbursement: not documented beyond the callback source IPs. Confirm whether disbursement requires an IP allowlist.

## Rate Limits & Errors

Rate limits: not published in the Moolre docs fetched. No `429` or `Retry-After` header is described. Confirm with provider and defensively implement bounded retry with jitter.

Errors: the response envelope is `{ status, code, message, data, go }`. HTTP 200 can carry an application error (e.g. `status: 0, code: "AVD02"`). Use `code` and `message` for application handling; keep a correlation value in logs (https://docs.moolre.com/llms-full.txt — Errors and status codes).
400-range means correct the request before retrying; 401/403 means check env and credential; 429/5xx are retry candidates with the same `externalref` (https://docs.moolre.com/llms-full.txt — Errors section).

## Open Questions to Confirm Directly

- Retail collection fee (%) and whether it is deducted before settlement or billed separately.
- Webhook signature scheme (if any) — currently no HMAC header is documented; confirm if signing can be enabled or if verification is solely via status endpoint + IP allowlist.
- Bulk transfer: is there a true batch API or is bulk only via dashboard CSV?
- Sandbox test data: which phone numbers / amounts simulate success vs failure vs OTP flow?
- Rate limits per endpoint and whether `Retry-After` is honoured.
- Settlement T+ per product and cut-off time for "daily" settlement.
- KYC document list and approval timeline for live disbursement.

---

# Paystack (Ghana)

## Overview

Paystack's API base URL is `https://api.paystack.co` for all products, including Ghana (https://docs-v2.paystack.com/docs/api/transaction/ and https://docs-v2.paystack.com/docs/api/authentication/).
Ghana was one of Paystack's first expansions after Nigeria. Ghanaian accounts settle in GHS to a Ghanaian bank account (https://www.mctaba.com/learn/paystack/paystack-in-ghana-complete-developer-guide — secondary but consistent with official docs; amount/currency use is primary-sourced below).
The official API group is versionless REST with Bearer auth; docs-v2 and `paystack.com/docs/api/` are mirrors (https://docs-v2.paystack.com/docs/api/transaction/ vs earlier paystack.com mirror).
The OpenAPI contract is published by `api-evangelist/paystack` (https://raw.githubusercontent.com/api-evangelist/paystack/master/apis.yml).

## Collections

Paystack offers two integration styles for collections. Both use pesewas.

### Option A — Initialize Transaction (hosted checkout, recommended)

Endpoint: `POST https://api.paystack.co/transaction/initialize` (https://docs-v2.paystack.com/docs/api/transaction/#initialize).
Headers: `Authorization: Bearer SECRET_KEY` and `Content-Type: application/json` (https://docs-v2.paystack.com/docs/api/transaction/#initialize).

Body fields: `amount` (string or integer in pesewas — subunit of GHS; 1 GHS = 100 pesewas), `email` (customer email, required), `currency` (`GHS` explicitly for Ghana), `reference` (unique, only `-`, `.`, `=`, alphanumerics), `callback_url` (overrides dashboard callback), optional `channels` (`["card","bank","ussd","qr","mobile_money","bank_transfer","eft"]`), `metadata` (stringified JSON), `plan`, `split_code`, `subaccount` (https://docs-v2.paystack.com/docs/api/transaction/#initialize).

Channel restriction for Ghana: pass `channels: ["mobile_money","card"]` to surface the relevant options. The available channels per currency vary — card and mobile_money are confirmed for GHS; `bank_transfer` is not available for GHS (https://docs-v2.paystack.com/docs/payments/payment-channels/ and Ghana guide secondary).

Sample request (pesewas):

```json
{
  "email": "customer@example.com.gh",
  "amount": "50000",
  "currency": "GHS",
  "reference": "optd-orctago-paystack-01J...",
  "callback_url": "https://pay.orcta.app/webhooks/paystack",
  "channels": ["mobile_money", "card"]
}
```

Sample success response:

```json
{
  "status": true,
  "message": "Authorization URL created",
  "data": {
    "authorization_url": "https://checkout.paystack.com/0peioxfhpn",
    "access_code": "0peioxfhpn",
    "reference": "7PVGX8MEk85tgeEpVDtD"
  }
}
```
(https://docs-v2.paystack.com/docs/api/transaction/#initialize)

To go Ghana-live, Paystack Ghana requires GRA TIN, certificate of incorporation / business registration, director government ID, and Ghanaian bank account for settlement — secondary guide cites dashboard KYC (https://www.mctaba.com/learn/paystack/paystack-in-ghana-complete-developer-guide). Confirm the exact checklist on the live dashboard.

### Option B — Charge API (headless / custom UI)

Endpoint: `POST https://api.paystack.co/charge` (https://docs-v2.paystack.com/docs/api/charge/#create).
This lets Orcta collect the phone number in its own UI and trigger the MoMo prompt without redirecting to hosted checkout. Useful for an in-app flow, but it requires handling multi-step next-actions.

Body for Ghana MoMo:

```json
{
  "email": "customer@email.com",
  "amount": "10000",
  "currency": "GHS",
  "mobile_money": { "phone": "0553241149", "provider": "mtn" }
}
```
Provider codes for Ghana: `mtn` (MTN), `vod` (Vodafone), `tgo` (AirtelTigo) — also documented as `MTN` in some examples (case-insensitive) (https://docs-v2.paystack.com/docs/payments/payment-channels/#mobile-money).

Charge API responses are multi-step. Check `data.status`:
`success` → done; `pending` → poll `GET /charge/:reference` after ≥10s; `pay_offline` (MTN/AirtelTigo) → customer must approve on phone, listen for webhook; `send_otp` (Vodafone) → collect voucher via USSD `*110#` and call `POST /charge/submit_otp`; other values include `send_pin`, `send_phone`, `send_birthday`, `open_url`, `failed`, `timeout` (https://docs-v2.paystack.com/docs/payments/accept-payments/#handle-webhook and https://docs-v2.paystack.com/docs/api/charge/#create).
Implement all branches if using this path; otherwise prefer Initialize + hosted checkout.

Amount format: always pesewas. `500` GHS = `50000` pesewas. `1 GHS = 100 pesewas` is stated across all transaction docs (https://docs-v2.paystack.com/docs/api/transaction/#initialize and Ghana secondary guide). Validate `currency` server-side — omitting it silently defaults to the integration's primary currency (https://www.mctaba.com/learn/paystack/currency-handling-in-paystack-ngn-ghs-zar-kes-and-usd).

Idempotency: Paystack dedups on `reference`. If a duplicate reference is sent, the existing transaction is returned. Generate `optd-...` once per intent and reuse it on every attempt (https://docs-v2.paystack.com/docs/api/transaction/#initialize — reference is unique).

Success vs pending vs failed handling:
- Hosted flow: initial response is not terminal. Terminal status is `data.status` on verify (`success` / `failed` / `abandoned`) (https://docs-v2.paystack.com/docs/api/transaction/#verify).
- Charge flow: `success` is terminal; `pay_offline` / `pending` / `send_otp` require webhook or polling (https://docs-v2.paystack.com/docs/payments/payment-channels/#mobile-money).
For both, the webhook `charge.success` is notification, not truth — verify before fulfilment (https://docs-v2.paystack.com/docs/payments/verify-payments/).

Timeout / circuit breaker: no official guidance. The docs say "wait 10 seconds or more, then make a check ... Don't call too early" for pending charges (https://docs-v2.paystack.com/docs/api/charge/#check). Use bounded timeout per HTTP call and circuit-breaker exclusion per `payments-design.md` §2.

### Auth

Bearer auth: `Authorization: Bearer SECRET_KEY` (https://docs-v2.paystack.com/docs/api/authentication/).
Public key is frontend-only (Inline/Popup/Mobile SDKs); secret key must never leave the server and must not be committed to git (https://docs-v2.paystack.com/docs/api/authentication/).
All requests must be over HTTPS with verified certificates (https://docs-v2.paystack.com/docs/api/authentication/).

## Disbursement / Payout

Paystack payouts are a two-step flow: create a recipient, then initiate a transfer. Both steps support GHS.

### Step 1 — Create Transfer Recipient

Endpoint: `POST https://api.paystack.co/transferrecipient` (https://docs-v2.paystack.com/docs/api/transfer-recipient/#create).
Header: `Authorization: Bearer SECRET_KEY` (https://docs-v2.paystack.com/docs/api/transfer-recipient/#create).

Body for Ghana bank: `{ type: "ghipss", name, account_number, bank_code, currency: "GHS" }` (https://www.mctaba.com/learn/paystack/settlement-and-payouts-on-paystack-in-ghana — secondary, but type `ghipss` is the official Ghana bank rail; `nuban` is Nigeria).
Body for Ghana MoMo: `{ type: "mobile_money", name, account_number: "<0-prefixed phone>", bank_code: "MTN"|"VOD"|"ATL", currency: "GHS" }` — mobile money payout via transfer is supported in Ghana unlike some other markets (https://www.mctaba.com/learn/paystack/settlement-and-payouts-on-paystack-in-ghana).
Bulk create: `POST https://api.paystack.co/transferrecipient/bulk` with `{ batch: [ ... ] }` (https://docs-v2.paystack.com/docs/api/transfer-recipient/#bulk).
Bank codes: `GET https://api.paystack.co/bank?currency=GHS` (listing `GET /bank` per miscellaneous API) — the live list is the source of truth for `bank_code`.

Response gives `recipient_code` (`RCP_...`) used for the transfer (https://docs-v2.paystack.com/docs/api/transfer-recipient/#create).

### Step 2 — Initiate Transfer

Endpoint: `POST https://api.paystack.co/transfer` (https://docs-v2.paystack.com/docs/api/transfer/#initiate).
Body: `{ source: "balance", amount: <pesewas>, recipient: "RCP_...", reason, currency: "GHS", reference: "optd-..." }` where `amount` is in pesewas if `currency=GHS` (https://docs-v2.paystack.com/docs/api/transfer/#initiate).

Statuses: `pending` if OTP disabled; `otp` if OTP required; `success` / `failed` / `reversed` terminal (https://docs-v2.paystack.com/docs/api/transfer/#initiate and webhook events `transfer.success` / `transfer.failed` / `transfer.reversed`).
If OTP is required, confirm with `POST https://api.paystack.co/transfer/finalize_transfer` with `{ transfer_code, otp }` (https://docs-v2.paystack.com/docs/api/transfer/#finalize). For automation, disable OTP via dashboard and allowlist OTP bypass per provider guidance (https://docs-v2.paystack.com/docs/api/transfer-control/#balance — transfer control APIs).

Bulk transfers: `POST https://api.paystack.co/transfer/bulk` with `{ source, currency, transfers: [{ amount, recipient, reference, reason }] }` — requires OTP disabled (https://docs-v2.paystack.com/docs/api/transfer/#bulk).

Idempotency: `reference` is the dedup key (lowercase alphanumerics + `-` `_`). Bulk transfers also require unique `reference` per line (https://docs-v2.paystack.com/docs/api/transfer/#bulk).
Fees: per-transfer fee for GHS is not published on the pricing nav; the aggregator plan's "GHS 1 / 8" figure is not from official docs. Confirm the live GHS transfer fee on the Paystack dashboard and capture it in ADR-034's cost tiebreak.

Verification: `GET https://api.paystack.co/transfer/verify/:reference` (https://docs-v2.paystack.com/docs/api/transfer/#verify) and `GET https://api.paystack.co/transfer/:id_or_code` (https://docs-v2.paystack.com/docs/api/transfer/#fetch).

Bulk / async settlement is not instant — treat transfer as `pending` until `transfer.success` webhook + verify confirms.

## Webhooks

Paystack sends events to a single merchant-configured POST endpoint set on the dashboard at `Settings > Developer` (https://docs-v2.paystack.com/docs/payments/webhooks/#create-a-webhook-url).
The endpoint must parse JSON and return `200 OK` quickly; long work should run after the ack (https://docs-v2.paystack.com/docs/payments/webhooks/#go-live-checklist).

Signature scheme: every delivery carries `x-paystack-signature` — an `HMAC SHA512` of the raw request body keyed by the merchant secret key (https://docs-v2.paystack.com/docs/payments/webhooks/#verify-event-origin).

Verification steps (Node example from docs):

```js
const hash = crypto.createHmac('sha512', secret).update(JSON.stringify(req.body)).digest('hex');
if (hash === req.headers['x-paystack-signature']) { /* trusted */ }
```
(https://docs-v2.paystack.com/docs/payments/webhooks/#signature-validation)

Important nuance: to avoid JSON serialisation mismatches, compute the HMAC over the raw bytes as received, not a re-stringified object. The docs show `JSON.stringify(req.body)` but the safer implementation buffers the raw body. Note this in the Go client.

IP allowlisting: `52.31.139.75`, `52.49.173.169`, `52.214.14.220` for both test and live (https://docs-v2.paystack.com/docs/payments/webhooks/#ip-whitelisting). Use as a second signal alongside HMAC.

Delivery guarantees and retry: if the endpoint does not return `200 OK`, Paystack retries for up to 72 hours — in live, every 3 minutes for the first 4 attempts then hourly; in test, hourly (https://docs-v2.paystack.com/docs/payments/webhooks/#create-a-webhook-url and go-live checklist).
Dedup: implement `webhook_inbox` unique on `aggregator_event_id` (`data.id` or event id) and return 2xx on duplicate (per `payments-design.md` §5).

Supported events: `charge.success`, `transfer.success`, `transfer.failed`, `transfer.reversed`, `refund.*`, `invoice.*`, `subscription.*`, `paymentrequest.*`, `customeridentification.*`, `dedicatedaccount.assign.*`, `dispute.*` (https://docs-v2.paystack.com/docs/payments/webhooks/#supported-events).
For Orcta Pay, subscribe to `charge.success`, `transfer.success`, `transfer.failed`, `transfer.reversed` at minimum.

Verification endpoint (truth): `GET https://api.paystack.co/transaction/verify/:reference` (https://docs-v2.paystack.com/docs/api/transaction/#verify). Response includes `data.status` (`success`/`failed`/`abandoned`), `data.amount` (pesewas), `data.currency`, `data.gateway_response`, `data.fees`, and `log.history` (https://docs-v2.paystack.com/docs/api/transaction/#verify).
Always verify amount and currency before marking an outbox as `confirmed` — the docs explicitly warn to do so (https://docs-v2.paystack.com/docs/payments/accept-payments/#verify-the-transaction-status).

## Settlement & Reconciliation

Settlement timeline for Ghana: `T+1` or `T+2` business days to the registered Ghanaian bank account. Weekend and public-holiday transactions settle next business day (https://www.mctaba.com/learn/paystack/settlement-and-payouts-on-paystack-in-ghana — secondary; official docs do not state T+ for Ghana).
Official settlement API is currency-agnostic: `GET https://api.paystack.co/settlement` lists settlements with `total_amount`, `effective_amount`, `total_fees`, `settlement_date`, `currency` (https://docs-v2.paystack.com/docs/api/settlement/#list).
Per-settlement transactions: `GET https://api.paystack.co/settlement/:id/transactions` (https://docs-v2.paystack.com/docs/api/settlement/#transactions).
Banks for payout settlement include GCB, Ecobank Ghana, Stanbic, Access, Absa, Fidelity, CalBank (per secondary Ghana guide — confirm the current list via `GET /bank?currency=GHS`).

Reporting / reconciliation: the standing daily job should call `List Settlements` + `Settlement Transactions` and diff against `vendor_ledger_entries` for that period. Invariant is `sum(debits+credits) + sum(commission) + aggregator net = 0` (`payments-design.md` §6) — model settlement delay `T+1`–`T+3` per provider.

Fees: Paystack Ghana fee page is not linked from docs-v2 nav. Secondary research cites card/MoMo collection around 1.95% (plan assumption). Transfer fee for GHS is unpublished. Confirm both with provider and store the effective rate from the `fees` field on verify responses, not from marketing copy.

Ghana-specific notes: Ghanaian MoMo requires `currency: "GHS"` and provider-specific offline authorisation flows (MTN/AirtelTigo = `pay_offline`, Vodafone = `send_otp` with voucher `*110#`) (https://docs-v2.paystack.com/docs/payments/payment-channels/#mobile-money). No Ghana Card verification is exposed via Paystack.

## Credentials & Sandbox

Sandbox vs production separation: Paystack issues `sk_test_...` / `sk_live_...` keys per environment; dashboard toggles between them (https://docs-v2.paystack.com/docs/api/authentication/ — key management on dashboard).
Tests for cards use `4084084084084081` and for banks `0000000000` with bank `057` (Zenith) as shown in examples (https://docs-v2.paystack.com/docs/api/charge/#create).
For Ghana MoMo testing, the docs note "We have some test credentials that can be used to run some tests" but do not list MoMo test phone numbers (https://docs-v2.paystack.com/docs/payments/payment-channels/#mobile-money). Confirm Ghana MoMo test MSISDNs and sandbox success simulation with provider.

IP whitelisting for disbursement: not required for Paystack transfers beyond the standard auth and optional OTP; the transfer-control APIs (`POST /transfer/disable_otp`, etc.) exist for payout automation (https://docs-v2.paystack.com/docs/api/transfer-control/#balance).

KYC: to go live in Ghana requires GRA TIN + business registration + director ID + Ghanaian settlement bank account (secondary guide; matches dashboard KYC expectations). Confirm the current KYC packet on the live dashboard.

## Rate Limits & Errors

Rate limits: not published in docs-v2. Secondary guides do not state Paystack rate limits either. Defensively implement a token-bucket and observe `429` with `Retry-After`.

Errors: HTTP codes `200` (note: charges always return 200 even on failure — inspect `data.status`), `201`, `400`, `401`, `404`, `5xx` (https://docs-v2.paystack.com/docs/api/errors/). A `400` typically carries a validation message. Report `5xx` to Paystack as it indicates a platform error (https://docs-v2.paystack.com/docs/api/errors/).

Retry guidance: on Charge API, do not call `Check Pending Charge` "too early as you may get a lot more pending than you should" — wait ≥10s (https://docs-v2.paystack.com/docs/api/charge/#check). For transfers, no specific retry guidance is published.

## Open Questions to Confirm Directly

- Ghana collection fee (card vs MoMo) and whether it differs from Nigeria's published schedule.
- GHS transfer fee and whether bulk transfers carry a different fee.
- Rate limits per endpoint and whether `Retry-After` is honoured.
- Ghana MoMo test phone numbers and sandbox behaviour for `pay_offline` vs `send_otp`.
- Settlement T+ for Ghana (official confirmation that it is T+1/T+2, not T+3) and cut-off time.

---

# Comparison Table

| Dimension | Hubtel | Moolre | Paystack (Ghana) |
|---|---|---|---|
| Base URL | `https://payproxyapi.hubtel.com` (primary) and `https://api.hubtel.com/v2/` for unified-pay checkout (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | Live `https://api.moolre.com`, Sandbox `https://sandbox.moolre.com` (https://docs.moolre.com/llms.txt) | `https://api.paystack.co` (all environments) (https://docs-v2.paystack.com/docs/api/transaction/) |
| Amount format | Decimal GHS string / number e.g. `50.00` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | Decimal GHS string e.g. `"120.00"` (https://docs.moolre.com/llms-full.txt) | Integer pesewas, 1 GHS = 100 e.g. `50000` for GHS 500 (https://docs-v2.paystack.com/docs/api/transaction/#initialize) |
| Idempotency key | `ClientReference` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | `externalref` (also called `reference` in some endpoints) (https://docs.moolre.com/ai/initiate-payment.md) | `reference` on transaction, `reference` on transfer (https://docs-v2.paystack.com/docs/api/transaction/#initialize) |
| Collections endpoints | `POST /items/initiate` (hosted) and `POST /receive/initiate` (direct USSD) (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | `POST /open/transact/payment` (MoMo USSD) and `POST /embed/link` (hosted Web POS) (https://docs.moolre.com/ai/initiate-payment.md and https://docs.moolre.com/ai/generate-payment-link.md) | `POST /transaction/initialize` (hosted, recommended) and `POST /charge` with `mobile_money` (headless) (https://docs-v2.paystack.com/docs/api/transaction/#initialize and https://docs-v2.paystack.com/docs/api/charge/#create) |
| Callback / return URLs | `PrimaryCallbackUrl` + `callbackUrl`/`returnUrl` per request (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | `callback` (webhook) + `redirect` (return) per payment link; wallet callback per account (https://docs.moolre.com/ai/generate-payment-link.md and https://docs.moolre.com/ai/account-status.md) | `callback_url` per transaction (overrides dashboard) (https://docs-v2.paystack.com/docs/api/transaction/#initialize) |
| Channels (Ghana) | `mtn-gh`, `vodafone-gh`, `airteltigo-gh`, `card`, `ghipss`, `ghqr` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | `13`=MTN, `6`=Telecel, `7`=AT, `2`=bank (https://docs.moolre.com/ai/initiate-payment.md and https://docs.moolre.com/ai/initiate-transfer.md) | `mobile_money` with `mtn`/`vod`/`tgo` (+ `mpesa` for KE) (https://docs-v2.paystack.com/docs/payments/payment-channels/#mobile-money) |
| Auth header | `Authorization: Basic base64(clientId:clientSecret)` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | `X-API-USER` + `X-API-PUBKEY` (collections) / `X-API-KEY` (transfers) (https://docs.moolre.com/ai/initiate-payment.md) | `Authorization: Bearer sk_...` (https://docs-v2.paystack.com/docs/api/authentication/) |
| Disbursement endpoint | `POST /send/money` (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | `POST /open/transact/transfer` (+ `POST /open/transact/validate` pre-check) (https://docs.moolre.com/ai/initiate-transfer.md and https://docs.moolre.com/ai/validate-name.md) | `POST /transferrecipient` then `POST /transfer` (also `POST /transfer/bulk`) (https://docs-v2.paystack.com/docs/api/transfer-recipient/#create and https://docs-v2.paystack.com/docs/api/transfer/#initiate) |
| Bulk payout | Not documented | Dashboard CSV; no batch API published — loop single transfers (https://moolre.com/?product=disbursement and https://docs.moolre.com/ai/initiate-transfer.md) | `POST /transferrecipient/bulk` + `POST /transfer/bulk` (requires OTP disabled) (https://docs-v2.paystack.com/docs/api/transfer-recipient/#bulk and https://docs-v2.paystack.com/docs/api/transfer/#bulk) |
| Payout amount format | Decimal GHS | Decimal GHS string | Pesewas (if `currency=GHS`) |
| Webhook signature | Not published | Not published (use IP + verify) (https://docs.moolre.com/ai/guides/authentication.md and https://docs.moolre.com/llms-full.txt) | `x-paystack-signature` = `HMAC SHA512` over raw body with secret (https://docs-v2.paystack.com/docs/payments/webhooks/#verify-event-origin) |
| Webhook retry | 2xx required within ~30s (https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel) | Not published; idempotent handling required (https://docs.moolre.com/llms-full.txt) | 72h: 3-min ×4 then hourly (live), hourly (test) (https://docs-v2.paystack.com/docs/payments/webhooks/#create-a-webhook-url) |
| Verify / status endpoint | Not published on landing page — confirm | `POST /open/transact/status` with `type`, `idtype`, `id`, `accountnumber` (https://docs.moolre.com/ai/payment-status.md and https://docs.moolre.com/ai/transfer-status.md) | `GET /transaction/verify/:reference` and `GET /transfer/verify/:reference` (https://docs-v2.paystack.com/docs/api/transaction/#verify and https://docs-v2.paystack.com/docs/api/transfer/#verify) |
| Settlement | "Instant" marketed (https://developers.hubtel.com) — confirm T+ | "Daily" marketed (https://moolre.com/) — confirm T+; reconciliation via `POST /open/account/status type:2` (https://docs.moolre.com/ai/list-account-transactions.md) | `T+1`/`T+2` per Ghana secondary guide; official API `GET /settlement` + `GET /settlement/:id/transactions` (https://docs-v2.paystack.com/docs/api/settlement/#list) |
| Ghana-specific verification | Ghana ID / Ghana Card / mobile / bank verify APIs (https://developers.hubtel.com) | None published | None (MoMo is the channel; no ID verify) |
| Sandbox | Not published on landing — confirm | `https://sandbox.moolre.com` (https://docs.moolre.com/llms.txt) | `sk_test_...` on same host (https://docs-v2.paystack.com/docs/api/authentication/) |

---

# References

## Hubtel

- https://developers.hubtel.com — Hubtel For Developers landing (programmatic payments, settlement, verification)
- https://unity.hubtel.com — Hubtel E-Commerce / Unity merchant portal (referenced as checkout SDK host `https://unified-pay.hubtel.com/js/v1/checkout.js` on the landing)
- https://skillsmp.com/creators/africandigitalassetframework/africa-stack-skills/west-africa-ghana-hubtel — Hubtel Payment Gateway skill (auth, base URL `payproxyapi.hubtel.com`, endpoints, channels, webhook notes) — secondary but cross-checked against portal nav
- http://docs-developers.hubtel.com/ — Hubtel API Documentation 1.0.0 (OpenAPI stub)
- https://www.scribd.com/document/801021377/Hubtel-Repeat-Payments-API-v1-2 — Hubtel Recurring Payments API v1.2 (historical, for context only)

## Moolre

- https://docs.moolre.com — Moolre API Documentation 2.0 (interactive docs, Postman collection)
- https://docs.moolre.com/llms.txt — environment list and full index (live `https://api.moolre.com`, sandbox `https://sandbox.moolre.com`)
- https://docs.moolre.com/llms-full.txt — full guides + API reference bundle (idempotency, webhooks, safe retries, PHP SDK, errors)
- https://docs.moolre.com/ai/initiate-payment.md — Mobile Money Collection (`POST /open/transact/payment`)
- https://docs.moolre.com/ai/generate-payment-link.md — Generate Payment Link (`POST /embed/link`)
- https://docs.moolre.com/ai/create-payment-id.md — Create Payment ID (`POST /open/account/create`, type 2)
- https://docs.moolre.com/ai/payment-status.md — Payment Status (`POST /open/transact/status`)
- https://docs.moolre.com/ai/transfer-status.md — Transfer Status (`POST /open/transact/status` for transfers)
- https://docs.moolre.com/ai/initiate-transfer.md — Initiate Transfer (`POST /open/transact/transfer`)
- https://docs.moolre.com/ai/validate-name.md — Validate Name (`POST /open/transact/validate`)
- https://docs.moolre.com/ai/miscellaneous-data.md — Banks list (`GET /open/transact/data`)
- https://docs.moolre.com/ai/account-status.md — Account Status (`POST /open/account/status` type 1 — balance/callback)
- https://docs.moolre.com/ai/list-account-transactions.md — List Transactions (`POST /open/account/status` type 2 — reconciliation)
- https://docs.moolre.com/ai/payment-webhook.md — Payment Webhook (POST to callback)
- https://docs.moolre.com/ai/guides/authentication.md — Callback source IPs (192.241.135.134 and 174.138.44.22)
- https://moolre.com/ — Moolre marketing + product pages (merchant, disbursement, USSD, SMS)
- https://moolre.com/?product=disbursement — Bulk disbursement product page

## Paystack

- https://docs-v2.paystack.com/docs/api/transaction/ — Transactions (initialize, verify, list, totals, export)
- https://docs-v2.paystack.com/docs/api/transaction/#initialize — Initialize Transaction fields and cURL
- https://docs-v2.paystack.com/docs/api/transaction/#verify — Verify Transaction response
- https://docs-v2.paystack.com/docs/api/authentication/ — Bearer auth and key hygiene
- https://docs-v2.paystack.com/docs/api/errors/ — HTTP codes and charge-always-200 note
- https://docs-v2.paystack.com/docs/api/transfer/ — Transfers (initiate, finalize, bulk, fetch, verify)
- https://docs-v2.paystack.com/docs/api/transfer-recipient/ — Transfer Recipients (create, bulk, list, fetch)
- https://docs-v2.paystack.com/docs/api/transfer-control/#balance — Transfer control (OTP, balance)
- https://docs-v2.paystack.com/docs/api/charge/ — Charge API (create, submit PIN/OTP/phone/birthday, check pending)
- https://docs-v2.paystack.com/docs/api/settlement/ — Settlements (list, transactions)
- https://docs-v2.paystack.com/docs/payments/accept-payments/ — Accept Payments (popup, redirect, charge API, verify)
- https://docs-v2.paystack.com/docs/payments/payment-channels/ — Payment Channels (card, bank, USSD, MoMo with GH provider codes `mtn`/`tgo`/`vod`, QR, EFT)
- https://docs-v2.paystack.com/docs/payments/webhooks/ — Webhooks (HMAC SHA512 `x-paystack-signature`, IPs, retry schedule)
- https://docs-v2.paystack.com/docs/payments/verify-payments/ — Verify Payments pattern
- https://paystack.com/docs/api/transaction/ — Mirror of transaction API (older domain)
- https://raw.githubusercontent.com/api-evangelist/paystack/master/apis.yml — Paystack OpenAPI index (apis.json contract)

## Secondary / cross-check only

- https://www.mctaba.com/learn/paystack/paystack-in-ghana-complete-developer-guide — Ghana mobile money landscape, GHS pesewas, bank list
- https://www.mctaba.com/learn/paystack/settlement-and-payouts-on-paystack-in-ghana — GHS payout type `ghipss` vs `mobile_money`, T+1/T+2
- https://www.mctaba.com/learn/paystack/currency-handling-in-paystack-ngn-ghs-zar-kes-and-usd — currency must be explicit
- https://afrilayer.vercel.app/apis/hubtel — Directory listing (rate limit 500/min, Basic Auth)
- https://www.postman.com/paystack-developers/paystack-api/request/89o5om8/initialize-transaction — Postman example (init)
