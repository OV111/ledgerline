package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql
)

// These tests need a real PostgreSQL with the migrations applied.
// They run only when TEST_DATABASE_URL is set, and are skipped otherwise
// (for example in CI, which has no database yet).
//
// WARNING: the tests delete all users and sessions. Point TEST_DATABASE_URL
// only at a throwaway database, never at one with data you want to keep.
func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`TRUNCATE users, sessions`); err != nil {
		t.Fatal(err)
	}
	return NewStore(db)
}

// randomWallet returns a fresh valid address, so tests never collide.
func randomWallet(t *testing.T) string {
	b := make([]byte, 20)
	rand.Read(b)
	w, err := NormalizeWallet("0x" + hex.EncodeToString(b))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// fakeHash is a value that passes the DB CHECK (bcrypt format, 60 chars).
// Real hashing is tested in password_test.go; here it would only be slow.
var fakeHash = "$2a$12$" + strings.Repeat("x", 53)

func TestCreateAndFindUser(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	w := randomWallet(t)

	u, err := s.CreateUser(ctx, "vahe@example.com", fakeHash, w)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.UserByEmail(ctx, "vahe@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID || got.Wallet != w || got.PasswordHash != fakeHash {
		t.Errorf("got %+v, want %+v", got, u)
	}

	if _, err := s.UserByEmail(ctx, "nobody@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown email: got %v, want ErrNotFound", err)
	}
}

func TestCreateUserDuplicates(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	w := randomWallet(t)
	if _, err := s.CreateUser(ctx, "a@example.com", fakeHash, w); err != nil {
		t.Fatal(err)
	}

	_, err := s.CreateUser(ctx, "a@example.com", fakeHash, randomWallet(t))
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("same email: got %v, want ErrEmailTaken", err)
	}
	// Same wallet, written in lowercase: still the same wallet.
	_, err = s.CreateUser(ctx, "b@example.com", fakeHash, strings.ToLower(w))
	if !errors.Is(err, ErrWalletTaken) {
		t.Errorf("same wallet: got %v, want ErrWalletTaken", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	u, _ := s.CreateUser(ctx, "vahe@example.com", fakeHash, randomWallet(t))

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.UserBySession(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserBySession = %v, %v; want user %d", got.ID, err, u.ID)
	}

	// A made-up token must not log anyone in.
	if _, err := s.UserBySession(ctx, "made-up-token"); !errors.Is(err, ErrNotFound) {
		t.Errorf("fake token: got %v, want ErrNotFound", err)
	}

	// The database must hold the hash, never the token.
	var stored []byte
	s.db.QueryRow(`SELECT token_hash FROM sessions`).Scan(&stored)
	if bytes.Contains(stored, []byte(token)) {
		t.Error("token stored in plain form")
	}

	// After logout the token is dead.
	if err := s.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("after logout: got %v, want ErrNotFound", err)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	u, _ := s.CreateUser(ctx, "vahe@example.com", fakeHash, randomWallet(t))

	// Insert a session that expired yesterday.
	token, hash, _ := newSessionToken()
	_, err := s.db.Exec(`
		INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
		VALUES ($1, $2, now() - interval '8 days', now() - interval '1 day')`,
		hash, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session: got %v, want ErrNotFound", err)
	}
}

func TestUpdateWalletLogsOutOtherSessions(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	u, _ := s.CreateUser(ctx, "vahe@example.com", fakeHash, randomWallet(t))
	current, _ := s.CreateSession(ctx, u.ID)
	other, _ := s.CreateSession(ctx, u.ID)

	newWallet := randomWallet(t)
	if err := s.UpdateWallet(ctx, u.ID, newWallet, current); err != nil {
		t.Fatal(err)
	}

	got, err := s.UserBySession(ctx, current)
	if err != nil {
		t.Fatalf("current session was logged out: %v", err)
	}
	if got.Wallet != newWallet {
		t.Errorf("wallet = %s, want %s", got.Wallet, newWallet)
	}
	if _, err := s.UserBySession(ctx, other); !errors.Is(err, ErrNotFound) {
		t.Errorf("other session still works: %v", err)
	}
}

func TestUpdateWalletTakenByOtherUser(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	w1 := randomWallet(t)
	s.CreateUser(ctx, "a@example.com", fakeHash, w1)
	b, _ := s.CreateUser(ctx, "b@example.com", fakeHash, randomWallet(t))
	tok, _ := s.CreateSession(ctx, b.ID)
	other, _ := s.CreateSession(ctx, b.ID)

	if err := s.UpdateWallet(ctx, b.ID, w1, tok); !errors.Is(err, ErrWalletTaken) {
		t.Fatalf("got %v, want ErrWalletTaken", err)
	}
	// The transaction rolled back, so the other session was NOT deleted.
	if _, err := s.UserBySession(ctx, other); err != nil {
		t.Errorf("failed update still logged out other session: %v", err)
	}
}
