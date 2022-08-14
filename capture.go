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
	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var SrcIp_client = net.IPv4(127, 0, 0, 1)
var SrcIp_server = net.IPv4(243, 0, 0, 2)

func init() {
	register_command(&CaptureCMD{})
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

type CaptureCMD struct {
	server_address string
}

func (*CaptureCMD) Name() string     { return "capture" }
func (*CaptureCMD) Synopsis() string { return "capture packets in a pcap file" }

func (p *CaptureCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.server_address, "address", "", "remote server address")
}
func (c *CaptureCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *CaptureCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, hostname, err := server_input(c.server_address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	listener, serverConn, clientConn, err := create_proxy(ctx, address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fio, err := os.Create(hostname + "-" + time.Now().Format("2006-01-02_15-04-05") + ".pcap")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer fio.Close()
	w := pcapgo.NewWriter(fio)
	w.WriteFileHeader(65536, layers.LinkTypeEthernet)

	_wl := sync.Mutex{}

	go func() {
		defer listener.Disconnect(clientConn, "connection lost")
		defer serverConn.Close()

		for {
			pk, err := clientConn.ReadPacket()
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
			return 1
		}

		_wl.Lock()
		dump_packet(false, w, pk)
		_wl.Unlock()

		err = clientConn.WritePacket(pk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write packet: %s\n", err)
			return 1
		}
	}
}
