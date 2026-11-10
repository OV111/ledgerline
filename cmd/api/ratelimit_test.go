package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// The limiter runs before the handler, so we can test it without a database:
// we send requests with the wrong Content-Type (415) and check that the
// 11th one is stopped earlier, with 429.
func TestLoginRateLimit(t *testing.T) {
	h := newRouter(newServer(nil))

	attempt := func(path, ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader("x"))
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	for i := 1; i <= 10; i++ {
		if rec := attempt("/login", "203.0.113.7"); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was limited, want allowed", i)
		}
	}

	rec := attempt("/login", "203.0.113.7")
	wantStatus(t, rec, http.StatusTooManyRequests)
	secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || secs < 1 || secs > 60 {
		t.Errorf("Retry-After = %q, want 1..60 seconds", rec.Header().Get("Retry-After"))
	}

	// The limit is shared by all password endpoints, so switching to
	// /signup doesn't give the attacker new attempts.
	wantStatus(t, attempt("/signup", "203.0.113.7"), http.StatusTooManyRequests)

	// Another IP is not affected.
	if rec := attempt("/login", "198.51.100.1"); rec.Code == http.StatusTooManyRequests {
		t.Error("a different IP was limited")
	}
}

// A fake X-Forwarded-For header must not give a fresh limit.
func TestRateLimitIgnoresForwardedFor(t *testing.T) {
	h := newRouter(newServer(nil))
	var last int
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest("POST", "/login", strings.NewReader("x"))
		req.RemoteAddr = "203.0.113.9:1234"
		req.Header.Set("X-Forwarded-For", "10.0.0."+strconv.Itoa(i))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("11th request = %d, want 429 despite changing X-Forwarded-For", last)
	}
}
