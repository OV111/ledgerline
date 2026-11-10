# ADR 005: Sepolia testnet only for v1

Status: accepted (Nov 11, 2026)

## Context
The project is a demo with a fixed deadline (Nov 26, 2026). Real money would
need audits, monitoring and legal review that we cannot do in this time.

## Decision
v1 runs only on Ethereum Sepolia with testnet USDC. Every public page and the
README say "testnet only". No mainnet code path exists.
If testnet USDC is hard to get, we deploy our own test ERC-20 token.

## Trade-off
- Good: zero risk of losing real money, free tokens for testing.
- Bad: not a production product yet. Mainnet would be a separate project.
