# ADR 003: Go for the API and the listener

Status: accepted (Nov 11, 2026)

## Context
We need an HTTP API and a long-running process that reads the blockchain. Both
must be reliable and handle many things at once.

## Decision
Write both in Go (`cmd/api`, `cmd/listener`). Go is fast, has strong
concurrency, a good standard library (HTTP, crypto, SQL), compiles to one small
binary, and matches the author's earlier fintech work.

## Trade-off
- Good: simple deployment, few dependencies, easy to run the listener forever.
- Bad: more code than Node for plain CRUD endpoints.
