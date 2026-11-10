# ADR 004: PostgreSQL with a double-entry ledger

Status: accepted (Nov 11, 2026)

## Context
We record money movements. Mistakes here are the worst kind of bug, and we must
be able to explain every number.

## Decision
Use PostgreSQL 16 and record every money event as a double-entry ledger
transaction: total debits must equal total credits. This is checked in Go
(`internal/ledger.Validate`) and again by a database trigger. The ledger is
append-only; mistakes are fixed with a reversing transaction, never an edit.
Marking an invoice paid and writing its ledger entry happen in one database
transaction (all or nothing).

## Trade-off
- Good: ACID guarantees, the database refuses unbalanced data even if the Go
  code has a bug, full audit history.
- Bad: stricter schema and more careful work up front.
