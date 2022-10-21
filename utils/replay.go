package utils

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"reflect"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func create_replay_connection(ctx context.Context, log *logrus.Logger, filename string, onConnect ConnectCallback, packetCB PacketCallback) error {
	log.Infof("Reading replay %s", filename)

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

	proxy := NewProxy(logrus.StandardLogger())
	proxy.Server = minecraft.NewConn()

	game_started := false
	i := 0
	for {
		i += 1
		var magic uint32 = 0
		var packet_length uint32 = 0
		var toServer bool = false

		offset, _ := f.Seek(0, io.SeekCurrent)
		if offset == size {
			log.Info("Reached End")
			return nil
		}

		binary.Read(f, binary.LittleEndian, &magic)
		if magic != 0xAAAAAAAA {
			logrus.Fatal("Wrong Magic")
		}
		binary.Read(f, binary.LittleEndian, &packet_length)
		binary.Read(f, binary.LittleEndian, &toServer)
		payload := make([]byte, packet_length)
		n, err := f.Read(payload)
		if err != nil {
			log.Error(err)
			return nil
		}
		if n != int(packet_length) {
			log.Errorf("Truncated %d", i)
			return nil
		}

		var magic2 uint32
		binary.Read(f, binary.LittleEndian, &magic2)
		if magic2 != 0xBBBBBBBB {
			logrus.Fatal("Wrong Magic2")
		}

		pk_data, err := minecraft.ParseData(payload, proxy.Server)
		if err != nil {
			return err
		}
		pks, err := pk_data.Decode(proxy.Server)
		if err != nil {
			log.Error(err)
			continue
		}

		for _, pk := range pks {
			logrus.Printf("%s", reflect.TypeOf(pk).String()[1:])

			if game_started {
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
					game_started = true
					if onConnect != nil {
						onConnect(proxy)
					}
				}
			}
		}
	}
}
