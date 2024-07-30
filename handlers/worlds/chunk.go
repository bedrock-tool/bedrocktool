package worlds

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/cespare/xxhash/v2"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (w *worldsHandler) blobKey(b uint64) []byte {
	k := binary.LittleEndian.AppendUint64(nil, xxhash.Sum64String(w.serverState.Name))
	k = binary.LittleEndian.AppendUint64(k, b)
	return k
}

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
		w.blobLock.Lock()
		defer w.blobLock.Unlock()

		var reply packet.ClientCacheBlobStatus
		var blobs []protocol.CacheBlob
		for _, h := range pk.BlobHashes {
			blob, err := w.blobcache.Get(w.blobKey(h), nil)
			if err != nil {
				if errors.Is(err, leveldb.ErrNotFound) {
					reply.MissHashes = append(reply.MissHashes, h)
					continue
				}
				return err
			}
			//reply.HitHashes = append(reply.HitHashes, h)
			blobs = append(blobs, protocol.CacheBlob{Hash: h, Payload: blob})
		}
		err = w.processBlobs(blobs, false)
		if err != nil {
			return err
		}

		err = w.session.Server.WritePacket(&reply)
		if err != nil {
			return err
		}
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

	if !pk.CacheEnabled {
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
	}

	w.session.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{
		"Chunks":   len(w.currentWorld.StoredChunks),
		"Entities": w.currentWorld.EntityCount(),
		"Name":     w.currentWorld.Name,
	}, len(w.currentWorld.StoredChunks)))

	return nil
}

func (w *worldsHandler) processBlobs(blobs []protocol.CacheBlob, put bool) (err error) {
	// no lock!
	var chunks = make(map[world.ChunkPos]*chunk.Chunk)
	for _, blob := range blobs {
		if put {
			err = w.blobcache.Put(w.blobKey(blob.Hash), blob.Payload, nil)
			if err != nil {
				return err
			}
		}
		wb := w.waitingBlobs[blob.Hash]

		if w.currentWorld.IgnoredChunks[wb.pos] {
			continue
		}
		if _, ok := chunks[wb.pos]; ok {
			continue
		}
		ch, ok, err := w.currentWorld.LoadChunk(wb.pos)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("bug check: subchunk received before chunk")
		}
		chunks[wb.pos] = ch
	}

	for _, blob := range blobs {
		wb := w.waitingBlobs[blob.Hash]
		ch, ok := chunks[wb.pos]
		if !ok {
			continue
		}

		buf := bytes.NewBuffer(blob.Payload)
		if wb.biome {
			err = chunk.DecodeNetworkBiomes(ch, buf, w.serverState.useOldBiomes)
			if err != nil {
				return err
			}
		} else {
			var index byte
			ch.Sub()[index], err = chunk.DecodeSubChunk(
				buf,
				w.serverState.blocks,
				w.currentWorld.Range(),
				&index,
				chunk.NetworkEncoding,
				w.serverState.useHashedRids,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *worldsHandler) processSubChunk(pk *packet.SubChunk) error {
	var chunks = make(map[world.ChunkPos]*chunk.Chunk)
	var blockNBTs = make(map[world.ChunkPos]map[cube.Pos]worldstate.DummyBlock)

	w.worldStateLock.Lock()
	defer w.worldStateLock.Unlock()

	for _, ent := range pk.SubChunkEntries {
		if ent.Result != protocol.SubChunkResultSuccess {
			continue
		}
		var (
			absX = pk.Position[0] + int32(ent.Offset[0])
			absZ = pk.Position[2] + int32(ent.Offset[2])
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

	for _, ent := range pk.SubChunkEntries {
		var (
			absX = pk.Position[0] + int32(ent.Offset[0])
			absY = pk.Position[1] + int32(ent.Offset[1])
			absZ = pk.Position[2] + int32(ent.Offset[2])
			pos  = world.ChunkPos{absX, absZ}
		)

		switch ent.Result {
		case protocol.SubChunkResultSuccessAllAir:
		case protocol.SubChunkResultSuccess:
			buf := bytes.NewBuffer(ent.RawPayload)
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

			if buf.Len() > 0 {
				dec := nbt.NewDecoderWithEncoding(buf, nbt.NetworkLittleEndian)
				for buf.Len() > 0 {
					blockNBT := make(map[string]any, 0)
					if err = dec.Decode(&blockNBT); err != nil {
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

	w.mapUI.SchedRedraw()
	return nil
}
