package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func init() {
	register_command("skins", "skin stealer", skin_main)
}

var out_path string
var name_regexp = regexp.MustCompile(`§.`)

func cleanup_name(name string) string {
	name = strings.Split(name, "\n")[0]
	var _tmp struct {
		K string `json:"k"`
	}
	err := json.Unmarshal([]byte(name), &_tmp)
	if err == nil {
		name = _tmp.K
	}
	name = string(name_regexp.ReplaceAll([]byte(name), []byte("")))
	name = strings.TrimSpace(name)
	return name
}

// write skin as png without geometry
func write_skin_simple(name string, skin protocol.Skin) {
	f, err := os.Create(fmt.Sprintf("%s/%s.png", out_path, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing skin: %s\n", err)
		return
	}
	defer f.Close()
	skin_tex := image.NewRGBA(image.Rect(0, 0, int(skin.SkinImageWidth), int(skin.SkinImageHeight)))
	skin_tex.Pix = skin.SkinData

	if err := png.Encode(f, skin_tex); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing skin: %s\n", err)
		return
	}
}

func write_skin(name string, skin protocol.Skin) {
	fmt.Printf("Writing skin for %s\n", name)
	if len(skin.SkinGeometry) > 0 {
		fmt.Printf("%s has geometry\n", name)
		write_skin_simple(name, skin)
	} else {
		write_skin_simple(name, skin)
	}
}

func skin_main(args []string) error {
	var server string
	var help bool
	flag.StringVar(&server, "target", "", "target server")
	flag.BoolVar(&help, "help", false, "show help")
	flag.CommandLine.Parse(args)
	if help {
		flag.Usage()
		return nil
	}

	if server == "" {
		server = input_server()
	}
	if len(strings.Split(server, ":")) == 1 {
		server += ":19132"
	}
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid server: %s\n", err)
		os.Exit(1)
	}
	out_path = fmt.Sprintf("skins/%s", host)

	var packet_func func(header packet.Header, payload []byte, src, dst net.Addr) = nil
	if G_debug {
		packet_func = PacketLogger
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigs
		cancel()
		println("Exiting...")
		os.Exit(0)
	}()

	// connect
	fmt.Printf("Connecting to %s\n", server)
	serverConn, err := minecraft.Dialer{
		TokenSource: G_src,
		PacketFunc:  packet_func,
	}.DialContext(ctx, "raknet", server)
	if err != nil {
		return err
	}

	defer func() {
		if serverConn != nil {
			serverConn.Close()
			serverConn = nil
		}
	}()

	if err := serverConn.DoSpawnContext(ctx); err != nil {
		return err
	}

	println("Connected")
	println("Press ctrl+c to exit")

	os.MkdirAll(out_path, 0755)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			pk, err := serverConn.ReadPacket()
			if err != nil {
				return err
			}
			switch _pk := pk.(type) {
			case *packet.PlayerList:
				if _pk.ActionType == 1 { // remove
					continue
				}
				for _, player := range _pk.Entries {
					name := cleanup_name(player.Username)
					if name == "" {
						name = player.UUID.String()
					}
					write_skin(name, player.Skin)
				}
			}
		}
	}
}
