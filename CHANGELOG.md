# Changelog

All notable changes to this project are written here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Project skeleton: Go module, `cmd/api` (health check only), `cmd/listener` (heartbeat only).
- `internal/ledger.Validate`: double-entry balance check with tests.
- `migrations/001_init.sql`: ledger tables, balance trigger, append-only trigger.
- Docker Compose stack (PostgreSQL 16, api, listener) and Dockerfile.
- React + TypeScript app skeleton in `web/` (Vite).
- Project plan PDF in `docs/project-plan.pdf`.
- README now contains the full project plan (from the PDF) plus setup steps.
- GitHub Actions CI: gofmt, `go vet`, `go test`, web build.
- Stage 1 docs: `docs/prd.md` (frozen scope), `docs/design.md` (architecture, unique amount, ledger, security), ADRs 001–006.
- Accounts: `POST /signup`, `/login`, `/logout`, `GET /me`, `PUT /me/wallet` (needs password, logs out other sessions).
- `internal/auth`: email/password/wallet (EIP-55) validation, bcrypt hashing, sessions stored as SHA-256 hashes.
- `migrations/002_users.sql`: `users` (unique email and wallet) and `sessions` tables. Local DB needs `docker compose down -v` once.
- `internal/ratelimit`: max 10 password attempts per IP per minute (429 + `Retry-After`).
- Dependencies: `golang.org/x/crypto` v0.31.0 (bcrypt, Keccak), `github.com/jackc/pgx/v5` v5.7.1 (PostgreSQL driver).

### Changed
- Own port block to avoid clashes: API 8090, PostgreSQL 5440, test DB 5441, web 5180. All bound to `127.0.0.1` (API was open to the network on `0.0.0.0:8080`).
