package main

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

	OLD_BROKEN := true

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	reader, err := pcapgo.NewReader(f)
	if err != nil {
		return err
	}
	SetUnexportedField(reflect.ValueOf(reader).Elem().Field(5), uint32(0xFFFFFFFF))

	dummy_conn := minecraft.NewConn()
	dummy_conn.SetGameData(minecraft.GameData{
		BaseGameVersion: "1.17.40", // SPECIFIC TO THE SERVER; TODO
	})

	proxy := ProxyContext{}
	proxy.server = dummy_conn

	if onConnect != nil {
		onConnect(&proxy)
	}

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

	start := time.Time{}
	for {
		data, ci, err := reader.ReadPacketData()
		if err != nil {
			return err
		}
		if start.Unix() == 0 {
			start = ci.Timestamp
		}

		payload := data[0x14:]
		if len(payload) == 0 {
			continue
		}

		if OLD_BROKEN {
			payload = append(fake_header, payload...)
		}

		pk_data, err := minecraft.ParseData(payload, dummy_conn)
		if err != nil {
			return err
		}
		pks, err := pk_data.Decode(dummy_conn)
		if err != nil {
			log.Error(err)
			continue
		}

		for _, pk := range pks {
			if data[0x10] == 127 { // to client
				packetCB(pk, &proxy, false)
			} else {
				packetCB(pk, &proxy, true)
			}
		}
	}
}
