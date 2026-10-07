package worlds

import (
	"bytes"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"strings"
	"testing"
)

func TestUnknownNetworkHashRejected(t *testing.T) {
	br, err := newNetworkBlockRegistry(true)
	if err != nil {
		t.Fatal(err)
	}
	br.Finalize()
	for _, version := range []byte{1, 8, 9} {
		buf := bytes.NewBuffer([]byte{version})
		if version != 1 {
			buf.WriteByte(1)
		}
		if version == 9 {
			buf.WriteByte(0)
		}
		buf.WriteByte(1)
		protocol.WriteVarint32(buf, int32(-123456789))
		index := byte(0)
		sub, err := chunk.DecodeSubChunk(buf, br, cube.Range{0, 15}, &index, chunk.NetworkEncoding, true)
		if err == nil || !strings.Contains(err.Error(), "unknown network block hash") || sub != nil {
			t.Errorf("version=%d sub=%v error=%v: unsafe unknown hash accepted", version, sub, err)
		}
	}
}

func TestOptionalMapFailureDoesNotEscape(t *testing.T) {
	br, err := newNetworkBlockRegistry(true)
	if err != nil {
		t.Fatal(err)
	}
	br.Finalize()
	ch := chunk.New(br, cube.Range{0, 15})
	ch.SetBlock(0, 1, 0, 0, 2920631295)
	water, ok := br.StateToRuntimeID("minecraft:water", map[string]any{"liquid_depth": int32(0)})
	if !ok {
		t.Fatal("water missing")
	}
	ch.SetBlock(0, 2, 0, 0, water)
	m := NewMapUI(nil)
	close(m.haveColors)
	m.ChunkRenderer = utils.NewChunkRenderer(nil)
	m.SetChunk([2]int32{0, 0}, ch)
	m.processQueue()
	if !m.renderFailed {
		t.Fatal("renderer should disable itself after failure")
	}
	m.SetChunk([2]int32{1, 0}, ch)
	if len(m.renderQueue) != 0 {
		t.Fatal("failed optional renderer retained queued chunks")
	}
}
