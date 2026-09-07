---
title: ADR-034
description: Orcta Pay is a service with an API, not a shared library — one webhook per gateway, product attribution via internal reference
status: accepted
date: 2026-08-29
owner: backend
impact: high
supersedes: null
---

## Context

Orcta will have two products calling payments soon (OrctaGo and Orcta POS). Today `internal/payments` lives inside OrctaGo and each product would copy Hubtel/Paystack + ledger logic. That duplicates fixes N times and splits the source of truth for payment state. POS kickoff is imminent.

## Decision

Orcta Pay is a standalone internal service with a unified API. Products integrate via thin per-language client SDKs. Gateway logic lives in one deployable. There is one webhook endpoint per gateway, never per product. Attribution uses Orcta Pay's reference `optd-{product}-{gateway}-{ulid}` passed as the gateway's `reference`/`clientReference`. Gateway-side routing is not relied upon.

Gateway adapters sit behind a sealed interface `Initiate / Verify / Refund / Payout → ChargeResult` (mirroring OrctaGo's `ChargeResult`). Money is integer pesewas. Webhook ingestion writes to a transactional outbox in the same transaction as the ledger update; fan-out to the owning product is async from the outbox.

## Why

- One place to fix, one source of truth for ledger and analytics, no N× duplication across stacks.
- One webhook per gateway is simpler to operate and to verify (single signature surface). Product attribution stays in Orcta Pay, not in gateway config, so adding a product does not touch gateway routing.
- Routing lives behind the service: eligibility (channel/min/max/currency), ranking (success-rate floor → cost tiebreak, Valkey for rolling metrics, circuit-breaker exclusion), and failover (sync retry next gateway, async webhook timeout via `GetTransactionStatus`). Products see `POST /charges` → intent, regardless of gateway.

## Revisit if

A product needs a payment flow that cannot be expressed through `Initiate / Verify / Payout` (e.g. PCI-scoped card handling off-service), or the service becomes a single point of failure that outweighs duplication. Until then, OrctaGo's current `internal/payments` is the future SDK shape — swap its Hubtel/Paystack providers for an `OrctaPayClient` with no API change to `orders`.
