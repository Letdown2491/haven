// init_whitelist_patch.go
// Non-invasive wiring for SW2-style whitelists.
// This file *does not* redeclare your config, DBs, or relays.
// It simply prepends write gates and wraps read queries after your relays/DBs are set up.

package main

import (
	"context"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/letdown2491/haven/internal/whitelist"
)

var whitelistGatesApplied bool

// wireWhitelistGates should be called *after* initRelays() has created relays and appended DB QueryEvents.
// ownerHex is your owner's hex pubkey (e.g., nPubToPubkey(config.OwnerNpub)).
func wireWhitelistGates(ownerHex string) {
	if whitelistGatesApplied {
		return
	}

	// -------------------- WRITE gate (EVENT) --------------------
	gate := func(ctx context.Context, ev *nostr.Event) (bool, string) {
		if !whitelist.AllowedToWrite(ev.PubKey, ownerHex) {
			return true, "pubkey not in write whitelist"
		}
		return false, ""
	}

	prependReject := func(r *khatru.Relay) {
		if r == nil {
			return
		}
		r.RejectEvent = append([]func(context.Context, *nostr.Event) (bool, string){gate}, r.RejectEvent...)
	}

	prependReject(outboxRelay)
	prependReject(inboxRelay)   // optional if inbox accepts writes
	prependReject(chatRelay)    // optional
	prependReject(privateRelay) // optional

	// -------------------- READ gate (REQ) --------------------
	wrapQueryWithReadWhitelist := func(r *khatru.Relay, db func(context.Context, nostr.Filter) (chan *nostr.Event, error)) {
		if r == nil || db == nil {
			return
		}
		// Prepend wrapper so it runs before any DB handler previously appended.
		r.QueryEvents = append([]func(context.Context, nostr.Filter) (chan *nostr.Event, error){
			func(ctx context.Context, f nostr.Filter) (chan *nostr.Event, error) {
				whitelist.ReadWL.ApplyReadToFilter(&f)
				return db(ctx, f)
			},
		}, r.QueryEvents...)
	}

	// Use your existing DB variables (these are defined elsewhere in Haven)
	wrapQueryWithReadWhitelist(outboxRelay, outboxDB.QueryEvents)
	wrapQueryWithReadWhitelist(inboxRelay, inboxDB.QueryEvents)
	wrapQueryWithReadWhitelist(chatRelay, chatDB.QueryEvents)
	wrapQueryWithReadWhitelist(privateRelay, privateDB.QueryEvents)

	whitelistGatesApplied = true
}
