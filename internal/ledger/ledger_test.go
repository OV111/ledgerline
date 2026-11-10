package ledger

import (
	"errors"
	"math"
	"testing"
)

func TestValidate(t *testing.T) {
	// Each case is one transaction and the error we expect (nil = valid).
	tests := []struct {
		name    string
		entries []Entry
		wantErr error
	}{
		{
			name: "invoice issued",
			entries: []Entry{
				{Account: AccountsReceivable, Debit: 100_000_417}, // 100.000417 USDC
				{Account: Revenue, Credit: 100_000_417},
			},
			wantErr: nil,
		},
		{
			name: "payment confirmed",
			entries: []Entry{
				{Account: USDCWallet, Debit: 100_000_417},
				{Account: AccountsReceivable, Credit: 100_000_417},
			},
			wantErr: nil,
		},
		{
			name: "invoice voided",
			entries: []Entry{
				{Account: Revenue, Debit: 100_000_417},
				{Account: AccountsReceivable, Credit: 100_000_417},
			},
			wantErr: nil,
		},
		{
			name: "balanced split across three entries",
			entries: []Entry{
				{Account: USDCWallet, Debit: 100_000_000},
				{Account: AccountsReceivable, Credit: 99_000_000},
				{Account: AccountsReceivable, Credit: 1_000_000},
			},
			wantErr: nil,
		},
		{
			name: "smallest unit balances",
			entries: []Entry{
				{Account: USDCWallet, Debit: 1}, // 0.000001 USDC
				{Account: AccountsReceivable, Credit: 1},
			},
			wantErr: nil,
		},
		{
			name:    "no entries",
			entries: nil,
			wantErr: ErrTooFewEntries,
		},
		{
			name: "single entry",
			entries: []Entry{
				{Account: USDCWallet, Debit: 5},
			},
			wantErr: ErrTooFewEntries,
		},
		{
			name: "unbalanced by one unit",
			entries: []Entry{
				{Account: USDCWallet, Debit: 25_000_000},
				{Account: AccountsReceivable, Credit: 24_999_999},
			},
			wantErr: ErrUnbalanced,
		},
		{
			name: "only debits",
			entries: []Entry{
				{Account: USDCWallet, Debit: 10},
				{Account: AccountsReceivable, Debit: 10},
			},
			wantErr: ErrUnbalanced,
		},
		{
			name: "entry with no amount",
			entries: []Entry{
				{Account: USDCWallet},
				{Account: AccountsReceivable},
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "entry with both debit and credit",
			entries: []Entry{
				{Account: USDCWallet, Debit: 10, Credit: 10},
				{Account: AccountsReceivable, Credit: 10},
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "negative debit",
			entries: []Entry{
				{Account: USDCWallet, Debit: -10},
				{Account: AccountsReceivable, Credit: 10},
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "negative credit",
			entries: []Entry{
				{Account: USDCWallet, Debit: 10},
				{Account: AccountsReceivable, Credit: -10},
			},
			wantErr: ErrInvalidAmount,
		},
		{
			name: "unknown account",
			entries: []Entry{
				{Account: "piggy_bank", Debit: 10},
				{Account: AccountsReceivable, Credit: 10},
			},
			wantErr: ErrUnknownAccount,
		},
		{
			name: "empty account",
			entries: []Entry{
				{Account: "", Debit: 10},
				{Account: AccountsReceivable, Credit: 10},
			},
			wantErr: ErrUnknownAccount,
		},
		{
			name: "debit total overflows int64",
			entries: []Entry{
				{Account: USDCWallet, Debit: math.MaxInt64},
				{Account: USDCWallet, Debit: 1},
				{Account: AccountsReceivable, Credit: math.MaxInt64},
			},
			wantErr: ErrOverflow,
		},
		{
			name: "credit total overflows int64",
			entries: []Entry{
				{Account: USDCWallet, Debit: math.MaxInt64},
				{Account: AccountsReceivable, Credit: math.MaxInt64},
				{Account: AccountsReceivable, Credit: 1},
			},
			wantErr: ErrOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(Transaction{Description: tt.name, Entries: tt.entries})

			// errors.Is also matches when the error was wrapped with extra text.
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
