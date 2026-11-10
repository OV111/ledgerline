// Command listener will watch Ethereum Sepolia for USDC transfers to
// freelancer wallets and record them in the ledger.
//
// It only READS the blockchain. It never holds funds or private keys.
//
// Right now it is a skeleton: it starts, logs a heartbeat, and shuts down
// cleanly. The chain-watching logic comes in its own feature branch.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// ctx is cancelled when the process is asked to stop (Ctrl+C or
	// `docker compose down`), so the loop below can exit cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("listener started (skeleton: not watching the chain yet)")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("listener stopped")
			return
		case <-ticker.C:
			// TODO: poll Sepolia for new USDC Transfer events here.
			log.Println("listener heartbeat")
		}
	}
}
