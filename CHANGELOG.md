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
