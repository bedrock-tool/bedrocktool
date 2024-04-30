package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"unsafe"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

// typedef void (*msg_cb)(int);
import "C"

var flagMap = make(map[string]*flag.FlagSet)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	for k, cmd := range commands.Registered {
		f := flag.NewFlagSet("", flag.ContinueOnError)
		cmd.SetFlags(f)
		flagMap[k] = f
	}
}

type Message struct {
	Target string
	Name   string
	Data   map[string]any
}

var okMsg = &Message{Target: "gui", Name: "ok", Data: make(map[string]any)}

var glue uiglue = uiglue{
	msgToUI:   make(chan *messages.Message),
	msgFromUI: make(chan *messages.Message),
}

//export SendMessage
func SendMessage(msgChars *C.char, msgLen int32, returnChars *C.char, returnLen int32) {
	msgData := unsafe.Slice((*byte)(unsafe.Pointer(msgChars)), int(msgLen))
	var msg Message
	err := json.Unmarshal(msgData, &msg)
	if err != nil {
		logrus.Error(err)
	}

	ret := glue.handleCMessage(&msg)
	retData, err := json.Marshal(ret)
	if err != nil {
		logrus.Error(err)
	}

	copy(unsafe.Slice((*byte)(unsafe.Pointer(returnChars)), int(returnLen)), retData)
}

//export ReadMessage
func ReadMessage(returnChars *C.char, returnLen int32) {
	msg := <-glue.msgToUI
	retData, err := json.Marshal(&Message{
		Target: "gui",
		Name:   "ui-msg",
		Data: map[string]any{
			"msg": msg,
		},
	})
	if err != nil {
		logrus.Error(err)
	}

	copy(unsafe.Slice((*byte)(unsafe.Pointer(returnChars)), int(returnLen)), retData)
}

type uiglue struct {
	msgToUI   chan *messages.Message
	msgFromUI chan *messages.Message
}

func (u *uiglue) HandleMessage(msg *messages.Message) *messages.Message {
	u.msgToUI <- msg
	return <-u.msgFromUI
}

func (u *uiglue) handleCMessage(msg *Message) (ret *Message) {
	switch msg.Name {
	case "ui-reply":
		u.msgFromUI <- &messages.Message{
			Source: "gui",
			Data:   msg.Data,
		}

	case "set-settings":
		sub := msg.Data["subcommand"].(string)
		settings := msg.Data["settings"].(map[string]string)

		_, ok := commands.Registered[sub]
		if !ok {
			return errMsg(errors.New("unknown command"))
		}
		f := flagMap[sub]
		for k, v := range settings {
			err := f.Set(k, v)
			if err != nil {
				return errMsg(err)
			}
		}
		return okMsg
	case "start-subcommand":
		sub := msg.Data["subcommand"].(string)
		cmd, ok := commands.Registered[sub]
		if !ok {
			return errMsg(errors.New("unknown command"))
		}
		go func() {
			cmd.Execute(context.Background(), &glue)
		}()
	}

	return errMsg(errors.ErrUnsupported)
}

func errMsg(err error) *Message {
	return &Message{
		Target: "gui",
		Name:   "error",
		Data: map[string]any{
			"error": err.Error(),
		},
	}
}
