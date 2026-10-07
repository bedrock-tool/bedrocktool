package worlds

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func TestChunkStorageFailureBlocksExport(t *testing.T) {
	for _, kind := range []string{"level", "sub"} {
		t.Run(kind, func(t *testing.T) {
			br, err := newNetworkBlockRegistry(true)
			if err != nil {
				t.Fatal(err)
			}
			br.Finalize()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			state, err := worldstate.New(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			state.SetDimension(world.Overworld)
			state.BlockRegistry = br
			state.Folder = filepath.Join(t.TempDir(), "world")
			injected := errors.New("injected chunk storage failure")
			calls := 0
			w := &worldsHandler{worldState: state, session: &proxy.Session{}, mapUI: NewMapUI(nil), log: logrus.NewEntry(logrus.New()), serverState: serverState{blocks: br, useHashedRids: true}, chunkStore: func(pos world.ChunkPos, ch *worldstate.Chunk) error {
				calls++
				if pos != (world.ChunkPos{2, 3}) || ch == nil {
					t.Fatalf("incorrect storage input: %v %v", pos, ch)
				}
				return injected
			}}
			sub := bytes.NewBuffer([]byte{9, 1, 0, 1})
			hash := uint32(2920631295)
			protocol.WriteVarint32(sub, int32(hash))
			if kind == "level" {
				payload := append([]byte(nil), sub.Bytes()...)
				for range len(chunk.New(br, state.Range()).Sub()) {
					payload = append(payload, 1, 0)
				}
				payload = append(payload, 0) // border blocks
				err = w.handleLevelChunk(&packet.LevelChunk{Position: protocol.ChunkPos{2, 3}, SubChunkCount: 1, RawPayload: payload}, time.Now())
			} else {
				ch := &worldstate.Chunk{Chunk: chunk.New(br, state.Range())}
				if err := state.StoreChunk(world.ChunkPos{2, 3}, ch); err != nil {
					t.Fatal(err)
				}
				err = w.processSubChunk(&packet.SubChunk{Position: protocol.SubChunkPos{2, 0, 3}, SubChunkEntries: []protocol.SubChunkEntry{{Result: protocol.SubChunkResultSuccess, RawPayload: protocol.Option(sub.Bytes())}}})
			}
			if calls != 1 {
				t.Fatalf("storage not exercised: calls=%d error=%v", calls, err)
			}
			if !errors.Is(err, injected) {
				t.Fatalf("storage failure lost: %v", err)
			}
			if !errors.Is(state.ExportError(), injected) {
				t.Fatalf("world still exportable: %v", state.ExportError())
			}
			if err := w.saveWorldState(state, proxy.Player{}, nil); !errors.Is(err, injected) {
				t.Fatalf("export did not refuse storage failure: %v", err)
			}
			if _, err := os.Stat(state.Folder + ".mcworld"); !os.IsNotExist(err) {
				t.Fatalf("mcworld artifact created: %v", err)
			}
		})
	}
}
