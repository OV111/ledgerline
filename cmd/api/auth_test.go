package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"ledgerline/internal/auth"
)

// Two valid wallets (EIP-55 test vectors).
const (
	walletA = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	walletB = "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"
)

// --- helpers -----------------------------------------------------------

// testServer returns a router backed by a real database.
// Skipped unless TEST_DATABASE_URL is set. WARNING: deletes all users and
// sessions, so only point it at a throwaway database.
func testServer(t *testing.T) http.Handler {
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
	return newRouter(newServer(auth.NewStore(db)))
}

// send makes one request. body "" means no body. cookie may be nil.
func send(t *testing.T, h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, want, rec.Body.String())
	}
}

// sessionFrom returns the session cookie set by a response, or nil.
func sessionFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	return nil
}

func signupBody(email, password, wallet string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password, "wallet": wallet})
	return string(b)
}

// signup creates a user and returns their session cookie.
func signup(t *testing.T, h http.Handler, email, wallet string) *http.Cookie {
	t.Helper()
	rec := send(t, h, "POST", "/signup", signupBody(email, "correct-horse", wallet), nil)
	wantStatus(t, rec, http.StatusCreated)
	return sessionFrom(rec)
}

// --- tests that need no database (they also run in CI) -------------------

func TestReadJSONRules(t *testing.T) {
	// readJSON runs before any database call, so an empty server is enough.
	h := newRouter(newServer(nil))

	t.Run("wrong content type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/signup", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		wantStatus(t, rec, http.StatusUnsupportedMediaType)
	})
	t.Run("unknown field", func(t *testing.T) {
		wantStatus(t, send(t, h, "POST", "/signup", `{"wallett":"x"}`, nil), http.StatusBadRequest)
	})
	t.Run("broken json", func(t *testing.T) {
		wantStatus(t, send(t, h, "POST", "/signup", `{"email":`, nil), http.StatusBadRequest)
	})
	t.Run("two objects", func(t *testing.T) {
		wantStatus(t, send(t, h, "POST", "/signup", `{} {}`, nil), http.StatusBadRequest)
	})
	t.Run("too big", func(t *testing.T) {
		big := `{"email":"` + strings.Repeat("a", 5000) + `"}`
		wantStatus(t, send(t, h, "POST", "/signup", big, nil), http.StatusRequestEntityTooLarge)
	})
}

func TestBadSignupInput(t *testing.T) {
	// Validation also runs before the database.
	h := newRouter(newServer(nil))
	tests := map[string]string{
		"bad email":      signupBody("not-an-email", "correct-horse", walletA),
		"short password": signupBody("a@x.com", "short", walletA),
		"bad wallet":     signupBody("a@x.com", "correct-horse", "0x123"),
		"zero wallet":    signupBody("a@x.com", "correct-horse", "0x"+strings.Repeat("0", 40)),
		"bad checksum":   signupBody("a@x.com", "correct-horse", "0x5AAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			wantStatus(t, send(t, h, "POST", "/signup", body, nil), http.StatusBadRequest)
		})
	}
}

func TestMeWithoutCookie(t *testing.T) {
	wantStatus(t, send(t, newRouter(newServer(nil)), "GET", "/me", "", nil), http.StatusUnauthorized)
}

// --- tests with a database --------------------------------------------

func TestSignupLogsInAndHidesHash(t *testing.T) {
	h := testServer(t)
	rec := send(t, h, "POST", "/signup", signupBody(" Vahe@Example.com ", "correct-horse", strings.ToLower(walletA)), nil)
	wantStatus(t, rec, http.StatusCreated)

	var got map[string]any
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["email"] != "vahe@example.com" {
		t.Errorf("email = %v, want normalized vahe@example.com", got["email"])
	}
	if got["wallet"] != walletA {
		t.Errorf("wallet = %v, want checksum form %s", got["wallet"], walletA)
	}
	if strings.Contains(rec.Body.String(), "$2a$") || strings.Contains(rec.Body.String(), "password") {
		t.Errorf("response leaks password data: %s", rec.Body.String())
	}

	c := sessionFrom(rec)
	if c == nil {
		t.Fatal("no session cookie after signup")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("unsafe cookie: HttpOnly=%v Secure=%v SameSite=%v", c.HttpOnly, c.Secure, c.SameSite)
	}
	wantStatus(t, send(t, h, "GET", "/me", "", c), http.StatusOK)
}

func TestSignupDuplicates(t *testing.T) {
	h := testServer(t)
	signup(t, h, "a@x.com", walletA)

	wantStatus(t, send(t, h, "POST", "/signup", signupBody("A@X.com", "correct-horse", walletB), nil), http.StatusConflict)
	wantStatus(t, send(t, h, "POST", "/signup", signupBody("b@x.com", "correct-horse", walletA), nil), http.StatusConflict)
}

func TestLogin(t *testing.T) {
	h := testServer(t)
	signup(t, h, "a@x.com", walletA)

	ok := send(t, h, "POST", "/login", `{"email":"A@x.com","password":"correct-horse"}`, nil)
	wantStatus(t, ok, http.StatusOK)
	if sessionFrom(ok) == nil {
		t.Error("no session cookie after login")
	}

	wrongPw := send(t, h, "POST", "/login", `{"email":"a@x.com","password":"wrong-password"}`, nil)
	noUser := send(t, h, "POST", "/login", `{"email":"nobody@x.com","password":"correct-horse"}`, nil)
	badEmail := send(t, h, "POST", "/login", `{"email":"garbage","password":"correct-horse"}`, nil)
	for _, rec := range []*httptest.ResponseRecorder{wrongPw, noUser, badEmail} {
		wantStatus(t, rec, http.StatusUnauthorized)
		if sessionFrom(rec) != nil {
			t.Error("failed login set a session cookie")
		}
	}
	// Same answer for all three, so attackers can't tell which emails exist.
	if wrongPw.Body.String() != noUser.Body.String() || noUser.Body.String() != badEmail.Body.String() {
		t.Errorf("different error bodies: %q / %q / %q", wrongPw.Body, noUser.Body, badEmail.Body)
	}
}

func TestLogout(t *testing.T) {
	h := testServer(t)
	c := signup(t, h, "a@x.com", walletA)

	wantStatus(t, send(t, h, "POST", "/logout", "", c), http.StatusNoContent)
	// The old cookie value must not work anymore, even if someone kept a copy.
	wantStatus(t, send(t, h, "GET", "/me", "", c), http.StatusUnauthorized)
	// Logging out when not logged in is fine.
	wantStatus(t, send(t, h, "POST", "/logout", "", nil), http.StatusNoContent)
}

func TestMeWithFakeCookie(t *testing.T) {
	h := testServer(t)
	fake := &http.Cookie{Name: sessionCookie, Value: "made-up-token"}
	wantStatus(t, send(t, h, "GET", "/me", "", fake), http.StatusUnauthorized)
}

func TestChangeWallet(t *testing.T) {
	h := testServer(t)
	current := signup(t, h, "a@x.com", walletA)
	// A second login of the same user, e.g. on another computer.
	other := sessionFrom(send(t, h, "POST", "/login", `{"email":"a@x.com","password":"correct-horse"}`, nil))

	body := func(wallet, pw string) string {
		b, _ := json.Marshal(map[string]string{"wallet": wallet, "password": pw})
		return string(b)
	}

	t.Run("not logged in", func(t *testing.T) {
		wantStatus(t, send(t, h, "PUT", "/me/wallet", body(walletB, "correct-horse"), nil), http.StatusUnauthorized)
	})
	t.Run("wrong password", func(t *testing.T) {
		wantStatus(t, send(t, h, "PUT", "/me/wallet", body(walletB, "wrong-password"), current), http.StatusForbidden)
	})
	t.Run("bad wallet", func(t *testing.T) {
		wantStatus(t, send(t, h, "PUT", "/me/wallet", body("0x123", "correct-horse"), current), http.StatusBadRequest)
	})
	t.Run("ok, and other sessions are logged out", func(t *testing.T) {
		wantStatus(t, send(t, h, "PUT", "/me/wallet", body(walletB, "correct-horse"), current), http.StatusOK)

		me := send(t, h, "GET", "/me", "", current)
		wantStatus(t, me, http.StatusOK)
		if !strings.Contains(me.Body.String(), walletB) {
			t.Errorf("wallet not changed: %s", me.Body.String())
		}
		wantStatus(t, send(t, h, "GET", "/me", "", other), http.StatusUnauthorized)
	})
	t.Run("wallet of another user", func(t *testing.T) {
		signup(t, h, "b@x.com", walletA) // walletA is free again
		wantStatus(t, send(t, h, "PUT", "/me/wallet", body(walletA, "correct-horse"), current), http.StatusConflict)
	})
}
