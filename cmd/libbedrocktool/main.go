package main

import (
	"context"
	"encoding/json"
	"flag"
	"unsafe"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

/*
#include <stdlib.h>
typedef char* (*msg_cb)(const char* message);
extern char* bridge_msg_cb(const char* message, msg_cb cb);
*/
import "C"

var flagMap = make(map[string]*flag.FlagSet)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	for k, cmd := range commands.Registered {
		f := flag.NewFlagSet("", flag.ContinueOnError)
		cmd.SetFlags(f)
		flagMap[k] = f
	}

	messages.Router.AddHandler("ui", glue.HandleMessage)
}

type uiglue struct {
	msgCB C.msg_cb
}

var glue uiglue = uiglue{}

//export SetMessageCallback
func SetMessageCallback(cb C.msg_cb) {
	glue.msgCB = cb
}

//export SendMessage
func SendMessage(msgChars *C.char, msgLen C.int) *C.char {
	msg, err := messages.Decode(C.GoBytes(unsafe.Pointer(msgChars), msgLen))
	if err != nil {
		logrus.Error(err)
		return nil
	}
	reply := messages.Router.Handle(msg)
	if reply == nil {
		return nil
	}
	return (*C.char)(C.CBytes(messages.Encode(reply)))
}

//export StartSubcommand
func StartSubcommand(sub_c *C.char) C.int {
	cmd, ok := commands.Registered[C.GoString(sub_c)]
	if !ok {
		return C.int(1)
	}
	go cmd.Execute(context.Background())
	return C.int(0)
}

//export SetSettings
func SetSettings(settings_c *C.char, settings_len C.int) C.int {
	settingsBytes := C.GoBytes(unsafe.Pointer(settings_c), settings_len)
	var settings struct {
		Name     string
		Settings map[string]string
	}
	err := json.Unmarshal(settingsBytes, &settings)
	if err != nil {
		logrus.Error(err)
		return C.int(1)
	}

	f, ok := flagMap[settings.Name]
	if !ok {
		return C.int(2)
	}

	for k, v := range settings.Settings {
		err := f.Set(k, v)
		if err != nil {
			return C.int(3)
		}
	}

	return C.int(0)
}

func (u *uiglue) HandleMessage(msg *messages.Message) *messages.Message {
	replyMsgData := C.bridge_msg_cb((*C.char)(C.CBytes(messages.Encode(msg))), glue.msgCB)
	replyMsg, err := messages.Decode([]byte(C.GoString(replyMsgData)))
	if err != nil {
		logrus.Error(err)
		return nil
	}
	C.free(unsafe.Pointer(replyMsgData))
	return replyMsg
}
