package worlds

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestHighUnsignedRealmHashSurvivesWorldDBExport(t *testing.T) {
	br, err := newNetworkBlockRegistry(true)
	if err != nil {
		t.Fatal(err)
	}
	br.Finalize()
	hash := uint32(2920631295)
	buf := bytes.NewBuffer([]byte{9, 1, 0, 1})
	protocol.WriteVarint32(buf, int32(hash))
	index := byte(0)
	sub, err := chunk.DecodeSubChunk(buf, br, world.Overworld.Range(), &index, chunk.NetworkEncoding, true)
	if err != nil {
		t.Fatal(err)
	}
	ch := chunk.New(br, world.Overworld.Range())
	ch.Sub()[index] = sub
	rid, ok := br.HashToRuntimeID(hash)
	if !ok {
		t.Fatal("missing hash")
	}
	if ch.Block(0, 0, 0, 0) != rid {
		t.Fatal("wire hash not converted to local index")
	}
	dir := filepath.Join(t.TempDir(), "worlds", "test-realm", "visited-world")
	db, err := (mcdb.Config{Blocks: br}).Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	pos := world.ChunkPos{4, -2}
	if err = db.StoreColumn(pos, world.Overworld, &chunk.Column{Chunk: ch}); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "level.dat")); err != nil {
		t.Fatal(err)
	}
	db, err = (mcdb.Config{Blocks: br}).Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	col, err := db.LoadColumn(pos, world.Overworld)
	if err != nil {
		t.Fatal(err)
	}
	name, props, found := br.RuntimeIDToState(col.Chunk.Block(0, 0, 0, 0))
	if !found {
		t.Fatal("missing exported block")
	}
	wantName, wantProps, _ := br.RuntimeIDToState(rid)
	if name != wantName || !reflect.DeepEqual(props, wantProps) {
		t.Fatalf("export changed state: %s %v want %s %v", name, props, wantName, wantProps)
	}
	t.Logf("verified level.dat + DB reopen: %s %v", name, props)
}
