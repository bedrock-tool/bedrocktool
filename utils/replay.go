package utils

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

func createReplayConnection(ctx context.Context, filename string, onConnect ConnectCallback, packetCB PacketCallback) error {
	logrus.Infof("Reading replay %s", filename)

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	totalSize := stat.Size()

	// default version is version 1, since that didnt have a header
	ver := 1

	magic := make([]byte, 4)
	io.ReadAtLeast(f, magic, 4)
	if bytes.Equal(magic, replayMagic) {
		var header replayHeader
		if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
			return err
		}
		ver = int(header.Version)
	} else {
		logrus.Info("Version 1 capture assumed.")
		f.Seek(-4, io.SeekCurrent)
	}

	proxy, _ := NewProxy("")
	proxy.Server = minecraft.NewConn()

	gameStarted := false
	i := 0
	for {
		i += 1
		var magic uint32 = 0
		var packetLength uint32 = 0
		var toServer bool = false
		timeReceived := time.Now()

		offset, _ := f.Seek(0, io.SeekCurrent)
		if offset == totalSize {
			logrus.Info("Reached End")
			return nil
		}

		binary.Read(f, binary.LittleEndian, &magic)
		if magic != 0xAAAAAAAA {
			return fmt.Errorf("wrong Magic")
		}
		binary.Read(f, binary.LittleEndian, &packetLength)
		binary.Read(f, binary.LittleEndian, &toServer)
		if ver >= 2 {
			var timeMs int64
			binary.Read(f, binary.LittleEndian, &timeMs)
			timeReceived = time.UnixMilli(timeMs)
		}

		payload := make([]byte, packetLength)
		n, err := f.Read(payload)
		if err != nil {
			logrus.Error(err)
			return nil
		}
		if n != int(packetLength) {
			return fmt.Errorf("truncated %d", i)
		}

		var magic2 uint32
		binary.Read(f, binary.LittleEndian, &magic2)
		if magic2 != 0xBBBBBBBB {
			return fmt.Errorf("wrong Magic2")
		}

		pkData, err := minecraft.ParseData(payload, proxy.Server)
		if err != nil {
			return err
		}
		pks, err := pkData.Decode(proxy.Server)
		if err != nil {
			logrus.Error(err)
			continue
		}

		for _, pk := range pks {
			f := bytes.NewBuffer(nil)
			b := protocol.NewWriter(f, 0)
			pk.Marshal(b)

			if GDebug {
				PacketLogger(packet.Header{PacketID: pk.ID()}, f.Bytes(), &net.UDPAddr{}, &net.UDPAddr{})
			}

			if gameStarted {
				if packetCB != nil {
					packetCB(pk, proxy, toServer, timeReceived)
				}
			} else {
				switch pk := pk.(type) {
				case *packet.StartGame:
					proxy.Server.SetGameData(minecraft.GameData{
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
					gameStarted = true
					if onConnect != nil {
						onConnect(proxy)
					}
				}
			}
		}
	}
}
