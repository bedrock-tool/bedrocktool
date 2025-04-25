package pcap2

import (
	"context"
	"errors"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/proxy/resourcepacks"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type ReplayConnector struct {
	reader *Pcap2Reader
	f      *os.File

	ctx       context.Context
	cancelCtx context.CancelCauseFunc

	spawn chan struct{}

	expectedIDs     atomic.Value
	deferredPackets []packet.Packet

	clientData login.ClientData
	gameData   minecraft.GameData

	resourcePackHandler *resourcepacks.ResourcePackHandler
}

type PacketFunc func(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time)

func CreateReplayConnector(ctx context.Context, filename string, packetFunc PacketFunc, resourcePackHandler *resourcepacks.ResourcePackHandler) (r *ReplayConnector, err error) {
	r = &ReplayConnector{
		spawn:               make(chan struct{}),
		resourcePackHandler: resourcePackHandler,
	}
	if r.resourcePackHandler != nil {
		r.resourcePackHandler.SetServer(r)
	}
	r.ctx, r.cancelCtx = context.WithCancelCause(ctx)

	logrus.Infof("Reading replay %s", filename)
	r.f, err = os.Open(filename)
	if err != nil {
		return nil, err
	}
	r.reader, err = NewPcap2Reader(r.f)
	if err != nil {
		return nil, err
	}
	r.reader.PacketFunc = packetFunc
	if r.resourcePackHandler != nil {
		r.resourcePackHandler.SetCache(r.reader.ResourcePacks)
	}

	return r, nil
}

func (r *ReplayConnector) ShieldID() int32 {
	return r.reader.shieldID.Load()
}

func (r *ReplayConnector) handleLoginSequence(pk packet.Packet) (bool, bool, error) {
	switch pk := pk.(type) {
	case *packet.ResourcePacksInfo:
		if r.resourcePackHandler != nil {
			return false, true, r.resourcePackHandler.OnResourcePacksInfo(pk)
		}
		r.Expect(packet.IDResourcePackClientResponse)
	case *packet.ResourcePackDataInfo:
		if r.resourcePackHandler != nil {
			return false, true, r.resourcePackHandler.OnResourcePackDataInfo(pk)
		}
	case *packet.ResourcePackChunkData:
		if r.resourcePackHandler != nil {
			return false, true, r.resourcePackHandler.OnResourcePackChunkData(pk)
		}
	case *packet.ResourcePackStack:
		if r.resourcePackHandler != nil {
			return false, true, r.resourcePackHandler.OnResourcePackStack(pk)
		}
	case *packet.StartGame:
		r.SetGameData(minecraft.GameData{
			WorldName:                    pk.WorldName,
			WorldSeed:                    pk.WorldSeed,
			Difficulty:                   pk.Difficulty,
			EntityUniqueID:               pk.EntityUniqueID,
			EntityRuntimeID:              pk.EntityRuntimeID,
			PlayerGameMode:               pk.PlayerGameMode,
			PersonaDisabled:              pk.PersonaDisabled,
			CustomSkinsDisabled:          pk.CustomSkinsDisabled,
			BaseGameVersion:              pk.BaseGameVersion,
			PlayerPosition:               pk.PlayerPosition,
			Pitch:                        pk.Pitch,
			Yaw:                          pk.Yaw,
			Dimension:                    pk.Dimension,
			WorldSpawn:                   pk.WorldSpawn,
			EditorWorldType:              pk.EditorWorldType,
			WorldGameMode:                pk.WorldGameMode,
			GameRules:                    pk.GameRules,
			Time:                         pk.Time,
			ServerBlockStateChecksum:     pk.ServerBlockStateChecksum,
			CustomBlocks:                 pk.Blocks,
			PlayerMovementSettings:       pk.PlayerMovementSettings,
			ServerAuthoritativeInventory: pk.ServerAuthoritativeInventory,
			Experiments:                  pk.Experiments,
			ClientSideGeneration:         pk.ClientSideGeneration,
			ChatRestrictionLevel:         pk.ChatRestrictionLevel,
			DisablePlayerInteractions:    pk.DisablePlayerInteractions,
			UseBlockNetworkIDHashes:      pk.UseBlockNetworkIDHashes,
		})
	case *packet.ItemRegistry:
		r.gameData.Items = pk.Items
	case *packet.SetLocalPlayerAsInitialised:
		if pk.EntityRuntimeID != r.gameData.EntityRuntimeID {
			//return false, true, fmt.Errorf("entity runtime ID mismatch: entity runtime ID in StartGame and SetLocalPlayerAsInitialised packets should be equal")
		}
		close(r.spawn)
		return true, true, nil
	}
	return false, false, nil
}

func (r *ReplayConnector) ReadUntilLogin() error {
	gameStarted := false
	for !gameStarted {
		pk, _, timeReceived, err := r.reader.ReadPacket(false)
		if err != nil {
			return err
		}
		_ = timeReceived

		var handled bool
		gameStarted, handled, err = r.handleLoginSequence(pk)
		if err != nil {
			return err
		}
		if !handled {
			r.deferredPackets = append(r.deferredPackets, pk)
		}
	}
	return nil
}

func (r *ReplayConnector) Context() context.Context {
	return r.ctx
}

func (r *ReplayConnector) Expect(ids ...uint32) {
	r.expectedIDs.Store(ids)
}

func (r *ReplayConnector) SetLoggedIn() {}

func (r *ReplayConnector) Close() error {
	r.cancelCtx(net.ErrClosed)
	return nil
}

func (r *ReplayConnector) Authenticated() bool {
	return true
}

func (r *ReplayConnector) ChunkRadius() int {
	return 80
}

func (r *ReplayConnector) ClientCacheEnabled() bool {
	return false
}

func (r *ReplayConnector) ClientData() login.ClientData {
	return r.clientData
}

func (r *ReplayConnector) DoSpawn() error {
	return r.DoSpawnContext(context.Background())
}

func (r *ReplayConnector) DoSpawnContext(ctx context.Context) error {
	select {
	case <-r.ctx.Done():
		ctx = r.ctx
	case <-ctx.Done():
	case <-r.spawn:
		// Conn was spawned successfully.
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.New("do spawn")
}

func (r *ReplayConnector) DoSpawnTimeout(timeout time.Duration) error {
	c, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.DoSpawnContext(c)
}

func (r *ReplayConnector) Flush() error {
	return nil
}

func (r *ReplayConnector) GameData() minecraft.GameData {
	return r.gameData
}

func (r *ReplayConnector) IdentityData() login.IdentityData {
	return login.IdentityData{}
}

func (r *ReplayConnector) Latency() time.Duration {
	return 0
}

func (r *ReplayConnector) Read(b []byte) (n int, err error) {
	return 0, errors.New("not Implemented")
}

func (r *ReplayConnector) ReadPacketWithTime() (pk packet.Packet, receivedAt time.Time, err error) {
	if r.ctx.Err() != nil {
		return nil, time.Time{}, net.ErrClosed
	}

	if len(r.deferredPackets) > 0 {
		pk = r.deferredPackets[0]
		r.deferredPackets = r.deferredPackets[1:]
		receivedAt = time.Now() // fixme
		return
	}

	pk, toServer, receivedTime, err := r.reader.ReadPacket(false)
	if err != nil {
		return nil, time.Time{}, err
	}
	_ = toServer // proxy puts both from client and from server packets into the same callback so doesnt matter
	return pk, receivedTime, nil
}

func (r *ReplayConnector) ReadPacket() (pk packet.Packet, err error) {
	pk, _, err = r.ReadPacketWithTime()
	return pk, err
}

func (r *ReplayConnector) Write(b []byte) (n int, err error) {
	panic("unimplemented")
}

func (r *ReplayConnector) WritePacket(pk packet.Packet) error {
	return nil
}

var replayLocalAddr = &net.UDPAddr{IP: net.IPv4(1, 1, 1, 1)}
var replayRemoteAddr = &net.UDPAddr{IP: net.IPv4(2, 2, 2, 2)}

func (r *ReplayConnector) LocalAddr() net.Addr  { return replayLocalAddr }
func (r *ReplayConnector) RemoteAddr() net.Addr { return replayRemoteAddr }

func (r *ReplayConnector) ResourcePacks() []resource.Pack {
	return r.resourcePackHandler.ResourcePacks()
}

func (r *ReplayConnector) SetGameData(data minecraft.GameData) {
	r.gameData = data
}

func (r *ReplayConnector) StartGame(data minecraft.GameData) error {
	return nil
}

func (r *ReplayConnector) StartGameContext(ctx context.Context, data minecraft.GameData) error {
	return nil
}

func (r *ReplayConnector) StartGameTimeout(data minecraft.GameData, timeout time.Duration) error {
	return nil
}

func (r *ReplayConnector) SetDeadline(t time.Time) error {
	return nil
}

func (r *ReplayConnector) SetReadDeadline(t time.Time) error {
	return nil
}

func (r *ReplayConnector) SetWriteDeadline(time.Time) error {
	return nil
}
