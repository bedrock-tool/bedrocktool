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
	session *proxy.Session
	log     *logrus.Entry

	PlayerNameFilter  string
	OnlyIfHasGeometry bool
	baseDir           string

	players map[uuid.UUID]*skinPlayer
}

type skinPlayer struct {
	UUID        uuid.UUID
	RuntimeID   uint64
	Name        string
	AltName     string
	Position    mgl32.Vec3
	SkinPack    *utils.SkinPack
	CurrentSkin *utils.Skin
	gone        bool
}

func (s *SkinSaver) AddOrGetPlayer(name string, id uuid.UUID) *skinPlayer {
	if player, ok := s.players[id]; ok {
		return player
	}

	player := &skinPlayer{
		UUID: id,
		Name: utils.CleanupName(name),
	}

	if player.Name == "" && name != "" {
		player.Name = utils.CleanupName(name)
	}

	s.players[id] = player

	return player
}

func (s *SkinSaver) AddSkin(player *skinPlayer, playerSkin *protocol.Skin) (*utils.Skin, bool) {
	if len(player.Name) == 0 {
		if len(player.AltName) > 0 {
			player.Name = player.AltName
		} else {
			player.Name = player.UUID.String()
		}
	}

	if !strings.HasPrefix(player.Name, s.PlayerNameFilter) {
		return nil, false
	}

	skin := &utils.Skin{Skin: playerSkin}
	player.CurrentSkin = skin
	if s.OnlyIfHasGeometry && !skin.HaveGeometry() {
		return nil, false
	}

	var added bool
	if player.SkinPack == nil {
		player.SkinPack = utils.NewSkinPack(player.Name, s.baseDir)
		creating := fmt.Sprintf("Creating Skinpack for %s", player.Name)
		s.log.Info(creating)
	}
	if player.SkinPack.AddSkin(skin) {
		addedStr := fmt.Sprintf("Added a skin %s", player.Name)
		s.session.SendPopup(addedStr)
		s.log.Info(addedStr)
		added = true
	}
	if err := player.SkinPack.Save(path.Join(s.baseDir, player.Name)); err != nil {
		s.log.Error(err)
	}

	return skin, added
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
		for _, player := range s.players {
			if player.RuntimeID == pk.EntityRuntimeID {
				player.Position = pk.Position
				break
			}
		}

	case *packet.PlayerList:
		if pk.ActionType == packet.PlayerListActionRemove { // remove
			return nil
		}
		for _, playerEntry := range pk.Entries {
			player := s.AddOrGetPlayer(playerEntry.Username, playerEntry.UUID)
			skin, wasAdded := s.AddSkin(player, &playerEntry.Skin)
			if wasAdded {
				out = append(out, SkinAdd{
					PlayerName: player.Name,
					Skin:       skin.Skin,
				})
			}
		}
	case *packet.PlayerSkin:
		player := s.AddOrGetPlayer("", pk.UUID)
		skin, wasAdded := s.AddSkin(player, &pk.Skin)
		if wasAdded {
			out = append(out, SkinAdd{
				PlayerName: player.Name,
				Skin:       skin.Skin,
			})
		}
	case *packet.AddPlayer:
		player := s.AddOrGetPlayer("", pk.UUID)
		player.AltName = pk.Username
		player.RuntimeID = pk.EntityRuntimeID
	case *packet.Animate:
		if pk.EntityRuntimeID == s.session.Player.RuntimeID && pk.ActionType == packet.AnimateActionSwingArm {
			s.stealSkin()
		}
	case *packet.ChangeDimension:
		for _, sp := range s.players {
			sp.gone = true
		}
	case *packet.RemoveActor:
		for _, sp := range s.players {
			if sp.RuntimeID == uint64(pk.EntityUniqueID) {
				sp.gone = true
				break
			}
		}
	}
	return out
}

func NewSkinSaver(skinCB func(SkinAdd)) *proxy.Handler {
	s := &SkinSaver{
		players: make(map[uuid.UUID]*skinPlayer),
		log:     logrus.WithField("part", "SkinSaver"),
	}
	return &proxy.Handler{
		Name: "Skin Saver",
		SessionStart: func(session *proxy.Session, hostname string) error {
			s.session = session
			outPathBase := fmt.Sprintf("skins/%s", hostname)
			os.MkdirAll(outPathBase, 0o755)
			s.baseDir = outPathBase
			return nil
		},
		PacketCallback: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
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
	s.log.Debugf("%d", len(s.players))

	var dist float64 = 40

	pitch := mgl64.DegToRad(float64(s.session.Player.Pitch))
	yaw := mgl64.DegToRad(float64(s.session.Player.HeadYaw + 90))

	dir := mgl64.Vec3{
		math.Cos(yaw) * math.Cos(pitch),
		math.Sin(-pitch),
		math.Sin(yaw) * math.Cos(pitch),
	}.Normalize()

	pos := s.session.Player.Position
	traceStart := mgl64.Vec3{float64(pos[0]), float64(pos[1]), float64(pos[2])}
	traceEnd := traceStart.Add(dir.Mul(dist))
	s.session.ClientWritePacket(&packet.SpawnParticleEffect{
		Dimension:      0,
		EntityUniqueID: -1,
		Position:       mgl32.Vec3{float32(traceEnd[0]), float32(traceEnd[1]), float32(traceEnd[2])},
		ParticleName:   "hivehub:emote_confounded",
	})

	for _, sp := range s.players {
		if sp.gone {
			continue
		}
		pos := mgl64.Vec3{float64(sp.Position[0]), float64(sp.Position[1]) - 1.8, float64(sp.Position[2])}
		bb := playerBBox.Translate(pos)

		res, ok := trace.BBoxIntercept(bb, traceStart, traceEnd)
		if ok {
			fmt.Printf("res: %v\n", res.Position())
			interceptPos := res.Position()

			s.session.ClientWritePacket(&packet.SpawnParticleEffect{
				Dimension:      0,
				EntityUniqueID: -1,
				Position:       mgl32.Vec3{float32(interceptPos[0]), float32(interceptPos[1]), float32(interceptPos[2])},
				ParticleName:   "hivehub:emote_confounded",
			})

			id := uuid.MustParse(s.session.Client.IdentityData().Identity)

			s.session.ClientWritePacket(&packet.PlayerSkin{
				UUID: id,
				Skin: *sp.CurrentSkin.Skin,
			})

			s.session.Server.WritePacket(&packet.PlayerSkin{
				UUID: id,
				Skin: *sp.CurrentSkin.Skin,
			})
			break
		}
	}
}
