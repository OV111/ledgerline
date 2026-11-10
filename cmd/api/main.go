// Command api is the HTTP API for Ledgerline.
//
// Right now it only answers a health check. Real endpoints (invoices,
// payments) come later, one feature per branch.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	// PORT can be set from the environment (Docker Compose does this).
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: newRouter(),
		// Without a timeout a slow client could keep a connection open forever.
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

// newRouter connects URL paths to handler functions.
// It is a separate function so tests can use it without starting a server.
func newRouter() http.Handler {
	mux := http.NewServeMux()
	// "GET /healthz" means: only the GET method, only this exact path.
	mux.HandleFunc("GET /healthz", handleHealth)
	return mux
}

// handleHealth tells Docker, CI and humans that the API process is alive.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("healthz: write response: %v", err)
	}
}
