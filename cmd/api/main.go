// Command api is the HTTP API for Ledgerline.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql

	"ledgerline/internal/auth"
	"ledgerline/internal/ratelimit"
)

func main() {
	// PORT can be set from the environment (Docker Compose does this).
	port := os.Getenv("PORT")
	if port == "" {
		// Ledgerline's own port (not the common 8080). See .env.example.
		port = "8090"
	}

	// DATABASE_URL is required: without a database the API can't do anything
	// useful, so we stop right away with a clear message.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	// sql.Open does not connect yet; it only prepares a connection pool.
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	// Ping makes a real connection, so a wrong URL or password fails here
	// at startup instead of on the first user request.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	s := newServer(auth.NewStore(db))
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: newRouter(s),
		// Without a timeout a slow client could keep a connection open forever.
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

// server holds what the handlers need.
type server struct {
	store *auth.Store
	// passwordLimiter limits endpoints that check a password or run bcrypt,
	// per IP address, to stop password guessing (see ratelimit.go).
	passwordLimiter *ratelimit.Limiter
}

// newServer is the only way to build a server, so the rate limiter can't be
// forgotten. Tests use it too.
func newServer(store *auth.Store) *server {
	return &server{
		store:           store,
		passwordLimiter: ratelimit.New(10, time.Minute),
	}
}

// newRouter connects URL paths to handler functions.
// It is a separate function so tests can use it without starting a server.
func newRouter(s *server) http.Handler {
	mux := http.NewServeMux()
	// "GET /healthz" means: only the GET method, only this exact path.
	mux.HandleFunc("GET /healthz", handleHealth)

	// limitPasswords wraps the endpoints that check a password or run bcrypt.
	mux.HandleFunc("POST /signup", s.limitPasswords(s.handleSignup))
	mux.HandleFunc("POST /login", s.limitPasswords(s.handleLogin))
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /me", s.requireUser(s.handleMe))
	mux.HandleFunc("PUT /me/wallet", s.limitPasswords(s.requireUser(s.handleChangeWallet)))
	return mux
}

// handleHealth tells Docker, CI and humans that the API process is alive.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("healthz: write response: %v", err)
	}
}
