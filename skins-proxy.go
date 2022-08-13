package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/sandertv/gophertunnel/minecraft"
)

func init() {
	register_command("skins-proxy", "skin stealer (proxy)", skin_proxy_main)
}

func skin_proxy_main(ctx context.Context, args []string) error {
	var server string
	if len(args) >= 1 {
		server = args[0]
		args = args[1:]
	}

	flag.StringVar(&skin_filter_player, "player", "", "only download the skin of this player")
	flag.CommandLine.Parse(args)
	if G_help {
		flag.Usage()
		return nil
	}

	address, hostname, err := server_input(server)
	if err != nil {
		return err
	}

	listener, clientConn, serverConn, err := create_proxy(ctx, address)
	if err != nil {
		return err
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
			process_packet_skins(clientConn, out_path, pk)

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
			return err
		case <-ctx.Done():
			return nil
		}
	}
}
