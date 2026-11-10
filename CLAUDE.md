# Ledgerline — project context

Non-custodial stablecoin payment gateway for freelancers. Testnet demo (Ethereum Sepolia, USDC). Deadline Nov 26, 2026.

## Stack
Go 1.22 (API in cmd/api, listener in cmd/listener), PostgreSQL 16, React + TypeScript (web/), Docker Compose, GitHub Actions.

## Rules
- Never hold funds or private keys. Payments go straight to the freelancer's wallet.
- Money = int64 USDC units (6 decimals). Never floats.
- Every ledger transaction must balance (internal/ledger.Validate + DB trigger in migrations/001_init.sql).
- Money logic must have tests. Run `go vet ./...` and `go test ./...` before finishing.
- Prefer the standard library; ask before adding a dependency.
- One feature per branch, Conventional Commits, update CHANGELOG.md.
- Keep code simple and readable: I am a junior developer and must understand everything. Explain non-obvious parts in short comments.

## Docs
docs/prd.md (scope), docs/design.md (architecture, accounting, security), docs/adr/ (decisions). Don't change scope without asking.
