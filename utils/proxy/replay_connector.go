package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type ReplayConnector struct {
	reader *Pcap2Reader
	f      *os.File

	spawn  chan struct{}
	close  chan struct{}
	closed atomic.Bool

	clientData login.ClientData
	gameData   minecraft.GameData
	shieldID   atomic.Int32

	resourcePackHandler *rpHandler
}

func (r *ReplayConnector) ShieldID() int32 {
	return r.shieldID.Load()
}

func (r *ReplayConnector) handleLoginSequence(pk packet.Packet) (bool, error) {
	switch pk := pk.(type) {
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
			Items:                        pk.Items,
			PlayerMovementSettings:       pk.PlayerMovementSettings,
			ServerAuthoritativeInventory: pk.ServerAuthoritativeInventory,
			Experiments:                  pk.Experiments,
			ClientSideGeneration:         pk.ClientSideGeneration,
			ChatRestrictionLevel:         pk.ChatRestrictionLevel,
			DisablePlayerInteractions:    pk.DisablePlayerInteractions,
			UseBlockNetworkIDHashes:      pk.UseBlockNetworkIDHashes,
		})
		for _, item := range pk.Items {
			if item.Name == "minecraft:shield" {
				r.shieldID.Store(int32(item.RuntimeID))
			}
		}

	case *packet.ResourcePacksInfo:
		return false, r.resourcePackHandler.OnResourcePacksInfo(pk)
	case *packet.ResourcePackDataInfo:
		return false, r.resourcePackHandler.OnResourcePackDataInfo(pk)
	case *packet.ResourcePackChunkData:
		return false, r.resourcePackHandler.OnResourcePackChunkData(pk)
	case *packet.ResourcePackStack:
		return false, r.resourcePackHandler.OnResourcePackStack(pk)

	case *packet.SetLocalPlayerAsInitialised:
		if pk.EntityRuntimeID != r.gameData.EntityRuntimeID {
			return false, fmt.Errorf("entity runtime ID mismatch: entity runtime ID in StartGame and SetLocalPlayerAsInitialised packets should be equal")
		}
		close(r.spawn)
		return true, nil
	}
	return false, nil
}

func (r *ReplayConnector) ReadUntilLogin() error {
	gameStarted := false
	for !gameStarted {
		pk, _, timeReceived, err := r.reader.ReadPacket(false)
		if err != nil {
			return err
		}
		_ = timeReceived

		gameStarted, err = r.handleLoginSequence(pk)
		if err != nil {
			return err
		}
	}
	return nil
}

func CreateReplayConnector(ctx context.Context, filename string, packetFunc PacketFunc, onResourcePackInfo func(), OnFinishedPack func(*resource.Pack)) (r *ReplayConnector, err error) {
	r = &ReplayConnector{
		spawn: make(chan struct{}),
		close: make(chan struct{}),
	}
	r.resourcePackHandler = newRpHandler(ctx, nil)
	r.resourcePackHandler.OnResourcePacksInfoCB = onResourcePackInfo
	r.resourcePackHandler.OnFinishedPack = OnFinishedPack
	r.resourcePackHandler.SetServer(r)

	logrus.Infof("Reading replay %s", filename)
	r.f, err = os.Open(filename)
	if err != nil {
		return nil, err
	}
	r.reader, err = NewPcap2Reader(r.f, packetFunc, &r.shieldID)
	if err != nil {
		return nil, err
	}
	r.resourcePackHandler.cache = r.reader.ResourcePacks

	return r, nil
}

func (r *ReplayConnector) DisconnectOnInvalidPacket() bool {
	return false
}

func (r *ReplayConnector) DisconnectOnUnknownPacket() bool {
	return false
}

func (r *ReplayConnector) OnDisconnect() <-chan struct{} {
	return r.close
}

func (r *ReplayConnector) Expect(...uint32) {
}

func (r *ReplayConnector) SetLoggedIn() {}

func (r *ReplayConnector) Close() error {
	if r.closed.CompareAndSwap(false, true) {
		close(r.close)
	}
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
	case <-r.close:
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
	if r.closed.Load() {
		return nil, time.Time{}, net.ErrClosed
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

func (r *ReplayConnector) ResourcePacks() []*resource.Pack {
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
