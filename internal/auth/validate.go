// Package auth holds the rules for user accounts: emails, passwords,
// wallet addresses, and (later) sessions.
//
// This file has only pure functions: no database, no HTTP. That makes every
// rule easy to test on its own.
package auth

import (
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/sha3"
)

// Errors returned to the caller. The HTTP layer turns them into 400 responses.
var (
	ErrInvalidEmail     = errors.New("invalid email address")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 72 bytes")
	ErrInvalidWallet    = errors.New("invalid wallet address")
	ErrWalletChecksum   = errors.New("wallet address checksum is wrong (typo?)")
	ErrZeroWallet       = errors.New("wallet address cannot be the zero address")
)

const (
	minPasswordLen = 8
	// bcrypt only uses the first 72 bytes of a password and silently ignores
	// the rest. We refuse longer passwords instead of pretending to use them.
	maxPasswordBytes = 72
	// 254 is the maximum length of an email address in practice (RFC 5321).
	maxEmailLen = 254
)

// NormalizeEmail trims spaces, lowercases the email and checks its format.
// It returns the cleaned email, which is what we store and compare.
// Lowercasing means "Vahe@X.com" and "vahe@x.com" are the same account.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(email) > maxEmailLen {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	// ParseAddress also accepts `Name <a@b.com>`. We only want the bare
	// address, so the parsed result must be exactly what the user typed.
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// ValidatePassword checks the password length.
// Length is measured in bytes because that is what bcrypt counts
// (a letter like "ա" is 2 bytes).
func ValidatePassword(password string) error {
	if len(password) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	// For the minimum we count characters (runes), which is what a user expects.
	if len([]rune(password)) < minPasswordLen {
		return ErrPasswordTooShort
	}
	return nil
}

// NormalizeWallet checks an Ethereum address and returns it in checksum form
// (EIP-55), which is what we store.
//
// A typo in the address would send the freelancer's money to a stranger
// forever, so we are strict here:
//   - must be "0x" + 40 hex characters
//   - if the user typed mixed case, the EIP-55 checksum must be right
//     (that catches most typos)
//   - all-lowercase or all-uppercase has no checksum, so we accept it
//   - the zero address 0x000...0 is refused (nobody owns it; money sent
//     there is burned)
func NormalizeWallet(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
		return "", ErrInvalidWallet
	}
	body := addr[2:]
	if _, err := hex.DecodeString(body); err != nil {
		return "", ErrInvalidWallet
	}
	if strings.Trim(body, "0") == "" {
		return "", ErrZeroWallet
	}

	checksummed := toChecksum(body)
	hasCase := body != strings.ToLower(body) && body != strings.ToUpper(body)
	if hasCase && addr != checksummed {
		return "", ErrWalletChecksum
	}
	return checksummed, nil
}

// toChecksum builds the EIP-55 form of a 40-character hex address.
//
// How EIP-55 works: hash the lowercase address with Keccak-256. For each
// letter (a-f) in the address, look at the hex digit at the same position in
// the hash. If that digit is 8 or more, the letter becomes uppercase.
// So the capital letters act as a built-in checksum.
func toChecksum(body string) string {
	lower := strings.ToLower(body)

	// Ethereum uses the original Keccak-256, not the final SHA3-256 standard.
	// They differ slightly, so NewLegacyKeccak256 is the correct one here.
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(lower))
	hash := hex.EncodeToString(h.Sum(nil))

	out := []byte(lower)
	for i, c := range out {
		if c >= 'a' && c <= 'f' && hash[i] >= '8' {
			out[i] = c - 'a' + 'A'
		}
	}
	return "0x" + string(out)
}
