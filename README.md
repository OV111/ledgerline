# Ledgerline

Non-custodial stablecoin payment gateway for freelancers.
Testnet demo: Ethereum Sepolia, USDC.

Ledgerline never holds funds or private keys. Payments go straight from the
client to the freelancer's wallet; Ledgerline only watches the chain and keeps
the books.

## Layout

| Path | What it is |
| --- | --- |
| `cmd/api` | HTTP API (Go) |
| `cmd/listener` | Watches Sepolia for USDC payments (Go) |
| `internal/ledger` | Double-entry accounting rules |
| `migrations` | PostgreSQL schema |
| `web` | React + TypeScript frontend |
| `docs/project-plan.pdf` | Project plan: scope, design, decisions, timeline |

## Run everything with Docker

```sh
docker compose up --build
curl http://localhost:8080/healthz
```

If port 5432 is already in use on your machine, copy `.env.example` to `.env`
and change `DB_PORT`.

The migrations run only when the database is first created. After changing a
migration, reset the database with `docker compose down -v`.

## Run without Docker

```sh
go run ./cmd/api        # API on http://localhost:8080
go run ./cmd/listener   # listener (heartbeat only for now)

cd web
npm install
npm run dev             # frontend on http://localhost:5173
```

## Before finishing any change

```sh
gofmt -l .      # should print nothing
go vet ./...
go test ./...
```

## Money

All amounts are `int64` USDC units with 6 decimals: `1 USDC = 1_000_000`.
Never use floats for money.
