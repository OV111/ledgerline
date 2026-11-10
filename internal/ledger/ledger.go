// Package ledger holds the double-entry accounting rules.
//
// Double-entry in one sentence: every transaction writes the same amount
// twice, once as a debit on one account and once as a credit on another.
// If the two sides are not equal, money was created or lost by a bug, so we
// reject the transaction.
//
// The accounting entries we use (from the project plan):
//
//	Event              Debit                 Credit
//	Invoice issued     Accounts Receivable   Revenue
//	Payment confirmed  USDC Wallet           Accounts Receivable
//	Invoice voided     Revenue               Accounts Receivable
package ledger

import (
	"errors"
	"fmt"
	"math"
)

// Account is the name of a ledger account.
type Account string

// The only accounts that exist in v1. The database has the same list
// (see the CHECK on ledger_entries.account in migrations/001_init.sql).
const (
	AccountsReceivable Account = "accounts_receivable" // money clients still owe
	Revenue            Account = "revenue"             // money earned by issuing invoices
	USDCWallet         Account = "usdc_wallet"         // USDC that arrived in the freelancer's wallet
)

// Entry is one line of a transaction.
//
// Amounts are USDC units with 6 decimals: 1 USDC = 1_000_000.
// Exactly one of Debit and Credit is set; the other one is 0.
type Entry struct {
	Account Account
	Debit   int64
	Credit  int64
}

// Transaction is a group of entries that must balance together.
type Transaction struct {
	Description string
	Entries     []Entry
}

// Errors returned by Validate. Callers can check them with errors.Is.
var (
	ErrTooFewEntries  = errors.New("ledger: a transaction needs at least two entries")
	ErrUnknownAccount = errors.New("ledger: unknown account")
	ErrInvalidAmount  = errors.New("ledger: an entry needs exactly one of debit or credit, greater than zero")
	ErrOverflow       = errors.New("ledger: total amount is too large")
	ErrUnbalanced     = errors.New("ledger: debits and credits are not equal")
)

// Validate checks that a transaction is safe to save.
// It returns nil when the transaction is valid.
//
// The database checks the balance again (see migrations/001_init.sql), so a
// bug in one place is still caught by the other.
func Validate(tx Transaction) error {
	if len(tx.Entries) < 2 {
		return ErrTooFewEntries
	}

	var debits, credits int64

	for i, e := range tx.Entries {
		switch e.Account {
		case AccountsReceivable, Revenue, USDCWallet:
			// known account
		default:
			return fmt.Errorf("entry %d: %w: %q", i, ErrUnknownAccount, e.Account)
		}

		// Valid shapes are (debit > 0, credit == 0) or (debit == 0, credit > 0).
		isDebit := e.Debit > 0 && e.Credit == 0
		isCredit := e.Debit == 0 && e.Credit > 0
		if !isDebit && !isCredit {
			return fmt.Errorf("entry %d: %w", i, ErrInvalidAmount)
		}

		// int64 silently wraps around to a negative number when it gets too
		// big, so we check there is still room before adding.
		if e.Debit > math.MaxInt64-debits || e.Credit > math.MaxInt64-credits {
			return fmt.Errorf("entry %d: %w", i, ErrOverflow)
		}
		debits += e.Debit
		credits += e.Credit
	}

	if debits != credits {
		return fmt.Errorf("%w: debits=%d credits=%d", ErrUnbalanced, debits, credits)
	}
	return nil
}
