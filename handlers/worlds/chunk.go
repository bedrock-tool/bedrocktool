package worlds

import (
	"bytes"
	"errors"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (w *worldsHandler) handleLevelChunk(pk *packet.LevelChunk, timeReceived time.Time) (err error) {
	if len(pk.RawPayload) == 0 {
		w.log.Info(locale.Loc("empty_chunk", nil))
		return
	}

	if pk.CacheEnabled {
		return errors.New("cache is supposed to be handled in proxy")
	}

	var subChunkCount int
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited, protocol.SubChunkRequestModeLimitless:
		subChunkCount = 0
	default:
		subChunkCount = int(pk.SubChunkCount)
	}

	w.worldStateMu.Lock()
	defer w.worldStateMu.Unlock()

	//os.WriteFile("chunk.bin", pk.RawPayload, 0777)

	levelChunk, blockNBTs, err := chunk.NetworkDecode(
		w.serverState.blocks,
		pk.RawPayload, subChunkCount,
		w.worldState.Range(),
		w.serverState.useHashedRids,
	)
	if err != nil {
		return err
	}

	ch := &worldstate.Chunk{
		Chunk:         levelChunk,
		BlockEntities: make(map[cube.Pos]map[string]any),
	}

	for _, blockNBT := range blockNBTs {
		x := int(blockNBT["x"].(int32))
		y := int(blockNBT["y"].(int32))
		z := int(blockNBT["z"].(int32))
		ch.BlockEntities[cube.Pos{x, y, z}] = blockNBT
	}

	pos := world.ChunkPos(pk.Position)
	if !w.scripting.OnChunkAdd(pos, timeReceived) {
		w.worldState.IgnoredChunks[pos] = true
		return
	}
	w.worldState.IgnoredChunks[pos] = false

	// request subchunks
	max := w.worldState.Dimension().Range().Height() / 16
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited:
		max = int(pk.HighestSubChunk)
		fallthrough
	case protocol.SubChunkRequestModeLimitless:
		var offsetTable []protocol.SubChunkOffset
		r := w.worldState.Dimension().Range()
		for y := int8(r.Min() / 16); y < int8(r.Max()/16)+1; y++ {
			offsetTable = append(offsetTable, protocol.SubChunkOffset{0, y, 0})
		}

		dimId, _ := world.DimensionID(w.worldState.Dimension())
		_ = w.session.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(dimId),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: offsetTable[:min(max+1, len(offsetTable))],
		})
	default:
	}

	err = w.worldState.StoreChunk(pos, ch)
	if err != nil {
		w.log.Error(err)
	}

	w.scripting.OnChunkData(pos)

	w.session.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{
		"Chunks":   len(w.worldState.StoredChunks),
		"Entities": w.worldState.EntityCount(),
		"Name":     w.worldState.Name,
	}, len(w.worldState.StoredChunks)))

	return nil
}

func (w *worldsHandler) processSubChunk(pk *packet.SubChunk) error {
	w.worldStateMu.Lock()
	defer w.worldStateMu.Unlock()

	var chunks = make(map[world.ChunkPos]*worldstate.Chunk)
	for _, ent := range pk.SubChunkEntries {
		if ent.Result != protocol.SubChunkResultSuccess {
			continue
		}
		var (
			absX = pk.Position[0] + int32(ent.Offset[0])
			absZ = pk.Position[2] + int32(ent.Offset[2])
			pos  = world.ChunkPos{absX, absZ}
		)

		if w.worldState.IgnoredChunks[pos] {
			continue
		}

		if _, ok := chunks[pos]; ok {
			continue
		}
		ch, ok, err := w.worldState.LoadChunk(pos)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("bug check: subchunk received before chunk")
		}
		chunks[pos] = ch
	}

	for _, ent := range pk.SubChunkEntries {
		var (
			absX = pk.Position[0] + int32(ent.Offset[0])
			absY = pk.Position[1] + int32(ent.Offset[1])
			absZ = pk.Position[2] + int32(ent.Offset[2])
			pos  = world.ChunkPos{absX, absZ}
		)

		ch, ok := chunks[pos]
		if !ok {
			continue
		}

		switch ent.Result {
		case protocol.SubChunkResultSuccessAllAir:
		case protocol.SubChunkResultSuccess:
			buf := bytes.NewBuffer(ent.RawPayload)
			index := uint8(absY)
			sub, err := chunk.DecodeSubChunk(
				buf,
				w.serverState.blocks,
				w.worldState.Dimension().Range(),
				&index,
				chunk.NetworkEncoding,
				w.serverState.useHashedRids,
			)
			if err != nil {
				return err
			}
			ch.Chunk.Sub()[index] = sub

			if buf.Len() > 0 {
				dec := nbt.NewDecoderWithEncoding(buf, nbt.NetworkLittleEndian)
				for buf.Len() > 0 {
					blockNBT := make(map[string]any, 0)
					if err := dec.Decode(&blockNBT); err != nil {
						return err
					}
					ch.BlockEntities[cube.Pos{
						int(blockNBT["x"].(int32)),
						int(blockNBT["y"].(int32)),
						int(blockNBT["z"].(int32)),
					}] = blockNBT
				}
			}
		}
	}

	for pos, ch := range chunks {
		w.worldState.StoreChunk(pos, ch)
	}
	for pos := range chunks {
		w.scripting.OnChunkData(pos)
	}

	w.mapUI.SchedRedraw()
	return nil
}
