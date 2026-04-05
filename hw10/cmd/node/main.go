// main.go – single binary for all node types.
//
// Environment variables:
//
//	ROLE      "leader" | "follower" | "leaderless"
//	MODE      "w5r1" | "w1r5" | "quorum"  (leader only)
//	PORT      HTTP port to listen on, default 8080
//	PEERS     comma-separated peer base URLs
//	           leader:    list of follower base URLs
//	           leaderless: list of other node base URLs (excluding self)
//
// Example (leader in w5r1 mode, 4 followers):
//
//	ROLE=leader MODE=w5r1 PORT=8080 \
//	PEERS=http://follower1:8080,http://follower2:8080,http://follower3:8080,http://follower4:8080
package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"cs6650-a4/internal/leader"
	"cs6650-a4/internal/leaderless"
	"cs6650-a4/internal/store"
)

func main() {
	role := strings.ToLower(os.Getenv("ROLE"))
	mode := strings.ToLower(os.Getenv("MODE"))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	peersRaw := os.Getenv("PEERS")
	var peers []string
	if peersRaw != "" {
		peers = strings.Split(peersRaw, ",")
	}

	s := store.New()
	mux := http.NewServeMux()

	// Health-check endpoint – useful for Docker health checks and tests.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	switch role {
	case "leader":
		if mode == "" {
			log.Fatal("MODE env var required for leader role (w5r1 | w1r5 | quorum)")
		}
		h := leader.NewLeaderHandler(s, peers, mode)
		h.RegisterRoutes(mux)
		log.Printf("Starting LEADER node, mode=%s, port=%s, peers=%v", mode, port, peers)

	case "follower":
		h := leader.NewFollowerHandler(s)
		h.RegisterRoutes(mux)
		log.Printf("Starting FOLLOWER node, port=%s", port)

	case "leaderless":
		h := leaderless.New(s, peers)
		h.RegisterRoutes(mux)
		log.Printf("Starting LEADERLESS node, port=%s, peers=%v", port, peers)

	default:
		log.Fatalf("Unknown ROLE %q. Must be leader | follower | leaderless", role)
	}

	addr := ":" + port
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
