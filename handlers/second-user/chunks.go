package seconduser

import (
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (s *secondaryUser) ResetWorld() {
	s.chunks = make(map[world.ChunkPos]*chunk.Chunk)
	s.blockNBT = make(map[protocol.SubChunkPos][]map[string]any)
}

func (s *secondaryUser) processChangeDimension(pk *packet.ChangeDimension) {
	s.ResetWorld()
	dimensionID := pk.Dimension
	if s.ispre118 && dimensionID == 0 {
		dimensionID += 10
	}
	s.dimension, _ = world.DimensionByID(int(dimensionID))
}

func (s *secondaryUser) processLevelChunk(pk *packet.LevelChunk) {
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

	ch, blockNBTs, err := chunk.NetworkDecode(world.AirRID(), pk.RawPayload, subChunkCount, s.dimension.Range(), s.ispre118, s.hasCustomBlocks)
	if err != nil {
		logrus.Error(err)
		return
	}
	if blockNBTs != nil {
		s.blockNBT[protocol.SubChunkPos{
			pk.Position.X(), 0, pk.Position.Z(),
		}] = blockNBTs
	}
	s.chunks[world.ChunkPos(pk.Position)] = ch

	for _, p := range s.server.Players() {
		p.Session().ViewChunk(world.ChunkPos(pk.Position), ch, nil)
	}

	max := s.dimension.Range().Height() / 16
	switch pk.SubChunkCount {
	case protocol.SubChunkRequestModeLimited:
		max = int(pk.HighestSubChunk)
		fallthrough
	case protocol.SubChunkRequestModeLimitless:
		var offsetTable []protocol.SubChunkOffset
		r := s.dimension.Range()
		for y := int8(r.Min() / 16); y < int8(r.Max()); y++ {
			offsetTable = append(offsetTable, protocol.SubChunkOffset{0, y, 0})
		}

		dimId, _ := world.DimensionID(s.dimension)
		s.proxy.Server.WritePacket(&packet.SubChunkRequest{
			Dimension: int32(dimId),
			Position: protocol.SubChunkPos{
				pk.Position.X(), 0, pk.Position.Z(),
			},
			Offsets: offsetTable[:max],
		})
	}
}

func (s *secondaryUser) processSubChunk(pk *packet.SubChunk) {
	offsets := make(map[world.ChunkPos]bool, len(pk.SubChunkEntries))
	for _, sub := range pk.SubChunkEntries {
		var (
			absX   = pk.Position[0] + int32(sub.Offset[0])
			absY   = pk.Position[1] + int32(sub.Offset[1])
			absZ   = pk.Position[2] + int32(sub.Offset[2])
			subPos = protocol.SubChunkPos{absX, absY, absZ}
			pos    = world.ChunkPos{absX, absZ}
		)
		offsets[pos] = true
		ch, ok := s.chunks[pos]
		if !ok {
			logrus.Error(locale.Loc("subchunk_before_chunk", nil))
			continue
		}
		blockNBT, err := ch.ApplySubChunkEntry(uint8(absY), &sub)
		if err != nil {
			logrus.Error(err)
		}
		if blockNBT != nil {
			s.blockNBT[subPos] = blockNBT
		}

		chunk.LightArea([]*chunk.Chunk{ch}, 0, 0).Fill()
	}

	for _, p := range s.server.Players() {
		for pos := range offsets {
			ch, ok := s.chunks[pos]
			if !ok {
				continue
			}
			p.Session().ViewChunk(pos, ch, nil)
		}

	}
}
