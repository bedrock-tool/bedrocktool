//go:generate protoc --go_out=./rpc --go_opt=paths=source_relative --go-grpc_out=./rpc --go-grpc_opt=paths=source_relative --go_opt=default_api_level=API_OPAQUE rpc.proto
package rui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"

	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

type Rui struct {
	listener net.Listener
	toClient chan any

	cmdCtx       context.Context
	cmdCtxCancel context.CancelCauseFunc
}

var _ ui.UI = &Rui{}

// Init implements ui.UI.
func (r *Rui) Init() error {
	r.toClient = make(chan any, 10)
	isDebug := updater.Version == ""
	messages.SetEventHandler(func(event any) error {
		r.toClient <- event
		return nil
	})
	utils.Auth.SetHandler(new(messages.AuthHandler))
	utils.ErrorHandler = func(err error) {
		if isDebug {
			panic(err)
		}
		utils.PrintPanic(err)
		messages.SendEvent(&messages.EventError{
			Error: err,
		})
	}
	return nil
}

func (r *Rui) Listen() error {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	r.listener = listener
	logrus.Debugf("rui listening on %s", listener.Addr())
	return nil
}

func (r *Rui) GetPort() int {
	return r.listener.Addr().(*net.TCPAddr).Port
}

func (r *Rui) Serve() {
	conn, err := r.listener.Accept()
	if err != nil {
		logrus.Error(err)
	}
	b := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	pr := protocol.NewReader(b, 0, false)
	pw := protocol.NewWriter(b, 0)
	go func() {
		for {
			for packet := range r.toClient {
				if err = writePacket(pw, packet); err != nil {
					return
				}
			}
		}
	}()
	for {
		packet, err := readPacket(pr)
		if err != nil {
			logrus.Error(err)
			close(r.toClient)
			return
		}
		r.receivePacket(packet)
	}
}

func (r *Rui) ReceiveUIPacket(data []byte) {
	packet, err := readPacket(protocol.NewReader(bytes.NewReader(data), 0, false))
	if err != nil {
		logrus.Error(err)
		return
	}
	r.receivePacket(packet)
}

func (r *Rui) HandlerLoop(handlerFunc func(b []byte)) {
	for packet := range r.toClient {
		b := bytes.NewBuffer(nil)
		err := writePacket(protocol.NewWriter(b, 0), packet)
		if err != nil {
			logrus.Error(err)
		}
		handlerFunc(b.Bytes())
	}
}

func (r *Rui) receivePacket(packet any) {
	switch packet := packet.(type) {
	case *getSubcommandsRequest:
		r.toClient <- r.getSubcommands()

	case *startSubcommandRequest:
		err := r.startSubcommand(packet.Name, packet.Settings)
		if err != nil {
			r.toClient <- &messages.EventError{
				Error: err,
			}
		}

	case *stopSubcommandRequest:
		r.stopSubcommand()

	case *requestLoginRequest:
		r.requestLogin()

	default:
		logrus.Errorf("invalid packet received %t", packet)
	}
}

// Start implements ui.UI.
func (r *Rui) Start(ctx context.Context, cancel context.CancelCauseFunc) error {
	<-ctx.Done()
	return nil
}

//

func (r *Rui) getSubcommands() *getSubcommandsResponse {
	var res getSubcommandsResponse
	for _, cmd := range commands.Registered {
		args, err := commands.ParseArgsType(reflect.ValueOf(cmd.Settings()), nil, nil)
		if err != nil {
			panic(err)
		}
		res.Commands = append(res.Commands, subcommand{
			Name: cmd.Name(),
			Args: args,
		})
	}
	return &res
}

func (r *Rui) startSubcommand(subcommand string, settingsJson []byte) error {
	cmd, ok := commands.Registered[subcommand]
	if !ok {
		return fmt.Errorf("no such command \"%s\"", subcommand)
	}
	settings := cmd.Settings()
	if err := json.Unmarshal(settingsJson, settings); err != nil {
		return err
	}

	r.cmdCtx, r.cmdCtxCancel = context.WithCancelCause(context.Background())
	go func() {
		err := cmd.Run(r.cmdCtx, settings)
		switch {
		case errors.Is(err, context.Canceled):
		case err != nil:
			r.toClient <- &messages.EventError{
				Error: err,
			}
		}
	}()
	return nil
}

func (r *Rui) stopSubcommand() {
	if r.cmdCtxCancel != nil {
		r.cmdCtxCancel(nil)
		r.cmdCtxCancel = nil
	}
}

func (r *Rui) requestLogin() {
	go func() {
		err := <-utils.RequestLogin()
		if err != nil {
			r.toClient <- &messages.EventError{
				Error: err,
			}
		}
	}()
}
