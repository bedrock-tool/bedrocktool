package worlds

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestNonHashedStartGameWireBlocksExport(t *testing.T) {
	// Negotiate from actual wire data rather than testing only the constructor.
	var wire bytes.Buffer
	sent := &packet.StartGame{UseBlockNetworkIDHashes: false}
	sent.Marshal(protocol.NewWriter(&wire, 0))
	received := &packet.StartGame{}
	received.Marshal(protocol.NewReader(&wire, 0, true))
	for _, tc := range []struct {
		name          string
		haveStartGame bool
		preLogin      bool
	}{
		{"initial_prelogin", false, true},
		{"duplicate_prelogin", true, true},
		{"duplicate_postspawn", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := worldstate.New(context.Background(), nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			state.Folder = filepath.Join(t.TempDir(), "world")
			w := &worldsHandler{worldState: state}
			w.serverState.haveStartGame = tc.haveStartGame
			w.serverState.useHashedRids = tc.haveStartGame
			got, err := w.packetHandler(nil, received, false, time.Now(), tc.preLogin)
			if err == nil || !strings.Contains(err.Error(), "non-hashed") || got != nil {
				t.Errorf("accepted unsupported wire IDs: packet=%T err=%v", got, err)
			}
			if err := state.ExportError(); err == nil || !strings.Contains(err.Error(), "non-hashed") {
				t.Fatalf("unsupported wire palette did not block export: %v", err)
			}
			if err := w.saveWorldState(state, proxy.Player{}, nil); err == nil || !strings.Contains(err.Error(), "non-hashed") {
				t.Fatalf("mcworld export did not reject unsupported palette: %v", err)
			}
			if _, err := os.Stat(state.Folder + ".mcworld"); !os.IsNotExist(err) {
				t.Fatalf("mcworld artifact created: %v", err)
			}
		})
	}
}

func TestNonHashedRegistryRejected(t *testing.T) {
	br, err := newNetworkBlockRegistry(false)
	if err == nil || br != nil || !strings.Contains(err.Error(), "non-hashed") {
		t.Fatalf("unsafe legacy registry returned: %T %v", br, err)
	}
}
