package whitelist

import (
	"os"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

var (
	ReadWL  *List
	WriteWL *List
)

// InitFromEnv initializes globals from environment.
// Env:
//   WHITELIST_READ_FILE   (optional)
//   WHITELIST_WRITE_FILE  (optional)
// Missing/empty files => open behavior.
func InitFromEnv() error {
	readFile := os.Getenv("WHITELIST_READ_FILE")
	writeFile := os.Getenv("WHITELIST_WRITE_FILE")

	var err error
	ReadWL, err = Load(readFile)
	if err != nil {
		 return err
	}
	WriteWL, err = Load(writeFile)
	if err != nil {
		 return err
	}
	return nil
}

// AllowedToWrite checks write access with owner override.
func AllowedToWrite(pub, ownerPub string) bool {
	if ownerPub != "" && strings.EqualFold(pub, ownerPub) {
		return true
	}
	return WriteWL.Allows(pub)
}

// ConstrainFilters applies the active read whitelist to each filter in place.
func ConstrainFilters(filters []nostr.Filter) {
	if ReadWL == nil || ReadWL.set == nil || len(ReadWL.set) == 0 {
		return
	}
	for i := range filters {
		ReadWL.ApplyReadToFilter(&filters[i])
	}
}
