# Ledgerline — Product Requirements (PRD)

Status: **scope frozen** (Stage 1, Nov 11, 2026). Changing anything below needs
an explicit decision; new ideas go to the v2 list at the end.

> Testnet only: Ethereum Sepolia, testnet USDC. No real money.

## 1. Problem

Freelancers in many countries (Armenia included) can't fully use Stripe or
PayPal. International bank transfers take days and cost 3–7% in fees and FX
spread.

## 2. Solution

A **non-custodial** payment gateway. The freelancer creates an invoice and
shares a link. The client pays USDC straight to the freelancer's own wallet.
We only *watch* the blockchain, mark the invoice paid, and record it in a
double-entry ledger. We never hold funds or private keys.

## 3. Users

| User | Goal |
| --- | --- |
| Freelancer (merchant) | Create invoices, share links, see what was paid |
| Client (payer) | Open a link and pay with a crypto wallet (MetaMask) |

## 4. In scope (v1)

1. Sign up, log in, save wallet address
2. Create, list and view invoices
3. Public payment page with MetaMask payment
4. On-chain payment detection with confirmations
5. Double-entry ledger and reconciliation view
6. Webhook to notify the merchant's system

## 5. Out of scope (v1)

Real money, mainnet, other tokens or chains, refunds, fiat conversion, teams,
mobile app, KYC.

## 6. User stories and acceptance criteria

Each story is "done" only when every criterion is true.

### US-1 Account (stage 5 for login, stage 2 uses a demo merchant)
As a freelancer, I can sign up, log in and save my wallet address.
- Password is stored hashed (bcrypt), never in plain text.
- Wallet address is validated (valid Ethereum address format).

### US-2 Create an invoice (stage 2)
As a freelancer, I can create an invoice for a client.
- I enter client name, amount in USDC and due date.
- The system gives the invoice a **unique amount** (e.g. 100.000417 USDC) so
  the payment can be matched to it.
- The amount is stored as an integer in USDC units (1 USDC = 1_000_000).
- Creating the invoice writes a balanced ledger entry (Accounts Receivable /
  Revenue).
- I get a signed payment link.

### US-3 List and view invoices (stage 2 API, stage 4 UI)
As a freelancer, I can see all my invoices and their status
(pending / paid / voided).

### US-4 Pay an invoice (stage 4)
As a client, I can open the payment link and pay with MetaMask.
- The page shows the exact amount, token and wallet to pay.
- A link with a changed amount or address is rejected (signature check).
- Funds go directly to the freelancer's wallet.

### US-5 Payment detection (stage 3)
As a freelancer, my invoice is marked paid automatically after the client pays.
- Only a transfer with the **right token contract, right wallet and exact
  amount** counts. Anything else is ignored.
- The invoice becomes paid only after **6 block confirmations**.
- If a reorg drops the transaction before that, the payment goes back to
  pending.
- The same `tx_hash` can never be recorded twice.
- Marking paid writes a balanced ledger entry (USDC Wallet / Accounts
  Receivable).

### US-6 Notification (stage 5)
As a freelancer, my own system gets a signed webhook when an invoice is paid.
- The webhook is signed so I can verify it came from Ledgerline.

### US-7 Reconciliation (stage 3–5, first to cut)
As a freelancer, I can compare the ledger's USDC Wallet balance with on-chain
transfers and see any difference.

### US-8 Void an invoice (stage 2, cut list)
As a freelancer, I can void an unpaid invoice.
- Voiding adds a reversing ledger entry; old entries are never edited.

## 7. Success metrics

| Metric | Target |
| --- | --- |
| End-to-end test payment detected | under 2 minutes |
| Ledger balanced (debits = credits) | 100% of entries, enforced by test |
| Demo flow passes | 10 runs in a row without failure |
| Live public demo | 1 URL anyone can open |

## 8. Constraints

- Deadline: launch **Nov 26, 2026**. Hard gate: payment end to end by **Nov 15**.
- About 2–3 hours of work per day.
- Money is `int64` USDC units. Never floats.
- Prefer the standard library; adding a dependency needs approval.

## 9. Cut list (if the Nov 15 gate slips)

Cut from the top first:
1. Reconciliation job (show a manual check instead)
2. Webhooks with retries (keep a single send)
3. Invoice voiding
4. Login with sessions (use a single demo merchant)

## 10. v2 list (parked ideas)

New ideas go here, not into v1. Empty for now.
