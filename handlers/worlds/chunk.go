package worlds

import (
	"bytes"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/gregwebs/go-recovery"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (w *worldsHandler) processChangeDimension(pk *packet.ChangeDimension) {
	dimensionID := pk.Dimension
	if w.serverState.useOldBiomes && dimensionID == 0 {
		dimensionID += 10
	}
	dim, _ := world.DimensionByID(int(dimensionID))
	w.SaveAndReset(false, dim)
}

func (w *worldsHandler) processLevelChunk(pk *packet.LevelChunk) {
	if len(pk.RawPayload) == 0 {
		logrus.Info(locale.Loc("empty_chunk", nil))
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

	ch, blockNBTs, err := chunk.NetworkDecode(world.AirRID(), pk.RawPayload, subChunkCount, w.serverState.useOldBiomes, w.serverState.useHashedRids, w.currentWorldState.dimension.Range())
	if err != nil {
		logrus.Error(err)
		return
	}
	var chunkBlockNBT = make(map[cube.Pos]dummyBlock)
	for _, blockNBT := range blockNBTs {
		x := int(blockNBT["x"].(int32))
		y := int(blockNBT["y"].(int32))
		z := int(blockNBT["z"].(int32))
		chunkBlockNBT[cube.Pos{x, y, z}] = dummyBlock{
			id:  blockNBT["id"].(string),
			nbt: blockNBT,
		}
	}

	pos := world.ChunkPos(pk.Position)
	if w.scripting.CB.OnChunkAdd != nil {
		var ignore bool
		err := recovery.Call(func() error {
			ignore = w.scripting.CB.OnChunkAdd(pos)
			return nil
		})
		if err != nil {
			logrus.Errorf("Scripting: %s", err)
		}
		if ignore {
			return
		}
	}
	w.currentWorldState.storeChunk(pos, ch, chunkBlockNBT)

	max := w.currentWorldState.dimension.Range().Height() / 16
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited:
		max = int(pk.HighestSubChunk)
		fallthrough
	case protocol.SubChunkRequestModeLimitless:
		var offsetTable []protocol.SubChunkOffset
		r := w.currentWorldState.dimension.Range()
		for y := int8(r.Min() / 16); y < int8(r.Max()/16)+1; y++ {
			offsetTable = append(offsetTable, protocol.SubChunkOffset{0, y, 0})
		}

		dimId, _ := world.DimensionID(w.currentWorldState.dimension)
		_ = w.proxy.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(dimId),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: offsetTable[:min(max+1, len(offsetTable))],
		})
	default:
		// legacy
		var empty = true
		for _, sub := range ch.Sub() {
			if !sub.Empty() {
				empty = false
				break
			}
		}
		if !empty {
			w.mapUI.SetChunk((world.ChunkPos)(pk.Position), ch, w.currentWorldState.deferredState != nil)
		}
	}

	w.proxy.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{
		"Count": len(w.currentWorldState.storedChunks),
		"Name":  w.currentWorldState.Name,
	}, len(w.currentWorldState.storedChunks)))
}

func (w *worldsHandler) processSubChunk(pk *packet.SubChunk) error {
	var chunks = make(map[world.ChunkPos]*chunk.Chunk)
	var blockNBTs = make(map[world.ChunkPos]map[cube.Pos]dummyBlock)

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

		if _, ok := chunks[pos]; ok {
			continue
		}
		col, err := w.currentWorldState.state.provider.LoadColumn(pos, w.currentWorldState.dimension)
		if err != nil {
			return err
		}
		chunks[pos] = col.Chunk
		blockNBTs[pos] = make(map[cube.Pos]dummyBlock)
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
				world.AirRID(),
				w.currentWorldState.dimension.Range(),
				&index,
				chunk.NetworkEncoding,
			)
			if err != nil {
				return err
			}

			ch := chunks[pos]
			ch.Sub()[index] = sub

			if buf.Len() > 0 {
				dec := nbt.NewDecoderWithEncoding(buf, nbt.NetworkLittleEndian)
				for buf.Len() > 0 {
					blockNBT := make(map[string]any, 0)
					err = dec.Decode(&blockNBT)
					if err != nil {
						return err
					}
					x := int(blockNBT["x"].(int32))
					y := int(blockNBT["y"].(int32))
					z := int(blockNBT["z"].(int32))
					id := blockNBT["id"].(string)

					blockNBTs[pos][cube.Pos{x, y, z}] = dummyBlock{
						id:  id,
						nbt: blockNBT,
					}
				}
			}
		}
	}

	for cp, c := range chunks {
		w.currentWorldState.storeChunk(cp, c, blockNBTs[cp])
		w.mapUI.SetChunk(cp, c, w.currentWorldState.deferredState != nil)
	}

	w.mapUI.SchedRedraw()
	return nil
}

func (w *worldsHandler) handleChunkPackets(pk packet.Packet) (packet.Packet, error) {
	switch pk := pk.(type) {
	case *packet.ChangeDimension:
		w.processChangeDimension(pk)
	case *packet.LevelChunk:
		w.processLevelChunk(pk)
	case *packet.SubChunk:
		if err := w.processSubChunk(pk); err != nil {
			logrus.Error(err)
		}
	case *packet.BlockActorData:
		p := pk.Position
		pos := cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}
		w.currentWorldState.State().SetBlockNBT(pos, pk.NBTData, false)
		/*
			case *packet.UpdateBlock:
				if w.settings.BlockUpdates {
					cp := world.ChunkPos{pk.Position.X() >> 4, pk.Position.Z() >> 4}
					c, ok := w.worldState.state().chunks[cp]
					if ok {
						x, y, z := blockPosInChunk(pk.Position)
						c.SetBlock(x, y, z, uint8(pk.Layer), pk.NewBlockRuntimeID)
						w.mapUI.SetChunk(cp, c, w.worldState.useDeferred)
					}
				}
			case *packet.UpdateSubChunkBlocks:
				if w.settings.BlockUpdates {
					cp := world.ChunkPos{pk.Position.X(), pk.Position.Z()}
					c, ok := w.worldState.state().chunks[cp]
					if ok {
						for _, bce := range pk.Blocks {
							x, y, z := blockPosInChunk(bce.BlockPos)
							if bce.SyncedUpdateType == packet.BlockToEntityTransition {
								c.SetBlock(x, y, z, 0, world.AirRID())
							} else {
								c.SetBlock(x, y, z, 0, bce.BlockRuntimeID)
							}
						}
						w.mapUI.SetChunk(cp, c, w.worldState.useDeferred)
					}
				}
		*/
	}
	return pk, nil
}
