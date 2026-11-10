package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	// httptest lets us call the router directly, with no real network.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	// The health check doesn't use the database, so a nil store is fine.
	newRouter(newServer(nil)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("body = %s, want {\"status\":\"ok\"}", body)
	}
}

func TestHealthRejectsPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	newRouter(newServer(nil)).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
