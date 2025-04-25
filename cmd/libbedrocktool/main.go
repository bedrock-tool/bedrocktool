package main

import (
	"os"
	"unsafe"

	"github.com/bedrock-tool/bedrocktool/ui/rui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sirupsen/logrus"
)

/*
#include <stdint.h>
typedef void (*bedrocktoolHandler)(uint8_t* data, size_t length);
void call_bedrocktool_handler(bedrocktoolHandler h, uint8_t* data, size_t length) ;
*/
import "C"

var ui = &rui.Rui{}

func main() {
	select {}
}

//export BedrocktoolInit
func BedrocktoolInit() int {
	env, ok := os.LookupEnv("BEDROCK_ENV")
	if !ok {
		env = "prod"
	}
	err := utils.Auth.Startup(env)
	if err != nil {
		logrus.Error(err)
		return -1
	}

	if err := ui.Init(); err != nil {
		logrus.Error(err)
		return -1
	}

	return 0
}

//export BedrocktoolSetHandler
func BedrocktoolSetHandler(handler C.bedrocktoolHandler) {
	ui.HandlerLoop(func(b []byte) {
		C.call_bedrocktool_handler(handler, (*C.uint8_t)(&b[0]), C.size_t(len(b)))
	})
}

//export BedrocktoolSendPacket
func BedrocktoolSendPacket(dataP *byte, length C.size_t) {
	data := unsafe.Slice(dataP, int(length))
	ui.ReceiveUIPacket(data)
}
