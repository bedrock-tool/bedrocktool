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
	dim_id := pk.Dimension
	if w.ispre118 {
		dim_id += 10
	}
	w.Dim = dimension_ids[uint8(dim_id)]
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
			Offsets: Offset_table[:max],
		})
	}
}

func (w *WorldState) processSubChunk(pk *packet.SubChunk) {
	pos_to_redraw := make(map[protocol.ChunkPos]bool)

	for _, sub := range pk.SubChunkEntries {
		var (
			abs_x  = pk.Position[0] + int32(sub.Offset[0])
			abs_y  = pk.Position[1] + int32(sub.Offset[1])
			abs_z  = pk.Position[2] + int32(sub.Offset[2])
			subpos = protocol.SubChunkPos{abs_x, abs_y, abs_z}
			pos    = protocol.ChunkPos{abs_x, abs_z}
		)
		ch := w.chunks[pos]
		if ch == nil {
			logrus.Error(locale.Loc("subchunk_before_chunk", nil))
			continue
		}
		blockNBT, err := ch.ApplySubChunkEntry(uint8(abs_y), &sub)
		if err != nil {
			logrus.Error(err)
		}
		if blockNBT != nil {
			w.blockNBT[subpos] = blockNBT
		}

		pos_to_redraw[pos] = true
	}

	// redraw the chunks
	for pos := range pos_to_redraw {
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
