# ADR 001: Non-custodial — never hold funds or keys

Status: accepted (Nov 11, 2026)

## Context
A gateway that holds client money or private keys becomes the biggest target
for hackers and brings legal duties (licenses, safekeeping).

## Decision
Payments go straight from the client to the freelancer's own wallet. We store
only the wallet's public address. No private keys, no pooled funds. We only
watch the blockchain and record what happened.

## Trade-off
- Good: nothing to steal, much smaller legal risk, simpler to build.
- Bad: we cannot offer escrow or refunds. A refund is between the freelancer
  and the client.
