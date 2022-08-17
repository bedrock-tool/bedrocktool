package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft"
)

type SkinProxyCMD struct {
	server_address string
	filter         string
}

func (*SkinProxyCMD) Name() string     { return "skins-proxy" }
func (*SkinProxyCMD) Synopsis() string { return "download skins from players on a server with proxy" }

func (c *SkinProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.server_address, "address", "", "remote server address")
	f.StringVar(&c.filter, "filter", "", "player name filter prefix")
}
func (c *SkinProxyCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + SERVER_ADDRESS_HELP
}

func (c *SkinProxyCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, hostname, err := server_input(c.server_address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	listener, clientConn, serverConn, err := create_proxy(ctx, address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer listener.Close()

	out_path := fmt.Sprintf("skins/%s", hostname)

	println("Connected")
	println("Press ctrl+c to exit")

	os.MkdirAll(out_path, 0755)

	errs := make(chan error, 2)
	go func() { // server -> client
		defer serverConn.Close()
		defer listener.Disconnect(clientConn, "connection lost")
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(clientConn, disconnect.Error())
				}
				return
			}
			process_packet_skins(clientConn, out_path, pk, c.filter)

			if err = clientConn.WritePacket(pk); err != nil {
				return
			}
		}
	}()

	go func() { // client -> server
		for {
			pk, err := clientConn.ReadPacket()
			if err != nil {
				return
			}

			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(clientConn, disconnect.Error())
				}
				return
			}
		}
	}()

	for {
		select {
		case err := <-errs:
			fmt.Fprintln(os.Stderr, err)
			return 1
		case <-ctx.Done():
			return 0
		}
	}
}

func init() {
	register_command(&SkinProxyCMD{})
}
