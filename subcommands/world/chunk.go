package world

import (
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (w *WorldState) processChangeDimension(pk *packet.ChangeDimension) {
	if len(w.chunks) > 0 {
		w.SaveAndReset()
	} else {
		logrus.Info(locale.Loc("not_saving_empty", nil))
		w.Reset()
	}
	dimensionID := pk.Dimension
	if w.ispre118 {
		dimensionID += 10
	}
	w.Dim = dimensionIDMap[uint8(dimensionID)]
}

func (w *WorldState) processLevelChunk(pk *packet.LevelChunk) {
	_, exists := w.chunks[pk.Position]
	if exists {
		return
	}

	ch, blockNBTs, err := chunk.NetworkDecode(world.AirRID(), pk.RawPayload, int(pk.SubChunkCount), w.Dim.Range(), w.ispre118, w.bp != nil)
	if err != nil {
		logrus.Error(err)
		return
	}
	if blockNBTs != nil {
		w.blockNBT[protocol.SubChunkPos{
			pk.Position.X(), 0, pk.Position.Z(),
		}] = blockNBTs
	}

	w.chunks[pk.Position] = ch

	if pk.SubChunkRequestMode == protocol.SubChunkRequestModeLegacy {
		w.ui.SetChunk(pk.Position, ch)
	} else {
		w.ui.SetChunk(pk.Position, nil)
		// request all the subchunks

		max := w.Dim.Range().Height() / 16
		if pk.SubChunkRequestMode == protocol.SubChunkRequestModeLimited {
			max = int(pk.HighestSubChunk)
		}

		w.proxy.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(w.Dim.EncodeDimension()),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: offsetTable[:max],
		})
	}
}

func (w *WorldState) processSubChunk(pk *packet.SubChunk) {
	posToRedraw := make(map[protocol.ChunkPos]bool)

	for _, sub := range pk.SubChunkEntries {
		var (
			absX   = pk.Position[0] + int32(sub.Offset[0])
			absY   = pk.Position[1] + int32(sub.Offset[1])
			absZ   = pk.Position[2] + int32(sub.Offset[2])
			subPos = protocol.SubChunkPos{absX, absY, absZ}
			pos    = protocol.ChunkPos{absX, absZ}
		)
		ch, ok := w.chunks[pos]
		if !ok {
			logrus.Error(locale.Loc("subchunk_before_chunk", nil))
			continue
		}
		blockNBT, err := ch.ApplySubChunkEntry(uint8(absY), &sub)
		if err != nil {
			logrus.Error(err)
		}
		if blockNBT != nil {
			w.blockNBT[subPos] = blockNBT
		}

		posToRedraw[pos] = true
	}

	// redraw the chunks
	for pos := range posToRedraw {
		w.ui.SetChunk(pos, w.chunks[pos])
	}
	w.ui.SchedRedraw()
}

func (w *WorldState) ProcessChunkPackets(pk packet.Packet) packet.Packet {
	switch pk := pk.(type) {
	case *packet.ChangeDimension:
		w.processChangeDimension(pk)
	case *packet.LevelChunk:
		w.processLevelChunk(pk)

		w.proxy.SendPopup(locale.Locm("popup_chunk_count", locale.Strmap{"Count": len(w.chunks), "Name": w.WorldName}, len(w.chunks)))
	case *packet.SubChunk:
		w.processSubChunk(pk)
	}
	return pk
}
