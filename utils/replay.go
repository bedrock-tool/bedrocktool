package utils

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func createReplayConnection(ctx context.Context, filename string, onConnect ConnectCallback, packetCB PacketCallback) error {
	logrus.Infof("Reading replay %s", filename)

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	var size int64
	{
		stat, err := f.Stat()
		if err != nil {
			return err
		}
		size = stat.Size()
	}

	proxy := NewProxy()
	proxy.Server = minecraft.NewConn()

	gameStarted := false
	i := 0
	for {
		i += 1
		var magic uint32 = 0
		var packetLength uint32 = 0
		var toServer bool = false

		offset, _ := f.Seek(0, io.SeekCurrent)
		if offset == size {
			logrus.Info("Reached End")
			return nil
		}

		binary.Read(f, binary.LittleEndian, &magic)
		if magic != 0xAAAAAAAA {
			logrus.Fatal("Wrong Magic")
		}
		binary.Read(f, binary.LittleEndian, &packetLength)
		binary.Read(f, binary.LittleEndian, &toServer)
		payload := make([]byte, packetLength)
		n, err := f.Read(payload)
		if err != nil {
			logrus.Error(err)
			return nil
		}
		if n != int(packetLength) {
			logrus.Errorf("Truncated %d", i)
			return nil
		}

		var magic2 uint32
		binary.Read(f, binary.LittleEndian, &magic2)
		if magic2 != 0xBBBBBBBB {
			logrus.Fatal("Wrong Magic2")
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
					packetCB(pk, proxy, toServer)
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
