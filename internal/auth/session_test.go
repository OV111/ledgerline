package auth

import (
	"bytes"
	"testing"
)

func TestNewSessionToken(t *testing.T) {
	tok1, hash1, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	tok2, _, _ := newSessionToken()

	if tok1 == tok2 {
		t.Error("two tokens are equal: not random")
	}
	// 32 bytes in base64 without padding = 43 characters.
	if len(tok1) != 43 {
		t.Errorf("token length = %d, want 43", len(tok1))
	}
	if len(hash1) != 32 {
		t.Errorf("hash length = %d, want 32 (matches the DB CHECK)", len(hash1))
	}
	// The hash must be reproducible from the token, or we couldn't look it up.
	if !bytes.Equal(hash1, hashToken(tok1)) {
		t.Error("hashToken(token) does not match the stored hash")
	}
	if bytes.Equal(hash1, []byte(tok1)) {
		t.Error("hash equals the token: not hashed")
	}
}
