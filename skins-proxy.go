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
	flag.StringVar(&server, "server", "", "target server")
	flag.StringVar(&skin_filter_player, "player", "", "only download the skin of this player")
	flag.CommandLine.Parse(args)
	if G_help {
		flag.Usage()
		return nil
	}

	_status := minecraft.NewStatusProvider("Server")
	listener, err := minecraft.ListenConfig{
		StatusProvider: _status,
	}.Listen("raknet", ":19132")
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("Listening on %s\n", listener.Addr())

	c, err := listener.Accept()
	if err != nil {
		return err
	}
	conn := c.(*minecraft.Conn)

	hostname, serverConn, err := connect_server(ctx, server)
	if err != nil {
		return err
	}
	out_path := fmt.Sprintf("skins/%s", hostname)

	if err := spawn_conn(ctx, conn, serverConn); err != nil {
		return err
	}

	println("Connected")
	println("Press ctrl+c to exit")

	os.MkdirAll(out_path, 0755)

	errs := make(chan error, 2)
	go func() { // server -> client
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
			process_packet_skins(conn, out_path, pk)

			if err = conn.WritePacket(pk); err != nil {
				return
			}
		}
	}()

	go func() { // client -> server
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}

			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
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
