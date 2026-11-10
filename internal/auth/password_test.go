package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCheckPassword(t *testing.T) {
	const pw = "correct-horse-battery"

	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !CheckPassword(hash, pw) {
		t.Error("right password was rejected")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("wrong password was accepted")
	}
	// Passwords are case sensitive.
	if CheckPassword(hash, strings.ToUpper(pw)) {
		t.Error("password with different case was accepted")
	}
	// The hash must never contain the password itself.
	if strings.Contains(hash, pw) {
		t.Error("hash contains the plain password")
	}
	if cost, _ := bcrypt.Cost([]byte(hash)); cost != bcryptCost {
		t.Errorf("cost = %d, want %d", cost, bcryptCost)
	}
}

func TestHashPasswordUsesSalt(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")
	if h1 == h2 {
		t.Error("same password gave the same hash: no salt?")
	}
}

func TestHashPasswordRejectsBadPassword(t *testing.T) {
	if _, err := HashPassword("short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("short: got %v, want ErrPasswordTooShort", err)
	}
	if _, err := HashPassword(strings.Repeat("a", 73)); !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("long: got %v, want ErrPasswordTooLong", err)
	}
}

func TestCheckPasswordUnknownUser(t *testing.T) {
	// Empty hash = user not found. Must be false, even for the dummy's own password.
	if CheckPassword("", "dummy-password-never-matches") {
		t.Error("unknown user was accepted")
	}
}

func TestCheckPasswordGarbageHash(t *testing.T) {
	// A broken value in the database must never let someone in.
	if CheckPassword("not-a-bcrypt-hash", "anything") {
		t.Error("garbage hash was accepted")
	}
}
