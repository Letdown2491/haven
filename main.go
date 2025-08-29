package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/spf13/afero"
	"github.com/letdown2491/haven/internal/whitelist"
)

var (
	pool   = nostr.NewSimplePool(context.Background(), nostr.WithPenaltyBox())
	config = loadConfig()
	fs     afero.Fs
)

func main() {
	// Initialize whitelists once at startup (open behavior if files are missing/empty).
	if err := whitelist.InitFromEnv(); err != nil {
		log.Fatal(err)
	}

	importFlag := flag.Bool("import", false, "Run the importNotes function after initializing relays")
	flag.Parse()

	nostr.InfoLogger = log.New(io.Discard, "", 0)
	slog.SetLogLoggerLevel(getLogLevelFromConfig())

	green := "\033[32m"
	reset := "\033[0m"
	fmt.Println(green + art + reset)
	log.Println("🚀 HAVEN", config.RelayVersion, "is booting up")

	fs = afero.NewOsFs()
	if err := fs.MkdirAll(config.BlossomPath, 0755); err != nil {
		log.Fatal("🚫 error creating blossom path:", err)
	}

	// Create relays, DB hooks, routes, etc.
	initRelays()

	// Apply whitelist gates after relays/DB QueryEvents are set up.
	wireWhitelistGates(nPubToPubkey(config.OwnerNpub))

	go func() {
		ensureImportRelays()
		refreshTrustNetwork()

		if *importFlag {
			log.Println("📦 importing notes")
			importOwnerNotes()
			importTaggedNotes()
			return
		}

		go subscribeInboxAndChat()
		go backupDatabase()
	}()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("templates/static"))))
	http.HandleFunc("/", dynamicRelayHandler)

	addr := fmt.Sprintf("%s:%d", config.RelayBindAddress, config.RelayPort)

	log.Printf("🔗 listening at %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("🚫 error starting server:", err)
	}
}

func dynamicRelayHandler(w http.ResponseWriter, r *http.Request) {
	var relay *khatru.Relay
	relayType := r.URL.Path

	if relayType == "" {
		relay = outboxRelay
	} else if relayType == "/private" {
		relay = privateRelay
	} else if relayType == "/chat" {
		relay = chatRelay
	} else if relayType == "/inbox" {
		relay = inboxRelay
	} else {
		relay = outboxRelay
	}

	relay.ServeHTTP(w, r)
}

func getLogLevelFromConfig() slog.Level {
	switch config.LogLevel {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo // Default level
	}
}
