package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"simple", "vahe@example.com", "vahe@example.com", false},
		{"uppercase becomes lowercase", "Vahe@Example.COM", "vahe@example.com", false},
		{"spaces trimmed", "  vahe@example.com  ", "vahe@example.com", false},
		{"empty", "", "", true},
		{"no at sign", "vahe.example.com", "", true},
		{"display name not allowed", "Vahe <vahe@example.com>", "", true},
		{"too long", strings.Repeat("a", 250) + "@x.com", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeEmail(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"ok", "correct-horse", nil},
		{"exactly 8", "12345678", nil},
		{"7 is too short", "1234567", ErrPasswordTooShort},
		{"exactly 72 bytes", strings.Repeat("a", 72), nil},
		{"73 bytes too long", strings.Repeat("a", 73), ErrPasswordTooLong},
		// "ա" is 2 bytes: 8 characters = 16 bytes, fine.
		{"8 Armenian letters", strings.Repeat("ա", 8), nil},
		// 37 x 2 bytes = 74 bytes: over the bcrypt limit even though only 37 characters.
		{"multibyte over 72 bytes", strings.Repeat("ա", 37), ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePassword(tt.in); !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNormalizeWallet(t *testing.T) {
	// Official test vector from the EIP-55 specification.
	const good = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"correct checksum", good, good, nil},
		{"all lowercase is accepted and checksummed", strings.ToLower(good), good, nil},
		{"all uppercase is accepted", "0x" + strings.ToUpper(good[2:]), good, nil},
		{"spaces trimmed", "  " + good + " ", good, nil},
		// Same address with one letter's case flipped (a -> A): a typo.
		{"wrong checksum", "0x5AAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", "", ErrWalletChecksum},
		{"missing 0x", good[2:], "", ErrInvalidWallet},
		{"too short", good[:41], "", ErrInvalidWallet},
		{"not hex", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeZ", "", ErrInvalidWallet},
		{"zero address", "0x" + strings.Repeat("0", 40), "", ErrZeroWallet},
		{"empty", "", "", ErrInvalidWallet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeWallet(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
