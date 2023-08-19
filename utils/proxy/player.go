package proxy

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Player struct {
	RuntimeID           uint64
	Position            mgl32.Vec3
	Pitch, Yaw, HeadYaw float32
}

func (p *Player) handlePackets(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.StartGame:
		p.RuntimeID = pk.EntityRuntimeID
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == p.RuntimeID {
			p.Position = pk.Position
			p.Pitch = pk.Pitch
			p.Yaw = pk.Yaw
			p.HeadYaw = pk.HeadYaw
		}
	case *packet.PlayerAuthInput:
		p.Position = pk.Position
		p.Pitch = pk.Pitch
		p.Yaw = pk.Yaw
		p.HeadYaw = pk.HeadYaw
	}
}
