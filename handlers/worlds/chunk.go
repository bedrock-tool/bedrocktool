package worlds

import (
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/repeale/fp-go"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (w *worldsHandler) processChangeDimension(pk *packet.ChangeDimension) {
	w.SaveAndReset()
	dimensionID := pk.Dimension
	if w.serverState.ispre118 && dimensionID == 0 {
		dimensionID += 10
	}
	w.worldState.dimension, _ = world.DimensionByID(int(dimensionID))
}

func (w *worldsHandler) processLevelChunk(pk *packet.LevelChunk) {
	// ignore empty chunks THANKS WEIRD SERVER SOFTWARE DEVS
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

	ch, blockNBTs, err := chunk.NetworkDecode(world.AirRID(), pk.RawPayload, subChunkCount, w.worldState.dimension.Range(), w.serverState.ispre118, w.bp.HasBlocks())
	if err != nil {
		logrus.Error(err)
		return
	}
	if blockNBTs != nil {
		w.worldState.blockNBT[protocol.SubChunkPos{
			pk.Position.X(), 0, pk.Position.Z(),
		}] = blockNBTs
	}
	w.worldState.chunks[pk.Position] = ch

	max := w.worldState.dimension.Range().Height() / 16
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited:
		max = int(pk.HighestSubChunk)
		fallthrough
	case protocol.SubChunkRequestModeLimitless:
		var offsetTable []protocol.SubChunkOffset
		r := w.worldState.dimension.Range()
		for y := int8(r.Min() / 16); y < int8(r.Max()); y++ {
			offsetTable = append(offsetTable, protocol.SubChunkOffset{0, y, 0})
		}

		dimId, _ := world.DimensionID(w.worldState.dimension)
		w.proxy.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(dimId),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: offsetTable[:max],
		})
	default:
		// legacy
		empty := fp.Every(func(sub *chunk.SubChunk) bool {
			return sub.Empty()
		})(ch.Sub())
		if !empty {
			w.mapUI.SetChunk(pk.Position, ch, true)
		}
	}
}

func (w *worldsHandler) processSubChunk(pk *packet.SubChunk) {
	posToRedraw := make(map[protocol.ChunkPos]bool)

	for _, sub := range pk.SubChunkEntries {
		var (
			absX   = pk.Position[0] + int32(sub.Offset[0])
			absY   = pk.Position[1] + int32(sub.Offset[1])
			absZ   = pk.Position[2] + int32(sub.Offset[2])
			subPos = protocol.SubChunkPos{absX, absY, absZ}
			pos    = protocol.ChunkPos{absX, absZ}
		)
		ch, ok := w.worldState.chunks[pos]
		if !ok {
			logrus.Error(locale.Loc("subchunk_before_chunk", nil))
			continue
		}
		blockNBT, err := ch.ApplySubChunkEntry(uint8(absY), &sub)
		if err != nil {
			logrus.Error(err)
		}
		if blockNBT != nil {
			w.worldState.blockNBT[subPos] = blockNBT
		}

		posToRedraw[pos] = true
	}

	// redraw the chunks
	for pos := range posToRedraw {
		w.mapUI.SetChunk(pos, w.worldState.chunks[pos], true)
	}
	w.mapUI.SchedRedraw()
}

func blockPosInChunk(pos protocol.BlockPos) (uint8, int16, uint8) {
	return uint8(pos.X() & 0x0f), int16(pos.Y() & 0x0f), uint8(pos.Z() & 0x0f)
}

func (w *worldsHandler) ProcessChunkPackets(pk packet.Packet) packet.Packet {
	switch pk := pk.(type) {
	case *packet.ChangeDimension:
		w.processChangeDimension(pk)
	case *packet.LevelChunk:
		w.processLevelChunk(pk)
		w.proxy.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{
			"Count": len(w.worldState.chunks),
			"Name":  w.worldState.Name,
		}, len(w.worldState.chunks)))
	case *packet.SubChunk:
		w.processSubChunk(pk)
	case *packet.BlockActorData:
		if w.settings.BlockUpdates {
			sp := protocol.SubChunkPos{pk.Position.X() << 4, 0, pk.Position.Z() << 4}
			b, ok := w.worldState.blockNBT[sp]
			if !ok {
				w.worldState.blockNBT[sp] = []map[string]any{pk.NBTData}
			} else {
				for i, v := range b {
					x, y, z := v["x"].(int32), v["y"].(int32), v["z"].(int32)
					if x == pk.Position.X() && y == pk.Position.Y() && z == pk.Position.Z() {
						b[i] = pk.NBTData
						break
					}
				}
			}
		}
	case *packet.UpdateBlock:
		if w.settings.BlockUpdates {
			cp := protocol.ChunkPos{pk.Position.X() >> 4, pk.Position.Z() >> 4}
			c, ok := w.worldState.chunks[cp]
			if ok {
				x, y, z := blockPosInChunk(pk.Position)
				c.SetBlock(x, y, z, uint8(pk.Layer), pk.NewBlockRuntimeID)
				w.mapUI.SetChunk(cp, w.worldState.chunks[cp], true)
			}
		}
	case *packet.UpdateSubChunkBlocks:
		if w.settings.BlockUpdates {
			cp := protocol.ChunkPos{pk.Position.X(), pk.Position.Z()}
			c, ok := w.worldState.chunks[cp]
			if ok {
				for _, bce := range pk.Blocks {
					x, y, z := blockPosInChunk(bce.BlockPos)
					if bce.SyncedUpdateType == packet.BlockToEntityTransition {
						c.SetBlock(x, y, z, 0, world.AirRID())
					} else {
						c.SetBlock(x, y, z, 0, bce.BlockRuntimeID)
					}
				}
				w.mapUI.SetChunk(cp, w.worldState.chunks[cp], true)
			}
		}
	}
	return pk
}
