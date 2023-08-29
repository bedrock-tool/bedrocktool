package worlds

import (
	"bytes"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"github.com/thomaso-mirodin/intmath/i32"
)

func (w *worldsHandler) processChangeDimension(pk *packet.ChangeDimension) {
	w.SaveAndReset()
	dimensionID := pk.Dimension
	if w.serverState.useOldBiomes && dimensionID == 0 {
		dimensionID += 10
	}
	w.worldState.dimension, _ = world.DimensionByID(int(dimensionID))
}

func (w *worldsHandler) processLevelChunk(pk *packet.LevelChunk) {
	if len(pk.RawPayload) == 0 {
		logrus.Info(locale.Loc("empty_chunk", nil))
		return
	}

	var subChunkCount int
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited:
		fallthrough
	case protocol.SubChunkRequestModeLimitless:
		subChunkCount = 0
	default:
		subChunkCount = int(pk.SubChunkCount)
	}

	ch, blockNBTs, err := chunk.NetworkDecode(world.AirRID(), pk.RawPayload, subChunkCount, w.serverState.useOldBiomes, w.serverState.useHashedRids, w.worldState.dimension.Range())
	if err != nil {
		logrus.Error(err)
		return
	}
	var chunkBlockNBT = make(map[cube.Pos]world.Block)
	for _, blockNBT := range blockNBTs {
		x := int(blockNBT["x"].(int32))
		y := int(blockNBT["y"].(int32))
		z := int(blockNBT["z"].(int32))
		chunkBlockNBT[cube.Pos{x, y, z}] = &dummyBlock{
			id:  blockNBT["id"].(string),
			nbt: blockNBT,
		}
	}
	w.worldState.storeChunk(world.ChunkPos(pk.Position), ch, chunkBlockNBT)

	max := uint16(w.worldState.dimension.Range().Height() / 16)
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited:
		max = uint16(pk.HighestSubChunk)
		fallthrough
	case protocol.SubChunkRequestModeLimitless:
		var offsetTable []protocol.SubChunkOffset
		r := w.worldState.dimension.Range()
		for y := int8(r.Min() / 16); y < int8(r.Max()/16)+1; y++ {
			offsetTable = append(offsetTable, protocol.SubChunkOffset{0, y, 0})
		}

		dimId, _ := world.DimensionID(w.worldState.dimension)
		w.proxy.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(dimId),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: offsetTable[:i32.Min(int32(max+1), int32(len(offsetTable)))],
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
			w.mapUI.SetChunk((world.ChunkPos)(pk.Position), ch, w.worldState.useDeferred)
		}
	}

	w.proxy.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{
		"Count": len(w.worldState.storedChunks),
		"Name":  w.worldState.Name,
	}, len(w.worldState.storedChunks)))
}

func (w *worldsHandler) processSubChunk(pk *packet.SubChunk) error {
	posToRedraw := make(map[world.ChunkPos]bool)

	var chunks = make(map[world.ChunkPos]*chunk.Chunk)
	for _, ent := range pk.SubChunkEntries {
		var (
			absX = pk.Position[0] + int32(ent.Offset[0])
			absZ = pk.Position[2] + int32(ent.Offset[2])
			pos  = world.ChunkPos{absX, absZ}
		)

		col, err := w.worldState.State.provider.LoadColumn(pos, w.worldState.dimension)
		if err != nil {
			return err
		}
		chunks[pos] = col.Chunk
	}

	var blockNBTs = make(map[world.ChunkPos]map[cube.Pos]world.Block)

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
				w.worldState.dimension.Range(),
				&index,
				chunk.NetworkEncoding,
			)
			if err != nil {
				return err
			}

			ch := chunks[pos]
			ch.Sub()[index] = sub

			blockNBTs[pos] = make(map[cube.Pos]world.Block)

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
					blockNBTs[pos][cube.Pos{x, y, z}] = &dummyBlock{
						id:  blockNBT["id"].(string),
						nbt: blockNBT,
					}
				}
			}
		}
		posToRedraw[pos] = true
	}

	for cp, c := range chunks {
		w.worldState.storeChunk(cp, c, blockNBTs[cp])
	}

	// redraw the chunks
	for pos := range posToRedraw {
		w.mapUI.SetChunk(pos, chunks[pos], w.worldState.useDeferred)
	}
	w.mapUI.SchedRedraw()
	return nil
}

func blockPosInChunk(pos protocol.BlockPos) (uint8, int16, uint8) {
	return uint8(pos.X() & 0x0f), int16(pos.Y() & 0x0f), uint8(pos.Z() & 0x0f)
}

func (w *worldsHandler) handleChunkPackets(pk packet.Packet) packet.Packet {
	switch pk := pk.(type) {
	case *packet.ChangeDimension:
		w.processChangeDimension(pk)
	case *packet.LevelChunk:
		w.processLevelChunk(pk)
	case *packet.SubChunk:
		w.processSubChunk(pk)
		/*
			case *packet.BlockActorData:
				p := pk.Position
				w.worldState.state().blockNBTs[cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}] = pk.NBTData

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
	return pk
}
