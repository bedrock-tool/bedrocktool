package worlds

import (
	"bytes"
	"errors"
	"fmt"
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

func (w *worldsHandler) storeReceivedChunk(pos world.ChunkPos, ch *worldstate.Chunk) error {
	if w.chunkStore != nil {
		return w.chunkStore(pos, ch)
	}
	return w.worldState.StoreChunk(pos, ch)
}

func (w *worldsHandler) handleLevelChunk(pk *packet.LevelChunk, timeReceived time.Time) (err error) {
	if len(pk.RawPayload) == 0 {
		w.log.Info(locale.Loc("empty_chunk", nil))
		return
	}

	if pk.CacheEnabled {
		return errors.New("cache is supposed to be handled in proxy")
	}

	/*
		var subChunkCount int
		switch pk.SubChunkCount {
		case protocol.SubChunkRequestModeLimited, protocol.SubChunkRequestModeLimitless:
			subChunkCount = 0
		default:
			subChunkCount = int(pk.SubChunkCount)
		}
	*/

	w.worldStateMu.Lock()
	defer w.worldStateMu.Unlock()

	defer func() { w.worldState.MarkIncomplete(err) }()

	levelChunk, blockNBTs, err := chunk.NetworkDecode(
		w.serverState.blocks,
		pk.RawPayload, int(pk.SubChunkCount),
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
		if _, ok := blockNBT["x"]; !ok {
			w.log.Warnf("The server sent invalid block data %#+v\n", blockNBT)
			continue
		}
		x := int(blockNBT["x"].(int32))
		y := int(blockNBT["y"].(int32))
		z := int(blockNBT["z"].(int32))
		ch.BlockEntities[cube.Pos{x, y, z}] = blockNBT
	}

	pos := world.ChunkPos(pk.Position)
	if w.scripting != nil {
		if !w.scripting.OnChunkAdd(pos, timeReceived) {
			w.worldState.IgnoredChunks[pos] = true
			return
		}
	}
	w.worldState.IgnoredChunks[pos] = false

	// request subchunks
	if subChunkLimit, ok := pk.SubChunkLimit.Value(); ok {
		max := w.worldState.Dimension().Range().Height() / 16
		if subChunkLimit != -1 {
			max = int(subChunkLimit)
		}

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
	}

	if err = w.storeReceivedChunk(pos, ch); err != nil {
		return fmt.Errorf("store level chunk %v: %w", pos, err)
	}

	if w.scripting != nil {
		w.scripting.OnChunkData(pos)
	}

	total, active := w.worldState.EntityCounts(w.serverState.getEntityRenderDistance())
	w.session.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{
		"Chunks":   len(w.worldState.StoredChunks),
		"Entities": fmt.Sprintf("%d (%d)", total, active),
		"Name":     w.worldState.Name,
	}, len(w.worldState.StoredChunks)))

	return nil
}

func (w *worldsHandler) processSubChunk(pk *packet.SubChunk) (err error) {
	w.worldStateMu.Lock()
	defer w.worldStateMu.Unlock()

	defer func() { w.worldState.MarkIncomplete(err) }()
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
			payload, ok := ent.RawPayload.Value()
			if !ok {
				continue
			}
			buf := bytes.NewBuffer(payload)
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
			if int(index) >= len(ch.Chunk.Sub()) {
				return fmt.Errorf("invalid sub-chunk index %d", index)
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
		if err = w.storeReceivedChunk(pos, ch); err != nil {
			return fmt.Errorf("store sub-chunk column %v: %w", pos, err)
		}
	}
	if w.scripting != nil {
		for pos := range chunks {
			w.scripting.OnChunkData(pos)
		}
	}

	w.mapUI.SchedRedraw()
	return nil
}
