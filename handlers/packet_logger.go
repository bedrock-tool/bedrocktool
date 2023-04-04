package handlers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/crypt"
	"github.com/fatih/color"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

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

var (
	FLog    io.Writer
	dmpLock sync.Mutex
)

func dmpStruct(level int, inputStruct any, withType bool, isInList bool) (s string) {
	tBase := strings.Repeat("\t", level)

	if inputStruct == nil {
		return "nil"
	}

	ii := reflect.Indirect(reflect.ValueOf(inputStruct))
	typeName := reflect.TypeOf(inputStruct).String()
	if typeName == "[]interface {}" {
		typeName = "[]any"
	}
	typeString := ""
	if withType {
		if slices.Contains([]string{"bool", "string"}, typeName) {
		} else {
			typeString = typeName
		}
	}

	if strings.HasPrefix(typeName, "protocol.Optional") {
		v := ii.MethodByName("Value").Call(nil)
		val, set := v[0], v[1]
		if !set.Bool() {
			s += typeName + " Not Set"
		} else {
			s += typeName + "{\n" + tBase + "\t"
			s += dmpStruct(level+1, val.Interface(), false, false)
			s += "\n" + tBase + "}"
		}
		return
	}

	switch ii.Kind() {
	case reflect.Struct:
		if ii.NumField() == 0 {
			s += typeName + "{}"
		} else {
			s += typeName + "{\n"
			for i := 0; i < ii.NumField(); i++ {
				fieldType := ii.Type().Field(i)

				if fieldType.IsExported() {
					s += fmt.Sprintf("%s\t%s: %s,\n", tBase, fieldType.Name, dmpStruct(level+1, ii.Field(i).Interface(), true, false))
				} else {
					s += tBase + " " + fieldType.Name + " (unexported)"
				}
			}
			s += tBase + "}"
		}
	case reflect.Slice:
		s += typeName + "{"

		if ii.Len() > 1000 {
			s += "<slice too long>"
		} else if ii.Len() == 0 {
		} else {
			e := ii.Index(0)
			t := reflect.TypeOf(e.Interface())
			is_elem_struct := t.Kind() == reflect.Struct

			if is_elem_struct {
				s += "\n"
			}
			for i := 0; i < ii.Len(); i++ {
				if is_elem_struct {
					s += tBase + "\t"
				}
				s += dmpStruct(level+1, ii.Index(i).Interface(), false, true) + ","
				if is_elem_struct {
					s += "\n"
				} else {
					if i != ii.Len()-1 {
						s += " "
					}
				}
			}
			if is_elem_struct {
				s += tBase
			}
		}
		s += "}"
	case reflect.Map:
		it := reflect.TypeOf(inputStruct)
		valType := it.Elem().String()
		if valType == "interface {}" {
			valType = "any"
		}
		keyType := it.Key().String()

		s += fmt.Sprintf("map[%s]%s{", keyType, valType)
		if ii.Len() > 0 {
			s += "\n"
		}

		iter := ii.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			s += fmt.Sprintf("%s\t%#v: %s,\n", tBase, k.Interface(), dmpStruct(level+1, v.Interface(), true, false))
		}

		if ii.Len() > 0 {
			s += tBase
		}
		s += "}"
	default:
		is_array := ii.Kind() == reflect.Array
		add_type := !isInList && !is_array && len(typeString) > 0
		if add_type {
			s += typeString + "("
		}
		s += fmt.Sprintf("%#v", ii.Interface())
		if add_type {
			s += ")"
		}
	}
	return s
}

func DumpStruct(data interface{}) {
	if FLog == nil {
		return
	}

	FLog.Write([]byte(dmpStruct(0, data, true, false)))
	FLog.Write([]byte("\n\n\n"))
}

var dirS2C = color.GreenString("S") + "->" + color.CyanString("C")
var dirC2S = color.CyanString("C") + "->" + color.GreenString("S")
var pool = packet.NewPool()

func NewDebugLogger(extraVerbose bool) *utils.ProxyHandler {
	var logPlain, logCrypt, logCryptEnc io.WriteCloser
	var packetsLogF *bufio.Writer

	if extraVerbose {
		// open plain text log
		logPlain, err := os.Create("packets.log")
		if err != nil {
			logrus.Error(err)
		}

		// open gpg log
		logCrypt, err := os.Create("packets.log.gpg")
		if err != nil {
			logrus.Error(err)
		} else {
			// encrypter for the log
			logCryptEnc, err = crypt.Encer("packets.log", logCrypt)
			if err != nil {
				logrus.Error(err)
			}
		}
		packetsLogF = bufio.NewWriter(io.MultiWriter(logPlain, logCryptEnc))
	}

	var proxy *utils.ProxyContext

	return &utils.ProxyHandler{
		Name: "Debug",
		ProxyRef: func(pc *utils.ProxyContext) {
			proxy = pc
		},
		PacketFunc: func(header packet.Header, payload []byte, src, dst net.Addr) {
			var pk packet.Packet
			if pkFunc, ok := pool[header.PacketID]; ok {
				pk = pkFunc()
			} else {
				pk = &packet.Unknown{PacketID: header.PacketID, Payload: payload}
			}

			defer func() {
				if recoveredErr := recover(); recoveredErr != nil {
					logrus.Errorf("%T: %s", pk, recoveredErr.(error))
				}
			}()
			pk.Marshal(protocol.NewReader(bytes.NewBuffer(payload), 0))

			if extraVerbose {
				dmpLock.Lock()
				packetsLogF.Write([]byte(dmpStruct(0, pk, true, false) + "\n\n\n"))
				dmpLock.Unlock()
			}

			pkName := reflect.TypeOf(pk).String()[1:]
			if !slices.Contains(MutedPackets, pkName) {
				var dir string = dirS2C
				if proxy.IsClient(src) {
					dir = dirC2S
				}
				logrus.Debugf("%s 0x%02x, %s", dir, pk.ID(), pkName)
			}
		},
		OnEnd: func() {
			if packetsLogF != nil {
				packetsLogF.Flush()
			}
			if logPlain != nil {
				logPlain.Close()
			}
			if logCryptEnc != nil {
				logCryptEnc.Close()
			}
			if logCrypt != nil {
				logCrypt.Close()
			}
		},
	}
}

func init() {
	// hacky solution to allow proxy to add this
	utils.NewDebugLogger = NewDebugLogger
}
