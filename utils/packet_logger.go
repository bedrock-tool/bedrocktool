package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/fatih/color"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

var pool = packet.NewPool()

var MutedPackets = []string{
	"packet.UpdateBlock",
	"packet.MoveActorAbsolute",
	"packet.SetActorMotion",
	"packet.SetTime",
	"packet.RemoveActor",
	"packet.AddActor",
	"packet.UpdateAttributes",
	"packet.Interact",
	"packet.LevelEvent",
	"packet.SetActorData",
	"packet.MoveActorDelta",
	"packet.MovePlayer",
	"packet.BlockActorData",
	"packet.PlayerAuthInput",
	"packet.LevelChunk",
	"packet.LevelSoundEvent",
	"packet.ActorEvent",
	"packet.NetworkChunkPublisherUpdate",
	"packet.UpdateSubChunkBlocks",
	"packet.SubChunk",
	"packet.SubChunkRequest",
	"packet.Animate",
	"packet.NetworkStackLatency",
	"packet.InventoryTransaction",
	"packet.PlaySound",
}

var ExtraVerbose []string
var F_Log io.Writer
var dmp_lock sync.Mutex

func dmp_struct(level int, in any, w_type bool) (s string) {
	t_base := strings.Repeat("\t", level)

	ii := reflect.Indirect(reflect.ValueOf(in))
	if w_type {
		type_name := reflect.TypeOf(in).String()
		s += type_name + " "
	} else {
		s += "\t"
	}

	if ii.Kind() == reflect.Struct {
		s += "{\n"
		for i := 0; i < ii.NumField(); i++ {
			field := ii.Type().Field(i)
			if field.IsExported() {
				d := dmp_struct(level+1, ii.Field(i).Interface(), true)
				s += t_base + fmt.Sprintf("\t%s = %s\n", field.Name, d)
			} else {
				s += t_base + "\t" + field.Name + " (unexported)"
			}
		}
		s += t_base + "}\n"
	} else if ii.Kind() == reflect.Slice {
		var t reflect.Type
		if ii.Len() > 0 {
			e := ii.Index(0)
			t = reflect.TypeOf(e.Interface())
		}
		if ii.Len() > 1000 {
			s += " [<slice too long>]"
		} else if ii.Len() == 0 || t.Kind() == reflect.Struct {
			s += "\t[\n"
			for i := 0; i < ii.Len(); i++ {
				s += t_base
				s += dmp_struct(level+1, ii.Index(i).Interface(), false)
			}
			s += t_base + "]\n"
		} else {
			s += fmt.Sprintf("%#v", ii.Interface())
		}
	} else if ii.Kind() == reflect.Map {
		j, err := json.MarshalIndent(ii.Interface(), t_base, "\t")
		if err != nil {
			s += err.Error()
		}
		s += string(j)
	} else {
		s += fmt.Sprintf(" %#v", ii.Interface())
	}

	return s
}

func PacketLogger(header packet.Header, payload []byte, src, dst net.Addr) {
	var pk packet.Packet
	if pkFunc, ok := pool[header.PacketID]; ok {
		pk = pkFunc()
	} else {
		pk = &packet.Unknown{PacketID: header.PacketID}
	}

	defer func() {
		if recoveredErr := recover(); recoveredErr != nil {
			logrus.Errorf("%T: %w", pk, recoveredErr)
		}
	}()

	pk.Unmarshal(protocol.NewReader(bytes.NewBuffer(payload), 0))

	if F_Log != nil {
		dmp_lock.Lock()
		defer dmp_lock.Unlock()
		F_Log.Write([]byte(dmp_struct(0, pk, true)))
		F_Log.Write([]byte("\n\n"))
	}

	pk_name := reflect.TypeOf(pk).String()[1:]
	if slices.Contains(MutedPackets, pk_name) {
		return
	}

	switch pk := pk.(type) {
	case *packet.Disconnect:
		logrus.Infof(locale.Loc("disconnect", locale.Strmap{"Pk": pk}))
	}

	dir_S2C := color.GreenString("S") + "->" + color.CyanString("C")
	dir_C2S := color.CyanString("C") + "->" + color.GreenString("S")
	var dir string = dir_S2C

	if Client_addr != nil {
		if src == Client_addr {
			dir = dir_C2S
		}
	} else {
		src_addr, _, _ := net.SplitHostPort(src.String())
		if IPPrivate(net.ParseIP(src_addr)) {
			dir = dir_C2S
		}
	}

	logrus.Debugf("%s 0x%02x, %s", dir, pk.ID(), pk_name)

	if slices.Contains(ExtraVerbose, pk_name) {
		logrus.Debugf("%+v", pk)
	}
}
