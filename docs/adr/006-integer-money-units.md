# ADR 006: Integer money units, no floats

Status: accepted (Nov 11, 2026)

## Context
Floating-point numbers cannot store most decimals exactly
(`0.1 + 0.2 != 0.3`). In finance, tiny rounding errors add up and break
balances. Matching a payment by exact amount (ADR 002) makes this even more
important.

## Decision
Store all money as `int64` USDC units with 6 decimals: `1 USDC = 1_000_000`.
In the database this is `BIGINT`. This is the same precision as USDC on-chain,
so no conversion is needed between our data and the blockchain.
Floats are never used for money. Decimal strings are created only for display.

## Trade-off
- Good: exact arithmetic, easy equality checks, no rounding bugs.
- Bad: we must convert to and from a readable form (for example `100.000417`)
  at the edges: user input and display.
