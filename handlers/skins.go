package handlers

import (
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type SkinSaver struct {
	PlayerNameFilter  string
	OnlyIfHasGeometry bool
	ServerName        string
	fpath             string
	proxy             *proxy.Context

	players map[uuid.UUID]*skinPlayer
}

type skinPlayer struct {
	UUID        uuid.UUID
	RuntimeID   uint64
	Name        string
	Position    mgl32.Vec3
	SkinPack    *utils.SkinPack
	CurrentSkin *utils.Skin
}

func (s *SkinSaver) AddPlayerSkin(playerID uuid.UUID, playerName string, skin *utils.Skin) (added bool) {
	p, ok := s.players[playerID]
	if !ok {
		p = &skinPlayer{}
		s.players[playerID] = p
	}
	if p.SkinPack == nil {
		p.SkinPack = utils.NewSkinPack(playerName, s.fpath)
		creating := fmt.Sprintf("Creating Skinpack for %s", playerName)
		logrus.Info(creating)
	}
	if p.SkinPack.AddSkin(skin) {
		addedStr := fmt.Sprintf("Added a skin %s", playerName)
		s.proxy.SendPopup(addedStr)
		logrus.Info(addedStr)
		added = true
	}
	if err := p.SkinPack.Save(path.Join(s.fpath, playerName)); err != nil {
		logrus.Error(err)
	}
	return added
}

func (s *SkinSaver) AddSkin(playerName string, playerID uuid.UUID, playerSkin *protocol.Skin) (string, *utils.Skin, bool) {
	p, ok := s.players[playerID]
	if !ok {
		p = &skinPlayer{}
		s.players[playerID] = p
	}
	if playerName == "" {
		if p.Name != "" {
			playerName = p.Name
		} else {
			playerName = playerID.String()
		}
	}

	if !strings.HasPrefix(playerName, s.PlayerNameFilter) {
		return playerName, nil, false
	}

	skin := &utils.Skin{Skin: playerSkin}
	p.CurrentSkin = skin
	if s.OnlyIfHasGeometry && !skin.HaveGeometry() {
		return playerName, nil, false
	}
	wasAdded := s.AddPlayerSkin(playerID, playerName, skin)

	return playerName, skin, wasAdded
}

type SkinAdd struct {
	PlayerName string
	Skin       *protocol.Skin
}

func (s *SkinSaver) ProcessPacket(pk packet.Packet) (out []SkinAdd) {
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		var player *skinPlayer
		for _, sp := range s.players {
			if sp.RuntimeID == pk.EntityRuntimeID {
				player = sp
				break
			}
		}
		if player == nil {
			return
		} else {
			player.Position = pk.Position
		}
	case *packet.MoveActorAbsolute:
		var player *skinPlayer
		for _, sp := range s.players {
			if sp.RuntimeID == pk.EntityRuntimeID {
				player = sp
				break
			}
		}
		if player == nil {
			return
		} else {
			player.Position = pk.Position
		}

	case *packet.PlayerList:
		if pk.ActionType == packet.PlayerListActionRemove { // remove
			return nil
		}
		for _, player := range pk.Entries {
			playerName, skin, wasAdded := s.AddSkin(utils.CleanupName(player.Username), player.UUID, &player.Skin)
			if wasAdded {
				out = append(out, SkinAdd{
					PlayerName: playerName,
					Skin:       skin.Skin,
				})
			}
		}
	case *packet.AddPlayer:
		p, ok := s.players[pk.UUID]
		if !ok {
			p = &skinPlayer{}
		}
		if p.Name == "" {
			p.Name = utils.CleanupName(pk.Username)
		}
		p.RuntimeID = pk.EntityRuntimeID
	case *packet.Animate:
		if pk.EntityRuntimeID == s.proxy.Player.RuntimeID && pk.ActionType == packet.AnimateActionSwingArm {
			s.stealSkin()
		}
	}
	return out
}

func NewSkinSaver(skinCB func(SkinAdd)) *proxy.Handler {
	s := &SkinSaver{
		players: make(map[uuid.UUID]*skinPlayer),
	}
	return &proxy.Handler{
		Name: "Skin Saver",
		ProxyRef: func(pc *proxy.Context) {
			s.proxy = pc
		},
		AddressAndName: func(address, hostname string) error {
			outPathBase := fmt.Sprintf("skins/%s", hostname)
			os.MkdirAll(outPathBase, 0o755)
			s.fpath = outPathBase
			return nil
		},
		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			for _, s := range s.ProcessPacket(pk) {
				if skinCB != nil {
					skinCB(s)
				}
			}
			return pk, nil
		},
	}
}

var playerBBox = cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3)

var rtTest = false

func (s *SkinSaver) stealSkin() {
	if !rtTest {
		return
	}
	logrus.Debugf("%d", len(s.players))

	var dist float64 = 4

	pitch := mgl64.DegToRad(float64(s.proxy.Player.Pitch))
	yaw := mgl64.DegToRad(float64(s.proxy.Player.HeadYaw + 90))

	dir := mgl64.Vec3{
		math.Cos(yaw) * math.Cos(pitch),
		math.Sin(-pitch),
		math.Sin(yaw) * math.Cos(pitch),
	}.Normalize()

	pos := s.proxy.Player.Position
	traceStart := mgl64.Vec3{float64(pos[0]), float64(pos[1]), float64(pos[2])}
	traceEnd := traceStart.Add(dir.Mul(dist))

	s.proxy.ClientWritePacket(&packet.SpawnParticleEffect{
		Dimension:      0,
		EntityUniqueID: -1,
		Position:       mgl32.Vec3{float32(traceEnd[0]), float32(traceEnd[1]), float32(traceEnd[2])},
		ParticleName:   "hivehub:emote_confounded",
	})

	for _, sp := range s.players {
		pos := mgl64.Vec3{float64(sp.Position[0]), float64(sp.Position[1]) - 1.8, float64(sp.Position[2])}
		bb := playerBBox.Translate(pos)

		res, ok := trace.BBoxIntercept(bb, traceStart, traceEnd)
		if ok {
			fmt.Printf("res: %v\n", res.Position())
			break
		}
	}
}
