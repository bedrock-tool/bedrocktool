package worlds

import (
	"context"
	"fmt"
	"image/png"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type playerPos struct {
	Position mgl32.Vec3
	Pitch    float32
	Yaw      float32
	HeadYaw  float32
}

// the state used for drawing and saving

type WorldSettings struct {
	// settings
	VoidGen         bool
	WithPacks       bool
	SaveImage       bool
	SaveEntities    bool
	SaveInventories bool
	BlockUpdates    bool
}

type serverState struct {
	ispre118     bool
	worldCounter int

	playerInventory []protocol.ItemInstance
	PlayerPos       playerPos
	packs           []utils.Pack

	Name string
}

type worldsHandler struct {
	ctx   context.Context
	wg    sync.WaitGroup
	proxy *proxy.Context
	mapUI *MapUI
	ui    ui.UI
	bp    *behaviourpack.BehaviourPack

	worldState   *worldState
	serverState  serverState
	settings     WorldSettings
	customBlocks []protocol.BlockEntry
}

func NewWorldsHandler(ctx context.Context, ui ui.UI, settings WorldSettings) *proxy.Handler {
	w := &worldsHandler{
		ctx: ctx,
		ui:  ui,

		serverState: serverState{
			ispre118:     false,
			worldCounter: 0,
			PlayerPos:    playerPos{},
		},

		settings: settings,
	}
	w.mapUI = NewMapUI(w)
	w.Reset()

	return &proxy.Handler{
		Name: "Worlds",
		ProxyRef: func(pc *proxy.Context) {
			w.proxy = pc

			w.proxy.AddCommand(func(cmdline []string) bool {
				return w.setWorldName(strings.Join(cmdline, " "), false)
			}, protocol.Command{
				Name:        "setname",
				Description: locale.Loc("setname_desc", nil),
				Overloads: []protocol.CommandOverload{{
					Parameters: []protocol.CommandParameter{{
						Name:     "name",
						Type:     protocol.CommandArgTypeString,
						Optional: false,
					}},
				}},
			})

			w.proxy.AddCommand(func(cmdline []string) bool {
				return w.setVoidGen(!w.worldState.VoidGen, false)
			}, protocol.Command{
				Name:        "void",
				Description: locale.Loc("void_desc", nil),
			})
		},
		AddressAndName: func(address, hostname string) error {
			w.bp = behaviourpack.New(hostname)
			w.serverState.Name = hostname
			return nil
		},
		OnClientConnect: func(conn minecraft.IConn) {
			w.ui.Message(messages.SetUIState(messages.UIStateConnecting))
		},

		ToClientGameDataModifier: func(gd *minecraft.GameData) {
			gd.ClientSideGeneration = false
		},

		ConnectCB: w.OnConnect,
		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			switch pk := pk.(type) {
			case *packet.ChunkRadiusUpdated:
				pk.ChunkRadius = 80
			case *packet.SetTime:
				w.worldState.timeSync = time.Now()
				w.worldState.time = int(pk.Time)
			case *packet.StartGame:
				w.worldState.timeSync = time.Now()
				w.worldState.time = int(pk.Time)
				w.worldState.dimension, _ = world.DimensionByID(int(pk.Dimension))
				world.InsertCustomItems(pk.Items)
				for _, ie := range pk.Items {
					w.bp.AddItem(ie)
				}
			case *packet.ItemComponent:
				w.bp.ApplyComponentEntries(pk.Items)
			}

			forward := true
			pk = w.handleItemPackets(pk, &forward)
			pk = w.handleMapPackets(pk, &forward, toServer)
			pk = w.handleChunkPackets(pk)
			pk = w.handleEntityPackets(pk)

			if !forward {
				return nil, nil
			}
			return pk, nil
		},
		OnEnd: func() {
			w.SaveAndReset()
			w.wg.Wait()
		},
	}
}

func (w *worldsHandler) OnConnect(err error) bool {
	w.ui.Message(messages.SetWorldName{
		WorldName: w.worldState.Name,
	})
	w.ui.Message(messages.SetUIState(messages.UIStateMain))
	if err != nil {
		return false
	}

	w.proxy.ClientWritePacket(&packet.ChunkRadiusUpdated{
		ChunkRadius: 80,
	})

	gd := w.proxy.Server.GameData()
	mapItemID, _ := world.ItemRidByName("minecraft:filled_map")
	MapItemPacket.Content[0].Stack.ItemType.NetworkID = mapItemID
	if gd.ServerAuthoritativeInventory {
		MapItemPacket.Content[0].StackNetworkID = 0xffff + rand.Int31n(0xfff)
	}

	w.serverState.packs = utils.GetPacks(w.proxy.Server)

	if len(gd.CustomBlocks) > 0 {
		logrus.Info(locale.Loc("using_customblocks", nil))
		for _, be := range gd.CustomBlocks {
			w.bp.AddBlock(be)
		}
		// telling the chunk code what custom blocks there are so it can generate offsets
		world.InsertCustomBlocks(gd.CustomBlocks)
		w.customBlocks = gd.CustomBlocks
	}

	{ // check game version
		gv := strings.Split(gd.BaseGameVersion, ".")
		var err error
		if len(gv) > 1 {
			var ver int
			ver, err = strconv.Atoi(gv[1])
			w.serverState.ispre118 = ver < 18
		}
		if err != nil || len(gv) <= 1 {
			logrus.Info(locale.Loc("guessing_version", nil))
		}

		dimensionID := gd.Dimension
		if w.serverState.ispre118 {
			logrus.Info(locale.Loc("using_under_118", nil))
			if dimensionID == 0 {
				dimensionID += 10
			}
		}
		w.worldState.dimension, _ = world.DimensionByID(int(dimensionID))
	}

	w.proxy.SendMessage(locale.Loc("use_setname", nil))
	w.mapUI.Start()
	return true
}

func (w *worldsHandler) setVoidGen(val bool, fromUI bool) bool {
	w.worldState.VoidGen = val
	var s = locale.Loc("void_generator_false", nil)
	if w.worldState.VoidGen {
		s = locale.Loc("void_generator_true", nil)
	}
	w.proxy.SendMessage(s)

	if !fromUI {
		w.ui.Message(messages.SetVoidGen{
			Value: w.worldState.VoidGen,
		})
	}

	return true
}

func (w *worldsHandler) setWorldName(val string, fromUI bool) bool {
	w.worldState.Name = val
	w.proxy.SendMessage(locale.Loc("worldname_set", locale.Strmap{"Name": w.worldState.Name}))

	if !fromUI {
		w.ui.Message(messages.SetWorldName{
			WorldName: w.worldState.Name,
		})
	}

	return true
}

func (w *worldsHandler) defaultName() string {
	worldName := "world"
	if w.serverState.worldCounter > 0 {
		worldName = fmt.Sprintf("world-%d", w.serverState.worldCounter)
	}
	return worldName
}

func (w *worldsHandler) Reset() {
	var dim world.Dimension
	if w.worldState != nil {
		dim = w.worldState.dimension
	}
	w.worldState = newWorldState(w.defaultName(), dim)
	w.worldState.VoidGen = w.settings.VoidGen
	w.mapUI.Reset()
}

func (w *worldsHandler) SaveAndReset() {
	w.worldState.cullChunks()
	if len(w.worldState.chunks) == 0 {
		w.Reset()
		return
	}

	playerPos := w.serverState.PlayerPos.Position
	spawnPos := cube.Pos{int(playerPos.X()), int(playerPos.Y()), int(playerPos.Z())}

	folder := fmt.Sprintf("worlds/%s/%s", w.serverState.Name, w.worldState.Name)
	filename := folder + ".mcworld"

	if w.settings.SaveImage {
		f, _ := os.Create(folder + ".png")
		png.Encode(f, w.mapUI.ToImage())
		f.Close()
	}

	logrus.Infof(locale.Loc("saving_world", locale.Strmap{"Name": w.worldState.Name, "Count": len(w.worldState.chunks)}))
	w.ui.Message(messages.SavingWorld{
		World: &messages.SavedWorld{
			Name:   w.worldState.Name,
			Path:   filename,
			Chunks: len(w.worldState.chunks),
		},
	})

	w.wg.Add(1)
	worldState := w.worldState
	go func() {
		defer w.wg.Done()
		err := worldState.Save(folder, w.playerData(), spawnPos, w.proxy.Server.GameData(), w.bp)
		if err != nil {
			logrus.Error(err)
			return
		}
		w.AddPacks(folder)

		// zip it
		err = utils.ZipFolder(filename, folder)
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Info(locale.Loc("saved", locale.Strmap{"Name": filename}))
		//os.RemoveAll(folder)
		w.ui.Message(messages.SetUIState(messages.UIStateMain))
	}()

	w.serverState.worldCounter += 1
	w.Reset()
}
