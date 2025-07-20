package handlers

import (
	"fmt"
	"math"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/bedrock-tool/bedrocktool/utils/skinconverter"
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
	log     *logrus.Entry
	session *proxy.Session

	PlayerNameFilter  *regexp.Regexp
	OnlyIfHasGeometry bool
	TextureOnly       bool

	fs            utils.WriterFS
	playersById   map[uuid.UUID]*skinPlayer
	playersByName map[string]uuid.UUID
}

func NewSkinSaver(skinCallback func(SkinAdd), PlayerNameFilter *regexp.Regexp, TextureOnly, Timestamped bool) func() *proxy.Handler {
	return func() *proxy.Handler {
		s := &SkinSaver{
			log: logrus.WithField("part", "SkinSaver"),

			PlayerNameFilter: PlayerNameFilter,
			TextureOnly:      TextureOnly,

			playersById:   make(map[uuid.UUID]*skinPlayer),
			playersByName: make(map[string]uuid.UUID),
		}
		return &proxy.Handler{
			Name: "Skin Saver",
			SessionStart: func(session *proxy.Session, hostname string) error {
				s.session = session
				basePath := utils.PathData("skins", hostname)
				if Timestamped {
					ts := time.Now().Format("2006-01-02_15-04-05")
					basePath = path.Join(basePath, ts)
				}
				s.fs = utils.OSWriter{Base: basePath}

				return nil
			},
			PacketCallback: func(session *proxy.Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
				for _, s := range s.ProcessPacket(pk) {
					if skinCallback != nil {
						skinCallback(s)
					}
				}
				return pk, nil
			},
		}
	}
}

type skinPlayer struct {
	UUID      uuid.UUID
	RuntimeID uint64
	Name      string
	AltName   string
	nameFinal bool
	Position  mgl32.Vec3
	SkinPack  *skinconverter.SkinPack

	seenSkins map[string]struct{}

	gone bool
}

// gets player by id, or creates it setting name to the name if one is given
func (s *SkinSaver) AddOrGetPlayer(id uuid.UUID, name string) *skinPlayer {
	player, ok := s.playersById[id]
	if !ok {
		player = &skinPlayer{UUID: id, seenSkins: make(map[string]struct{})}
		s.playersById[id] = player
	}
	if player.Name == "" && name != "" {
		player.Name = utils.CleanupName(name)
	}
	return player
}

func (s *SkinSaver) ensureFinalName(player *skinPlayer) {
	if !player.nameFinal {
		player.nameFinal = true
		var name = player.Name
		switch {
		case player.Name != "":
			name = player.Name
		case player.AltName != "":
			name = player.AltName
		default:
			name = player.UUID.String()
		}

		for i := 0; ; i++ {
			var nameIt = name
			if i > 0 {
				nameIt = fmt.Sprintf("%s_%d", name, i)
			}
			if _, ok := s.playersByName[nameIt]; !ok {
				player.Name = nameIt
				break
			}
		}
		s.playersByName[player.Name] = player.UUID
	}
}

func (s *SkinSaver) AddSkin(player *skinPlayer, newSkin *protocol.Skin) (*skinconverter.Skin, bool) {
	s.ensureFinalName(player)

	if s.PlayerNameFilter != nil {
		if !s.PlayerNameFilter.MatchString(player.Name) {
			return nil, false
		}
	}

	skin := &skinconverter.Skin{Skin: newSkin}
	if !skin.HaveGeometry() && s.OnlyIfHasGeometry {
		return nil, false
	}

	// check for duplicate
	sh := skin.Hash()
	_, ok := player.seenSkins[sh]
	if ok {
		return skin, false
	}
	player.seenSkins[sh] = struct{}{}

	if s.TextureOnly {
		skinName := player.Name
		if len(player.seenSkins) > 1 {
			skinName += "-" + strconv.Itoa(len(player.seenSkins))
		}

		err := skinconverter.WriteSkinTexture(s.fs, skinName, skin)
		if err != nil {
			s.log.WithError(err).Error("failed to write skin texture")
			return skin, false
		}

		return skin, true
	} else {
		if player.SkinPack == nil {
			player.SkinPack = skinconverter.NewSkinPack(player.Name)
			creating := fmt.Sprintf("Creating Skinpack for %s", player.Name)
			s.log.Info(creating)
		}
		player.SkinPack.AddSkin(skin)
		addedStr := fmt.Sprintf("Added a skin %s", player.Name)
		s.session.SendPopup(addedStr)
		s.log.Info(addedStr)
		pfs := utils.SubFS(s.fs, player.Name)
		err := player.SkinPack.Save(pfs)
		if err != nil {
			s.log.Error(err)
		}
		return skin, true
	}
}

type SkinAdd struct {
	PlayerName string
	Skin       *protocol.Skin
}

func (s *SkinSaver) ProcessPacket(pk packet.Packet) (out []SkinAdd) {
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		var player *skinPlayer
		for _, sp := range s.playersById {
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
		for _, player := range s.playersById {
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
			player := s.AddOrGetPlayer(playerEntry.UUID, playerEntry.Username)
			skin, wasAdded := s.AddSkin(player, &playerEntry.Skin)
			if wasAdded {
				out = append(out, SkinAdd{
					PlayerName: player.Name,
					Skin:       skin.Skin,
				})
			}
		}
	case *packet.PlayerSkin:
		player := s.AddOrGetPlayer(pk.UUID, "")
		skin, wasAdded := s.AddSkin(player, &pk.Skin)
		if wasAdded {
			out = append(out, SkinAdd{
				PlayerName: player.Name,
				Skin:       skin.Skin,
			})
		}
	case *packet.AddPlayer:
		player := s.AddOrGetPlayer(pk.UUID, "")
		player.AltName = pk.Username
		player.RuntimeID = pk.EntityRuntimeID
	case *packet.Animate:
		if pk.EntityRuntimeID == s.session.Player.RuntimeID && pk.ActionType == packet.AnimateActionSwingArm {
			s.stealSkin()
		}
	case *packet.ChangeDimension:
		for _, sp := range s.playersById {
			sp.gone = true
		}
	case *packet.RemoveActor:
		for _, sp := range s.playersById {
			if sp.RuntimeID == uint64(pk.EntityUniqueID) {
				sp.gone = true
				break
			}
		}
	}
	return out
}

var playerBBox = cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3)

var rtTest = false

func (s *SkinSaver) stealSkin() {
	if !rtTest {
		return
	}
	s.log.Debugf("%d", len(s.playersById))

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

	for _, sp := range s.playersById {
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
			playerSkin := sp.SkinPack.Latest().Skin
			if playerSkin == nil {
				logrus.Warnf("%s has no skin", sp.Name)
				continue
			}

			s.session.ClientWritePacket(&packet.PlayerSkin{
				UUID: id,
				Skin: *playerSkin,
			})

			s.session.Server.WritePacket(&packet.PlayerSkin{
				UUID: id,
				Skin: *playerSkin,
			})
			break
		}
	}
}
