// init.go — whitelist-integrated relay wiring (template)
// Adjust TODOs to match your repo’s actual types and module paths.

package main

import (
	"context"
	"log"
	"strings"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/Letdown2491/haven/tree/feature/whitelist-owner/internals/whitelist"
)

// ------------------------------------------------------------------------------------
// TODO: replace these with your actual config and store types.
// ------------------------------------------------------------------------------------
type Config struct {
	OwnerNpub string
}

type EventStore interface {
	QueryEvents(ctx context.Context, f nostr.Filter) (chan *nostr.Event, error)
	// ... any other methods your store exposes
}

// TODO: replace with your existing helper that turns npub -> hex pubkey
func nPubToPubkey(npub string) string {
	// stub for template; use your real implementation
	return strings.ToLower(npub)
}

// ------------------------------------------------------------------------------------
// initWhitelist is called early during process boot.
// You may call this from main.go too; having it here guarantees it's initialized
// before any relay wiring that references ReadWL/WriteWL.
// ------------------------------------------------------------------------------------
func init() {
	if err := whitelist.InitFromEnv(); err != nil {
		log.Fatalf("failed to initialize whitelist: %v", err)
	}
}

// ------------------------------------------------------------------------------------
// initRelays wires up your relays with SW2-style whitelist logic
//  - WRITE gate: RejectEvent wrapper using whitelist.AllowedToWrite(...)
//  - READ gate: QueryEvents wrapper that constrains f.Authors via ApplyReadToFilter
//
// Return whichever relays your program uses (outbox/inbox/chat/etc.).
// ------------------------------------------------------------------------------------
func initRelays(cfg Config, store EventStore) (outboxRelay *khatru.Relay, inboxRelay *khatru.Relay, err error) {
	// TODO: If your code creates relays elsewhere, move this logic next to that code
	// and just apply the wrappers shown below.
	outboxRelay = khatru.NewRelay()
	inboxRelay = khatru.NewRelay()

	// ----------------------------- WRITE GATE (EVENT) -----------------------------
	// Wrap existing RejectEvent for OUTBOX
	prevOutboxReject := outboxRelay.RejectEvent
	outboxRelay.RejectEvent = func(ctx context.Context, ev *nostr.Event) (bool, string) {
		// Owner override + write whitelist
		if !whitelist.AllowedToWrite(ev.PubKey, nPubToPubkey(cfg.OwnerNpub)) {
			return true, "pubkey not in write whitelist"
		}
		if prevOutboxReject != nil {
			return prevOutboxReject(ctx, ev)
		}
		return false, ""
	}

	// (Optional) also gate INBOX writes (if your design allows public writes there)
	prevInboxReject := inboxRelay.RejectEvent
	inboxRelay.RejectEvent = func(ctx context.Context, ev *nostr.Event) (bool, string) {
		if !whitelist.AllowedToWrite(ev.PubKey, nPubToPubkey(cfg.OwnerNpub)) {
			return true, "pubkey not in write whitelist"
		}
		if prevInboxReject != nil {
			return prevInboxReject(ctx, ev)
		}
		return false, ""
	}

	// ----------------------------- READ GATE (REQ) -----------------------------
	// Wrap QueryEvents so we constrain Authors before hitting the store.

	// OUTBOX read constraint
	prevOutboxQuery := outboxRelay.QueryEvents
	outboxRelay.QueryEvents = func(ctx context.Context, f nostr.Filter) (chan *nostr.Event, error) {
		whitelist.ReadWL.ApplyReadToFilter(&f)
		if prevOutboxQuery != nil {
			return prevOutboxQuery(ctx, f)
		}
		// Fallback: direct to store if no previous handler
		return store.QueryEvents(ctx, f)
	}

	// INBOX read constraint
	prevInboxQuery := inboxRelay.QueryEvents
	inboxRelay.QueryEvents = func(ctx context.Context, f nostr.Filter) (chan *nostr.Event, error) {
		whitelist.ReadWL.ApplyReadToFilter(&f)
		if prevInboxQuery != nil {
			return prevInboxQuery(ctx, f)
		}
		return store.QueryEvents(ctx, f)
	}

	// ----------------------------- RETURN -----------------------------
	return outboxRelay, inboxRelay, nil
}
