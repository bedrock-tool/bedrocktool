package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var SrcIp_client = net.IPv4(127, 0, 0, 1)
var SrcIp_server = net.IPv4(243, 0, 0, 2)

func init() {
	register_command("capture", "capture packets", packets_main)
}

func dump_packet(from_client bool, w *pcapgo.Writer, pk packet.Packet) {
	var err error
	var iface_index int
	var src_ip, dst_ip net.IP
	if from_client {
		iface_index = 1
		src_ip = SrcIp_client
		dst_ip = SrcIp_server
	} else {
		iface_index = 2
		src_ip = SrcIp_server
		dst_ip = SrcIp_client
	}

	var packet_data []byte
	{
		_pw := bytes.NewBuffer(nil)
		pw := protocol.NewWriter(_pw, 0x0)
		pk.Marshal(pw)
		packet_data = _pw.Bytes()
	}

	serialize_buf := gopacket.NewSerializeBuffer()
	err = gopacket.SerializeLayers(
		serialize_buf,
		gopacket.SerializeOptions{},
		&layers.IPv4{
			SrcIP:  src_ip,
			DstIP:  dst_ip,
			Length: uint16(len(packet_data)),
		},
		gopacket.Payload(packet_data),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = w.WritePacket(gopacket.CaptureInfo{
		Timestamp:      time.Now(),
		Length:         len(serialize_buf.Bytes()),
		CaptureLength:  len(serialize_buf.Bytes()),
		InterfaceIndex: iface_index,
	}, serialize_buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
}

func packets_main(ctx context.Context, args []string) error {
	var server string
	flag.StringVar(&server, "server", "", "target server")
	flag.CommandLine.Parse(args)
	if G_help {
		flag.Usage()
		return nil
	}

	var hostname string
	hostname, server = server_input(ctx, server)

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
		log.Fatal(err)
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
		fmt.Fprintf(os.Stderr, "Failed to connect to %s: %s\n", server, err)
		return nil
	}

	if err := spawn_conn(ctx, conn, serverConn); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to spawn: %s\n", err)
		return nil
	}

	f, err := os.Create(hostname + "-" + time.Now().Format("2006-01-02_15-04-05") + ".pcap")
	if err != nil {
		return err
	}
	defer f.Close()
	w := pcapgo.NewWriter(f)
	w.WriteFileHeader(65536, layers.LinkTypeEthernet)

	_wl := sync.Mutex{}
	/* TEST
	{
		for i := 0; i < 1000; i++ {
			dump_packet(false, w, &packet.SetTitle{
				Text: fmt.Sprintf("Test %d", i),
			})
			dump_packet(true, w, &packet.MovePlayer{
				Tick: uint64(i),
			})
		}
	}
	return nil
	*/
	go func() {
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()

		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read packet: %s\n", err)
				return
			}

			_wl.Lock()
			dump_packet(true, w, pk)
			_wl.Unlock()

			err = serverConn.WritePacket(pk)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write packet: %s\n", err)
				return
			}
		}
	}()

	for {
		pk, err := serverConn.ReadPacket()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read packet: %s\n", err)
			return nil
		}

		_wl.Lock()
		dump_packet(false, w, pk)
		_wl.Unlock()

		err = conn.WritePacket(pk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write packet: %s\n", err)
			return nil
		}
	}
}
