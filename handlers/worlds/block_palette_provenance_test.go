package worlds

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// This public upstream asset is not a packet capture or private world fixture.
func TestVanillaPaletteProvenance(t *testing.T) {
	const want = "f0784a6284d6ca7d98cc3472f4ce84241a11e11b18ed16f6591dfd5e6da6fbd6"
	if got := fmt.Sprintf("%x", sha256.Sum256(networkBlockStates)); got != want {
		t.Fatalf("pinned df-mc/dragonfly palette checksum: got %s, want %s", got, want)
	}
}
