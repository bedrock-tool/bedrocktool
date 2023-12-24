package proxy

import (
	"archive/zip"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type replayConnector struct {
	f       *os.File
	packetF io.Reader
	ver     uint32

	packets chan packet.Packet
	err     error

	spawn  chan struct{}
	close  chan struct{}
	closed atomic.Bool
	once   sync.Once

	pool       packet.Pool
	proto      minecraft.Protocol
	clientData login.ClientData

	gameData minecraft.GameData

	packetFunc PacketFunc

	resourcePackHandler *rpHandler
}

func (r *replayConnector) readPacket() (payload []byte, toServer bool, err error) {
	var magic uint32 = 0
	var packetLength uint32 = 0
	timeReceived := time.Now()

	err = binary.Read(r.packetF, binary.LittleEndian, &magic)
	if err != nil {
		if errors.Is(err, io.EOF) {
			logrus.Info("Reached End")
			return nil, false, nil
		}
		return nil, false, err
	}
	if magic != 0xAAAAAAAA {
		return nil, toServer, fmt.Errorf("wrong Magic")
	}
	binary.Read(r.packetF, binary.LittleEndian, &packetLength)
	binary.Read(r.packetF, binary.LittleEndian, &toServer)
	if r.ver >= 2 {
		var timeMs int64
		binary.Read(r.packetF, binary.LittleEndian, &timeMs)
		timeReceived = time.UnixMilli(timeMs)
	}

	payload = make([]byte, packetLength)
	n, err := io.ReadFull(r.packetF, payload)
	if err != nil {
		return nil, toServer, err
	}
	if n != int(packetLength) {
		return nil, toServer, fmt.Errorf("truncated")
	}

	var magic2 uint32
	binary.Read(r.packetF, binary.LittleEndian, &magic2)
	if magic2 != 0xBBBBBBBB {
		return nil, toServer, fmt.Errorf("wrong Magic2")
	}

	_ = timeReceived
	return payload, toServer, nil
}

func (r *replayConnector) handleLoginSequence(pk packet.Packet) (bool, error) {
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

func (r *replayConnector) loop() {
	gameStarted := false
	defer r.Close()
	for {
		payload, toServer, err := r.readPacket()
		if err != nil {
			r.err = err
			r.Close()
			return
		}
		if payload == nil {
			return
		}
		var src, dst = r.RemoteAddr(), r.LocalAddr()
		if toServer {
			src, dst = r.LocalAddr(), r.RemoteAddr()
		}

		pkData, err := minecraft.ParseData(payload, r, src, dst)
		if err != nil {
			r.err = err
			r.Close()
			return
		}
		pks, err := pkData.Decode(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		for _, pk := range pks {
			if !gameStarted {
				gameStarted, err = r.handleLoginSequence(pk)
				if err != nil {
					r.err = err
					r.Close()
					return
				}
			} else {
				if r.closed.Load() {
					return
				}
				r.packets <- pk
			}
		}
	}
}

func CreateReplayConnector(ctx context.Context, filename string, packetFunc PacketFunc, onResourcePackInfo func(), OnFinishedPack func(*resource.Pack)) (r *replayConnector, err error) {
	pool := minecraft.DefaultProtocol.Packets(true)
	maps.Copy(pool, minecraft.DefaultProtocol.Packets(false))
	r = &replayConnector{
		pool:       pool,
		proto:      minecraft.DefaultProtocol,
		packetFunc: packetFunc,
		spawn:      make(chan struct{}),
		close:      make(chan struct{}),
		packets:    make(chan packet.Packet),
	}
	r.resourcePackHandler = newRpHandler(ctx, nil)
	r.resourcePackHandler.OnResourcePacksInfoCB = onResourcePackInfo
	r.resourcePackHandler.OnFinishedPack = OnFinishedPack
	r.resourcePackHandler.SetServer(r)
	cache := &replayCache{}
	r.resourcePackHandler.cache = cache

	logrus.Infof("Reading replay %s", filename)

	r.f, err = os.Open(filename)
	if err != nil {
		return nil, err
	}

	var head = make([]byte, 16)
	r.f.Read(head)
	magic := string(head[0:4])

	var z *zip.Reader
	var zipSize uint64
	if magic != "BTCP" {
		logrus.Warn("capture is old format")
		stat, _ := r.f.Stat()
		z, err = zip.NewReader(r.f, stat.Size())
		if err != nil {
			return nil, err
		}
	} else {
		r.ver = binary.LittleEndian.Uint32(head[4:8])
		zipSize := binary.LittleEndian.Uint64(head[8:16])
		z, err = zip.NewReader(io.NewSectionReader(r.f, 16, int64(zipSize)), int64(zipSize))
		if err != nil {
			return nil, err
		}
	}

	// read all packs
	err = cache.ReadFrom(z)
	if err != nil {
		return nil, err
	}

	if r.ver < 4 {
		// open packets bin
		r.packetF, err = z.Open("packets.bin")
		if err != nil {
			return nil, err
		}
	} else {
		r.f.Seek(int64(zipSize+16), 0)
		fr := flate.NewReader(r.f)
		if err != nil {
			return nil, err
		}
		r.packetF = fr
	}

	go r.loop()
	return r, nil
}

func (r *replayConnector) DisconnectOnInvalidPacket() bool {
	return false
}

func (r *replayConnector) DisconnectOnUnknownPacket() bool {
	return false
}

func (r *replayConnector) OnDisconnect() <-chan struct{} {
	return r.close
}

func (r *replayConnector) Expect(...uint32) {
}

func (r *replayConnector) SetLoggedIn() {

}

func (r *replayConnector) Close() error {
	r.once.Do(func() {
		r.closed.Store(true)
		close(r.close)
		select {
		case <-r.packets:
		default:
		}
		close(r.packets)
	})
	return nil
}

func (r *replayConnector) Authenticated() bool {
	return true
}

func (r *replayConnector) ChunkRadius() int {
	return 80
}

func (r *replayConnector) ClientCacheEnabled() bool {
	return false
}

func (r *replayConnector) ClientData() login.ClientData {
	return r.clientData
}

func (r *replayConnector) DoSpawn() error {
	return r.DoSpawnContext(context.Background())
}

func (r *replayConnector) DoSpawnContext(ctx context.Context) error {
	select {
	case <-r.close:
	case <-ctx.Done():
	case <-r.spawn:
		// Conn was spawned successfully.
		return nil
	}
	if r.err != nil {
		return r.err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.New("do spawn")
}

func (r *replayConnector) DoSpawnTimeout(timeout time.Duration) error {
	c, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.DoSpawnContext(c)
}

func (r *replayConnector) Flush() error {
	return nil
}

func (r *replayConnector) GameData() minecraft.GameData {
	return r.gameData
}

func (r *replayConnector) IdentityData() login.IdentityData {
	return login.IdentityData{}
}

func (r *replayConnector) Latency() time.Duration {
	return 0
}

func (r *replayConnector) LocalAddr() net.Addr {
	return &net.UDPAddr{
		IP: net.IPv4(1, 1, 1, 1),
	}
}

func (r *replayConnector) Read(b []byte) (n int, err error) {
	return 0, errors.New("not Implemented")
}

func (r *replayConnector) ReadPacket() (pk packet.Packet, err error) {
	select {
	case <-r.close:
		if r.err != nil {
			return nil, r.err
		}
		return nil, net.ErrClosed
	case p, ok := <-r.packets:
		if !ok {
			err = net.ErrClosed
			if r.err != nil {
				err = r.err
			}
		}
		return p, err
	}
}

func (r *replayConnector) Write(b []byte) (n int, err error) {
	return 0, errors.New("not Implemented")
}

func (r *replayConnector) WritePacket(pk packet.Packet) error {
	return nil
}

func (r *replayConnector) RemoteAddr() net.Addr {
	return &net.UDPAddr{
		IP: net.IPv4(2, 2, 2, 2),
	}
}

func (r *replayConnector) ResourcePacks() []*resource.Pack {
	return r.resourcePackHandler.ResourcePacks()
}

func (r *replayConnector) SetGameData(data minecraft.GameData) {
	r.gameData = data
}

func (r *replayConnector) StartGame(data minecraft.GameData) error {
	return r.StartGameContext(context.Background(), data)
}

func (r *replayConnector) StartGameContext(ctx context.Context, data minecraft.GameData) error {
	return nil
}

func (r *replayConnector) StartGameTimeout(data minecraft.GameData, timeout time.Duration) error {
	c, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.StartGameContext(c, data)
}

func (r *replayConnector) SetDeadline(t time.Time) error {
	return nil
}

func (r *replayConnector) SetReadDeadline(t time.Time) error {
	return nil
}

func (r *replayConnector) SetWriteDeadline(time.Time) error {
	return nil
}

func (r *replayConnector) Pool() packet.Pool {
	return r.pool
}

func (r *replayConnector) ShieldID() int32 {
	return 0
}

func (r *replayConnector) Proto() minecraft.Protocol {
	return r.proto
}

func (r *replayConnector) PacketFunc(header packet.Header, payload []byte, src, dst net.Addr) {
	if r.packetFunc != nil {
		r.packetFunc(header, payload, src, dst)
	}
}
