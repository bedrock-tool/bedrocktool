package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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

	hostname, server := server_input(ctx, server)
	out_path := fmt.Sprintf("skins/%s", hostname)

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

	var packet_func func(header packet.Header, payload []byte, src, dst net.Addr) = nil
	if G_debug {
		packet_func = PacketLogger
	}

	fmt.Printf("Connecting to %s\n", server)
	serverConn, err := minecraft.Dialer{
		TokenSource: G_src,
		ClientData:  conn.ClientData(),
		PacketFunc:  packet_func,
	}.DialContext(ctx, "raknet", server)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %s", server, err)
	}

	println("Connected")
	println("Press ctrl+c to exit")

	os.MkdirAll(out_path, 0755)

	errs := make(chan error)
	go func() { // server -> client
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				errs <- err
				return
			}
			process_packet_skins(out_path, pk)

			conn.WritePacket(pk)
		}
	}()

	go func() { // client -> server
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				errs <- err
				return
			}
			serverConn.WritePacket(pk)
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
