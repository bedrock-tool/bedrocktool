package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
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

var players = make(map[string]string)

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

func write_skin_geometry(output_path string, skin protocol.Skin) {
	os.Mkdir(output_path, 0755)
	f, err := os.Create(path.Join(output_path, "geometry.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write skin geom %s: %s\n", out_path, err)
		return
	}
	defer f.Close()
	io.Copy(f, bytes.NewReader(skin.SkinGeometry))
}

func write_skin_texture(name string, skin protocol.Skin) {
	f, err := os.Create(name + ".png")
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

func write_skin_cape(output_path string, skin protocol.Skin) {
	os.Mkdir(output_path, 0755)
	f, err := os.Create(path.Join(output_path, "cape.png"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write skin cape %s: %s\n", out_path, err)
		return
	}
	defer f.Close()
	cape_tex := image.NewRGBA(image.Rect(0, 0, int(skin.CapeImageWidth), int(skin.CapeImageHeight)))
	cape_tex.Pix = skin.CapeData

	if err := png.Encode(f, cape_tex); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing skin: %s\n", err)
		return
	}
}

func write_skin_animations(output_path string, skin protocol.Skin) {
	os.Mkdir(output_path, 0755)
	fmt.Printf("%s has animations (unimplemented)\n", output_path)
}

func write_skin(name string, skin protocol.Skin) {
	if !strings.HasPrefix(name, player) {
		return
	}
	fmt.Printf("Writing skin for %s\n", name)
	complex := false
	skin_dir := path.Join(out_path, name)
	if len(skin.SkinGeometry) > 0 {
		write_skin_geometry(skin_dir, skin)
		complex = true
	}
	if len(skin.CapeData) > 0 {
		write_skin_cape(skin_dir, skin)
		complex = true
	}
	if len(skin.Animations) > 0 {
		write_skin_animations(skin_dir, skin)
		complex = true
	}

	if complex {
		write_skin_texture(path.Join(skin_dir, "skin"), skin)
	} else {
		write_skin_texture(skin_dir, skin)
	}
}

var player string

func skin_main(args []string) error {
	var server string
	var help bool
	flag.StringVar(&server, "target", "", "target server")
	flag.StringVar(&player, "player", "", "only download the skin of this player")
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
			case *packet.PlayerSkin:
				name := players[_pk.UUID.String()]
				if name == "" {
					name = _pk.UUID.String()
				}
				write_skin(name, _pk.Skin)
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
					players[player.UUID.String()] = name
				}
			}
		}
	}
}
