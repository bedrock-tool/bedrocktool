package utils

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"time"
	"unsafe"

	"github.com/google/gopacket/pcapgo"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func SetUnexportedField(field reflect.Value, value interface{}) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
		Elem().
		Set(reflect.ValueOf(value))
}

func create_replay_connection(ctx context.Context, log *logrus.Logger, filename string, onConnect ConnectCallback, packetCB PacketCallback) error {
	log.Infof("Reading replay %s", filename)

	OLD_BROKEN := false

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	reader, err := pcapgo.NewReader(f)
	if err != nil {
		return err
	}
	// maximum packet size
	SetUnexportedField(reflect.ValueOf(reader).Elem().Field(5), uint32(0xFFFFFFFF))

	proxy := NewProxy(logrus.StandardLogger())
	proxy.Server = minecraft.NewConn()

	// on old captures, the packet header is missing
	var fake_header []byte
	if OLD_BROKEN {
		// FOR OLD BROKEN CAPTURES
		fake_head := packet.Header{
			PacketID: packet.IDLevelChunk,
		}
		fake_header_w := bytes.NewBuffer(nil)
		fake_head.Write(fake_header_w)
		fake_header = fake_header_w.Bytes()
	}

	game_started := false

	start := time.Time{}
	for {
		data, ci, err := reader.ReadPacketData()
		if err != nil {
			return err
		}
		if start.Unix() == 0 {
			start = ci.Timestamp
		}
		if len(data) < 0x14 {
			continue
		}

		var payload []byte
		var toServer bool
		if OLD_BROKEN {
			payload = append(fake_header, data[0x14:]...)
			toServer = data[0x10] != 127
		} else {
			prefix := data[0:6]
			payload = data[6:]
			toServer = bytes.Equal(prefix, []byte("client"))
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
			if game_started || OLD_BROKEN {
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
