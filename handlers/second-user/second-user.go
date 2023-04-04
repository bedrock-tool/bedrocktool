package seconduser

import (
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type secondaryUser struct {
	listener *fwdlistener
	server   *server.Server
	proxy    *utils.ProxyContext

	ispre118        bool
	hasCustomBlocks bool

	chunks    map[world.ChunkPos]*chunk.Chunk
	blockNBT  map[protocol.SubChunkPos][]map[string]any
	dimension world.Dimension
	entities  map[int64]*serverEntity

	mainPlayer *player.Player
}

func NewSecondUser() *utils.ProxyHandler {
	s := &secondaryUser{
		listener: &fwdlistener{
			Conn: make(chan session.Conn),
		},

		chunks:    make(map[world.ChunkPos]*chunk.Chunk),
		blockNBT:  make(map[protocol.SubChunkPos][]map[string]any),
		dimension: world.Overworld,
		entities:  make(map[int64]*serverEntity),
	}

	s.server = server.Config{
		Listeners: []func(conf server.Config) (server.Listener, error){
			func(conf server.Config) (server.Listener, error) {
				return s.listener, nil
			},
		},
		Log:           logrus.StandardLogger(),
		Name:          "Secondary",
		Generator:     func(dim world.Dimension) world.Generator { return &world.NopGenerator{} },
		WorldProvider: &provider{s: s},
		ReadOnlyWorld: true,
	}.New()

	go s.loop()

	return &utils.ProxyHandler{
		Name: "Secondary User",
		ProxyRef: func(pc *utils.ProxyContext) {
			s.proxy = pc
		},
		SecondaryClientCB: s.SecondaryClientCB,
		OnClientConnect: func(conn *minecraft.Conn) {
			id := conn.IdentityData()
			s.mainPlayer = player.New(id.DisplayName, skin.New(64, 64), mgl64.Vec3{0, 00})
			s.server.World().AddEntity(s.mainPlayer)
		},
		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time) (packet.Packet, error) {

			switch pk := pk.(type) {
			case *packet.LevelChunk:
				s.processLevelChunk(pk)
			case *packet.SubChunk:
				s.processSubChunk(pk)
			case *packet.ChangeDimension:
				s.processChangeDimension(pk)

			case *packet.MovePlayer:
				v := mgl64.Vec3{float64(pk.Position.X()), float64(pk.Position.Y()), float64(pk.Position.Z())}
				s.mainPlayer.Teleport(v)
			case *packet.PlayerAuthInput:
				v := mgl64.Vec3{float64(pk.Position.X()), float64(pk.Position.Y()), float64(pk.Position.Z())}
				s.mainPlayer.Teleport(v)

			case *packet.AddActor:
				e := newServerEntity(pk.EntityType)
				s.entities[pk.EntityUniqueID] = e
				s.server.World().AddEntity(e)
			}

			return pk, nil
		},
	}
}

func (s *secondaryUser) SecondaryClientCB(conn *minecraft.Conn) {
	s.listener.Conn <- conn
}

func (s *secondaryUser) loop() {
	s.server.Listen()
	for s.server.Accept(func(p *player.Player) {
		logrus.Infof("%s Joined", p.Name())
		p.Teleport(s.mainPlayer.Position())
	}) {
	}
}
