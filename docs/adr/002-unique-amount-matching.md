# ADR 002: Unique amount per invoice for matching

Status: accepted (Nov 11, 2026)

## Context
All payments for a freelancer arrive at one wallet. A blockchain transfer has no
"invoice number" field, so we need another way to know which invoice was paid.

## Decision
Each invoice gets a unique amount: the real amount plus a small random suffix
(1 to 9,999 units, up to 0.009999 USDC). Example: 100 USDC becomes 100.000417.
The listener matches a transfer by exact amount. The database guarantees that
two open invoices of one freelancer never share an amount.
Details: [design.md](../design.md) section 3.

## Trade-off
- Good: works with one wallet and no smart contract.
- Bad: the client pays a tiny fraction extra, and a freelancer can have at most
  9,999 open invoices at a time. Fine for a demo.
