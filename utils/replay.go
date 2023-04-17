package utils

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type replayHeader struct {
	Version int32
}

var replayMagic = []byte("BTCP")

const (
	currentReplayVersion = 2
)

func WriteReplayHeader(f io.Writer) {
	f.Write(replayMagic)
	header := replayHeader{
		Version: currentReplayVersion,
	}
	binary.Write(f, binary.LittleEndian, &header)
}

type replayConnector struct {
	f         *os.File
	totalSize int64
	ver       int

	packets chan packet.Packet
	spawn   chan struct{}
	close   chan struct{}
	once    sync.Once

	pool       packet.Pool
	proto      minecraft.Protocol
	clientData login.ClientData

	gameData minecraft.GameData

	packetFunc PacketFunc

	downloadingPacks map[string]*downloadingPack
	resourcePacks    []*resource.Pack
}

// downloadingPack is a resource pack that is being downloaded by a client connection.
type downloadingPack struct {
	buf           *bytes.Buffer
	chunkSize     uint32
	size          uint64
	expectedIndex uint32
	newFrag       chan []byte
	contentKey    string
}

func (r *replayConnector) readHeader() error {
	r.ver = 1

	magic := make([]byte, 4)
	io.ReadAtLeast(r.f, magic, 4)
	if bytes.Equal(magic, replayMagic) {
		var header replayHeader
		if err := binary.Read(r.f, binary.LittleEndian, &header); err != nil {
			return err
		}
		r.ver = int(header.Version)
	} else {
		logrus.Info("Version 1 capture assumed.")
		r.f.Seek(-4, io.SeekCurrent)
	}
	return nil
}

func (r *replayConnector) readPacket() (payload []byte, toServer bool, err error) {
	var magic uint32 = 0
	var packetLength uint32 = 0
	timeReceived := time.Now()

	offset, _ := r.f.Seek(0, io.SeekCurrent)
	if offset == r.totalSize {
		logrus.Info("Reached End")
		return nil, toServer, nil
	}

	binary.Read(r.f, binary.LittleEndian, &magic)
	if magic != 0xAAAAAAAA {
		return nil, toServer, fmt.Errorf("wrong Magic")
	}
	binary.Read(r.f, binary.LittleEndian, &packetLength)
	binary.Read(r.f, binary.LittleEndian, &toServer)
	if r.ver >= 2 {
		var timeMs int64
		binary.Read(r.f, binary.LittleEndian, &timeMs)
		timeReceived = time.UnixMilli(timeMs)
	}

	payload = make([]byte, packetLength)
	n, err := r.f.Read(payload)
	if err != nil {
		return nil, toServer, err
	}
	if n != int(packetLength) {
		return nil, toServer, fmt.Errorf("truncated")
	}

	var magic2 uint32
	binary.Read(r.f, binary.LittleEndian, &magic2)
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
			EditorWorld:                  pk.EditorWorld,
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
		})

	case *packet.ResourcePacksInfo:
		for _, pack := range pk.TexturePacks {
			r.downloadingPacks[pack.UUID] = &downloadingPack{
				size:       pack.Size,
				buf:        bytes.NewBuffer(make([]byte, 0, pack.Size)),
				newFrag:    make(chan []byte),
				contentKey: pack.ContentKey,
			}
		}

	case *packet.ResourcePackDataInfo:
		pack, ok := r.downloadingPacks[pk.UUID]
		if !ok {
			// We either already downloaded the pack or we got sent an invalid UUID, that did not match any pack
			// sent in the ResourcePacksInfo packet.
			return false, fmt.Errorf("unknown pack to download with UUID %v", pk.UUID)
		}
		if pack.size != pk.Size {
			// Size mismatch: The ResourcePacksInfo packet had a size for the pack that did not match with the
			// size sent here.
			logrus.Printf("pack %v had a different size in the ResourcePacksInfo packet than the ResourcePackDataInfo packet\n", pk.UUID)
			pack.size = pk.Size
		}
		pack.chunkSize = pk.DataChunkSize

		chunkCount := uint32(pk.Size / uint64(pk.DataChunkSize))
		if pk.Size%uint64(pk.DataChunkSize) != 0 {
			chunkCount++
		}

		go func() {
			for i := uint32(0); i < chunkCount; i++ {
				select {
				case <-r.close:
					return
				case frag := <-pack.newFrag:
					// Write the fragment to the full buffer of the downloading resource pack.
					_, _ = pack.buf.Write(frag)
				}
			}

			if pack.buf.Len() != int(pack.size) {
				logrus.Printf("incorrect resource pack size: expected %v, but got %v\n", pack.size, pack.buf.Len())
				return
			}

			// First parse the resource pack from the total byte buffer we obtained.
			newPack, err := resource.FromBytes(pack.buf.Bytes())
			if err != nil {
				logrus.Printf("invalid full resource pack data for UUID %v: %v\n", pk.UUID, err)
				return
			}

			r.resourcePacks = append(r.resourcePacks, newPack.WithContentKey(pack.contentKey))
		}()

	case *packet.ResourcePackChunkData:
		pack, ok := r.downloadingPacks[pk.UUID]
		if !ok {
			// We haven't received a ResourcePackDataInfo packet from the server, so we can't use this data to
			// download a resource pack.
			return false, fmt.Errorf("resource pack chunk data for resource pack that was not being downloaded")
		}
		lastData := pack.buf.Len()+int(pack.chunkSize) >= int(pack.size)
		if !lastData && uint32(len(pk.Data)) != pack.chunkSize {
			// The chunk data didn't have the full size and wasn't the last data to be sent for the resource pack,
			// meaning we got too little data.
			return false, fmt.Errorf("resource pack chunk data had a length of %v, but expected %v", len(pk.Data), pack.chunkSize)
		}
		if pk.ChunkIndex != pack.expectedIndex {
			return false, fmt.Errorf("resource pack chunk data had chunk index %v, but expected %v", pk.ChunkIndex, pack.expectedIndex)
		}
		pack.expectedIndex++
		pack.newFrag <- pk.Data

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
			logrus.Error(err)
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
			logrus.Error(err)
			return
		}
		pks, err := pkData.Decode(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		for _, pk := range pks {
			if !gameStarted {
				gameStarted, _ = r.handleLoginSequence(pk)
			} else {
				r.packets <- pk
			}
		}
	}
}

func createReplayConnector(filename string, packetFunc PacketFunc) (r *replayConnector, err error) {
	r = &replayConnector{
		pool:             packet.NewPool(),
		proto:            minecraft.DefaultProtocol,
		packetFunc:       packetFunc,
		spawn:            make(chan struct{}),
		close:            make(chan struct{}),
		packets:          make(chan packet.Packet),
		downloadingPacks: make(map[string]*downloadingPack),
	}

	logrus.Infof("Reading replay %s", filename)

	r.f, err = os.Open(filename)
	if err != nil {
		return nil, err
	}
	stat, err := r.f.Stat()
	if err != nil {
		return nil, err
	}
	r.totalSize = stat.Size()

	err = r.readHeader()
	if err != nil {
		return nil, err
	}

	go r.loop()
	return r, nil
}

func (r *replayConnector) Close() error {
	r.once.Do(func() {
		close(r.close)
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
		return errors.New("do spawn")
	case <-ctx.Done():
		return errors.New("do spawn")
	case <-r.spawn:
		// Conn was spawned successfully.
		return nil
	}
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
		return nil, net.ErrClosed
	case p, ok := <-r.packets:
		if !ok {
			err = net.ErrClosed
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
	return r.resourcePacks
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
