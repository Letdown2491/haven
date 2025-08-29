// Package whitelist provides SW2-style read/write whitelists for a Nostr relay.
// Semantics: if a list is nil or has zero entries, it allows everyone.
// If it has entries, it allows only listed pubkeys (hex; lowercase recommended).
package whitelist

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

type List struct {
	set map[string]struct{}
}

// Load reads a JSON file like: { "pubkeys": ["<hex>", ...] }.
// Missing file or empty array => open (allow all).
func Load(path string) (*List, error) {
	if path == "" {
		return &List{set: nil}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &List{set: nil}, nil
		}
		return nil, err
	}
	var payload struct {
		Pubkeys []string `json:"pubkeys"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	if len(payload.Pubkeys) == 0 {
		return &List{set: nil}, nil
	}
	m := make(map[string]struct{}, len(payload.Pubkeys))
	for _, pk := range payload.Pubkeys {
		pk = strings.ToLower(strings.TrimSpace(pk))
		if pk != "" {
			m[pk] = struct{}{}
		}
	}
	return &List{set: m}, nil
}

// Allows reports whether the given pubkey is permitted under this list.
func (l *List) Allows(pubkey string) bool {
	if l == nil || l.set == nil || len(l.set) == 0 {
		return true // open
	}
	_, ok := l.set[strings.ToLower(pubkey)]
	return ok
}

// Keys returns the allow-list as a slice (useful for authors filters).
func (l *List) Keys() []string {
	if l == nil || l.set == nil || len(l.set) == 0 {
		return nil
	}
	out := make([]string, 0, len(l.set))
	for k := range l.set {
		out = append(out, k)
	}
	return out
}

// ApplyReadToFilter constrains f.Authors to the whitelist when active.
// - If list is open: no-op
// - If Authors empty: set to whitelist keys
// - If Authors present: intersect with whitelist
func (l *List) ApplyReadToFilter(f *nostr.Filter) {
	if f == nil || l == nil || l.set == nil || len(l.set) == 0 {
		return
	}
	if len(f.Authors) == 0 {
		f.Authors = l.Keys()
		return
	}
	dst := f.Authors[:0]
	for _, a := range f.Authors {
		if _, ok := l.set[strings.ToLower(a)]; ok {
			dst = append(dst, a)
		}
	}
	f.Authors = dst
}
