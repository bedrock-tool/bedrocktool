package worlds

import (
	"bytes"
	"errors"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (w *worldsHandler) processLevelChunk(pk *packet.LevelChunk, timeReceived time.Time) (err error) {
	if len(pk.RawPayload) == 0 {
		w.log.Info(locale.Loc("empty_chunk", nil))
		return
	}

	var subChunkCount int
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited, protocol.SubChunkRequestModeLimitless:
		subChunkCount = 0
	default:
		subChunkCount = int(pk.SubChunkCount)
	}

	w.worldStateLock.Lock()
	defer w.worldStateLock.Unlock()

	//os.WriteFile("chunk.bin", pk.RawPayload, 0777)

	var ch *chunk.Chunk
	var blockNBTs []map[string]any

	if pk.CacheEnabled {
		ch = chunk.New(w.serverState.blocks, w.currentWorld.Range())

		buf := bytes.NewBuffer(pk.RawPayload)
		borderBlocks, _ := buf.ReadByte()
		buf.Next(int(borderBlocks))

		blockNBTs, err = chunk.DecodeBlockNBTs(buf)
		if err != nil {
			return err
		}
	} else {
		ch, blockNBTs, err = chunk.NetworkDecode(
			w.serverState.blocks,
			pk.RawPayload, subChunkCount,
			w.serverState.useOldBiomes,
			w.serverState.useHashedRids,
			w.currentWorld.Range(),
		)
		if err != nil {
			return err
		}
	}

	var chunkBlockNBT = make(map[cube.Pos]worldstate.DummyBlock)
	for _, blockNBT := range blockNBTs {
		x := int(blockNBT["x"].(int32))
		y := int(blockNBT["y"].(int32))
		z := int(blockNBT["z"].(int32))
		chunkBlockNBT[cube.Pos{x, y, z}] = worldstate.DummyBlock{
			ID:  blockNBT["id"].(string),
			NBT: blockNBT,
		}
	}

	pos := world.ChunkPos(pk.Position)
	if !w.scripting.OnChunkAdd(pos, timeReceived) {
		w.currentWorld.IgnoredChunks[pos] = true
		return
	}
	w.currentWorld.IgnoredChunks[pos] = false

	err = w.currentWorld.StoreChunk(pos, ch, chunkBlockNBT)
	if err != nil {
		w.log.Error(err)
	}

	max := w.currentWorld.Dimension().Range().Height() / 16
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited:
		max = int(pk.HighestSubChunk)
		fallthrough
	case protocol.SubChunkRequestModeLimitless:
		var offsetTable []protocol.SubChunkOffset
		r := w.currentWorld.Dimension().Range()
		for y := int8(r.Min() / 16); y < int8(r.Max()/16)+1; y++ {
			offsetTable = append(offsetTable, protocol.SubChunkOffset{0, y, 0})
		}

		dimId, _ := world.DimensionID(w.currentWorld.Dimension())
		_ = w.session.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(dimId),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: offsetTable[:min(max+1, len(offsetTable))],
		})
	default:
	}

	w.session.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{
		"Chunks":   len(w.currentWorld.StoredChunks),
		"Entities": w.currentWorld.EntityCount(),
		"Name":     w.currentWorld.Name,
	}, len(w.currentWorld.StoredChunks)))

	return nil
}

func (w *worldsHandler) onBlobs(blobs []proxy.BlobResp, fromCache bool) (err error) {
	var subchunks = make(map[world.SubChunkPos][]protocol.SubChunkEntry)
	var biomes = make(map[world.SubChunkPos][]byte)
	for _, blob := range blobs {
		if blob.Entry != nil {
			ent := *blob.Entry
			ent.RawPayload = blob.Payload
			subchunks[blob.Position] = append(subchunks[blob.Position], ent)
		} else {
			biomes[blob.Position] = blob.Payload
		}
	}

	for pos, sc := range subchunks {
		err = w.processSubChunkEntries(pos, sc, false)
		if err != nil {
			return err
		}
	}

	w.worldStateLock.Lock()
	defer w.worldStateLock.Unlock()
	for pos, biome := range biomes {
		wpos := world.ChunkPos{pos[0], pos[2]}
		ch, ok, err := w.currentWorld.LoadChunk(wpos)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		buf := bytes.NewBuffer(biome)
		err = chunk.DecodeNetworkBiomes(ch, buf, w.serverState.useOldBiomes)
		if err != nil {
			return err
		}

		err = w.currentWorld.StoreChunk(wpos, ch, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *worldsHandler) processSubChunk(pk *packet.SubChunk) error {
	err := w.processSubChunkEntries(world.SubChunkPos(pk.Position), pk.SubChunkEntries, pk.CacheEnabled)
	if err != nil {
		return err
	}
	w.mapUI.SchedRedraw()
	return nil
}

func (w *worldsHandler) processSubChunkEntries(Position world.SubChunkPos, SubChunkEntries []protocol.SubChunkEntry, CacheEnabled bool) error {
	w.worldStateLock.Lock()
	defer w.worldStateLock.Unlock()

	var chunks = make(map[world.ChunkPos]*chunk.Chunk)
	var blockNBTs = make(map[world.ChunkPos]map[cube.Pos]worldstate.DummyBlock)

	for _, ent := range SubChunkEntries {
		if ent.Result != protocol.SubChunkResultSuccess {
			continue
		}
		var (
			absX = Position[0] + int32(ent.Offset[0])
			absZ = Position[2] + int32(ent.Offset[2])
			pos  = world.ChunkPos{absX, absZ}
		)

		if w.currentWorld.IgnoredChunks[pos] {
			continue
		}

		if _, ok := chunks[pos]; ok {
			continue
		}
		ch, ok, err := w.currentWorld.LoadChunk(pos)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("bug check: subchunk received before chunk")
		}
		chunks[pos] = ch
		blockNBTs[pos] = make(map[cube.Pos]worldstate.DummyBlock)
	}

	for _, ent := range SubChunkEntries {
		var (
			absX = Position[0] + int32(ent.Offset[0])
			absY = Position[1] + int32(ent.Offset[1])
			absZ = Position[2] + int32(ent.Offset[2])
			pos  = world.ChunkPos{absX, absZ}
		)

		switch ent.Result {
		case protocol.SubChunkResultSuccessAllAir:
		case protocol.SubChunkResultSuccess:
			buf := bytes.NewBuffer(ent.RawPayload)
			if !CacheEnabled {
				index := uint8(absY)
				sub, err := chunk.DecodeSubChunk(
					buf,
					w.serverState.blocks,
					w.currentWorld.Dimension().Range(),
					&index,
					chunk.NetworkEncoding,
					w.serverState.useHashedRids,
				)
				if err != nil {
					return err
				}

				ch, ok := chunks[pos]
				if !ok {
					continue
				}
				ch.Sub()[index] = sub
			}

			if buf.Len() > 0 {
				dec := nbt.NewDecoderWithEncoding(buf, nbt.NetworkLittleEndian)
				for buf.Len() > 0 {
					blockNBT := make(map[string]any, 0)
					if err := dec.Decode(&blockNBT); err != nil {
						return err
					}
					blockNBTs[pos][cube.Pos{
						int(blockNBT["x"].(int32)),
						int(blockNBT["y"].(int32)),
						int(blockNBT["z"].(int32)),
					}] = worldstate.DummyBlock{
						ID:  blockNBT["id"].(string),
						NBT: blockNBT,
					}
				}
			}
		}
	}

	for cp, c := range chunks {
		w.currentWorld.StoreChunk(cp, c, blockNBTs[cp])
	}

	return nil
}
