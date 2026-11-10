# Ledgerline — Design

How the system works: architecture, data, accounting, security. Scope is in
[prd.md](prd.md); the reasons behind big choices are in [adr/](adr/).

## 1. Architecture

```
Dashboard (React) ──► Go API ──► PostgreSQL ◄── Chain listener (Go) ◄── Sepolia
Payment page (React) ─┘                                  │
        │                                                └──► signed webhook ──► merchant
        └── client pays USDC with MetaMask ──► Sepolia ──► freelancer's wallet
```

- **API** (`cmd/api`): login, invoices, signed payment links. Writes to the DB.
- **Listener** (`cmd/listener`): reads the chain, matches transfers to
  invoices, writes payments and ledger entries, sends webhooks.
- **PostgreSQL**: single source of truth. API and listener never talk to each
  other directly; they share the database.
- **We never touch the money.** The client's USDC goes straight to the
  freelancer's wallet. We only read the chain.

## 2. Money rules

- All amounts are `int64` in USDC units: `1 USDC = 1_000_000` (6 decimals).
- No floats anywhere. Convert to a decimal string only when displaying.
- USDC on Sepolia has 6 decimals, so one unit on-chain = one unit in our DB.

## 3. Unique amount (how a payment finds its invoice)

All payments arrive at the same wallet, so the amount identifies the invoice.

**Generation.** When an invoice is created:

```
suffix        = random number from 1 to 9_999        (0.000001 – 0.009999 USDC)
unique_amount = amount + suffix
```

Example: a 100 USDC invoice becomes `100_000_417` units = 100.000417 USDC.

**Collision rule.** Two *open* invoices of the same freelancer must never have
the same `unique_amount`. This is enforced by the database (partial unique
index, see §4), not only by code. If the random suffix collides, the API picks
another one and retries (max 10 tries, then returns an error).

Once an invoice is paid or voided, its amount can be reused.

**Why the suffix is small.** The client pays at most 0.01 USDC extra. That is
the trade-off from ADR 002.

**Limit.** At most 9_999 open invoices per freelancer at once. Fine for a demo.

## 4. Data model

Money columns are `BIGINT` units. Timestamps are `TIMESTAMPTZ`.
`ledger_transactions` and `ledger_entries` already exist in
`migrations/001_init.sql`; the others come with their features.

### users (stage 5; stage 2 uses one demo merchant)
| Column | Notes |
| --- | --- |
| id | primary key |
| email | unique |
| password_hash | bcrypt, never the password |
| wallet_address | where the freelancer gets paid |
| created_at | |

### invoices (stage 2)
| Column | Notes |
| --- | --- |
| id | primary key |
| user_id | owner |
| client_name | text |
| amount | what the freelancer asked for |
| unique_amount | `amount + suffix`; what the client must pay |
| status | `pending`, `paid`, `voided` |
| due_date | |
| link_signature | HMAC of id + unique_amount + wallet (see §7) |
| created_at | |

Constraints:
- `amount > 0`, `unique_amount > amount`
- `UNIQUE (user_id, unique_amount) WHERE status = 'pending'`: the collision
  rule from §3

### payments (stage 3)
| Column | Notes |
| --- | --- |
| id | primary key |
| invoice_id | the matched invoice |
| tx_hash | **unique**: one blockchain transaction is recorded once |
| log_index | position of the Transfer event inside that transaction |
| block_number | block where it was included |
| status | `pending`, `confirmed` (see §5) |
| created_at | |

### ledger_transactions / ledger_entries (stage 2, done)
See §6 and `migrations/001_init.sql`.

### webhook_deliveries (stage 5)
| Column | Notes |
| --- | --- |
| id, invoice_id | |
| url | merchant's endpoint |
| attempt | how many tries so far |
| status | `pending`, `sent`, `failed` |
| next_retry_at | when to try again |

## 5. Payment detection (listener)

The listener polls Sepolia (not subscribe; polling is simpler and survives RPC
drops, see the risk table in the README).

Loop, every few seconds:

1. Ask the RPC for the latest block number.
2. Fetch `Transfer` logs from the USDC contract for the new block range.
3. For each log, **ignore it** unless all three agree:
   - the log came from the **exact USDC contract address** (blocks fake tokens)
   - `to` equals the freelancer's wallet
   - `value` equals the `unique_amount` of a `pending` invoice
4. Insert a `payments` row with status `pending`. `tx_hash` is unique, so seeing
   the same log twice does nothing (idempotent).
5. On each loop, for `pending` payments: if `latest - block_number >= 6`,
   re-check the transaction is still in the chain, then **confirm**.
6. Confirm = in **one database transaction**: payment → `confirmed`, invoice →
   `paid`, ledger entry written.

**Reorg handling.** A reorg means the chain replaced recent blocks. If a
`pending` payment's transaction is no longer in the chain, we delete that
payment row and the invoice stays `pending` (if the client's transaction
lands again in a new block, step 4 simply records it again). Because we wait 6 blocks
before booking money, nothing was written to the ledger yet.

**Restart safety.** The listener stores the last block it scanned. After a
restart it continues from there, and idempotency (unique `tx_hash`) protects
against re-reading.

## 6. Accounting (double-entry ledger)

Three accounts: `accounts_receivable`, `revenue`, `usdc_wallet`.
Every ledger transaction must balance: **debits = credits**.

| Event | Debit | Credit | Amount |
| --- | --- | --- | --- |
| Invoice issued | accounts_receivable | revenue | `unique_amount` |
| Payment confirmed | usdc_wallet | accounts_receivable | `unique_amount` |
| Invoice voided | revenue | accounts_receivable | `unique_amount` |

**We book `unique_amount`, not `amount`**, because that is the real number of
USDC that arrives on-chain. This keeps the ledger's wallet balance equal to
the chain, which reconciliation needs.

**Checked twice:**
1. In Go: `internal/ledger.Validate` before writing.
2. In the database: a deferred trigger rejects an unbalanced transaction at
   COMMIT.

**Append-only.** Entries are never edited or deleted (a trigger forbids it). To
fix a mistake we add a reversing transaction. That is exactly how voiding works.

**Reconciliation (first to cut).** Daily: sum of `usdc_wallet` in the ledger vs.
sum of on-chain transfers to the wallet. Any difference is flagged.

## 7. Security

| Threat | Defense |
| --- | --- |
| Client changes amount or address in the link | Payment link carries an HMAC signature of id + unique_amount + wallet; API rejects a bad one |
| Fake token with same name | Listener checks the exact USDC contract address |
| Same payment counted twice | `tx_hash` unique, plus `log_index` |
| Payment undone by a reorg | Wait 6 confirmations before booking |
| Wrong or lost amount | Exact match only; anything else is ignored |
| Unbalanced books | Go check + DB trigger + tests |
| Rewriting history | Ledger is append-only (trigger) |
| Stolen passwords | bcrypt hashes; no plain passwords stored |
| Leaked secrets | Secrets only in environment variables; none in the repo |
| Fake webhooks | Webhook body signed with HMAC; merchant verifies it |
| One user reads another's invoices | Every query filters by `user_id` |
| Lost funds / stolen keys | **Not possible: we hold no funds and no private keys** (ADR 001) |

The payment link is signed with a server secret (`LINK_SECRET`), and the
webhook with a per-merchant secret. Both live in environment variables.

## 8. Open points

None blocking Stage 2. Stage 3 will confirm the exact RPC provider and the
Sepolia USDC contract address (or a test token if none is available).
