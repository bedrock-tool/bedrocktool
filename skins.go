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
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Skin struct {
	protocol.Skin
}

func (skin *Skin) WriteGeometry(output_path string) error {
	os.Mkdir(output_path, 0755)
	f, err := os.Create(path.Join(output_path, "geometry.json"))
	if err != nil {
		return fmt.Errorf("failed to write Geometry %s: %s", output_path, err)
	}
	defer f.Close()
	io.Copy(f, bytes.NewReader(skin.SkinGeometry))
	return nil
}

func (skin *Skin) WriteCape(output_path string) error {
	os.Mkdir(output_path, 0755)
	f, err := os.Create(path.Join(output_path, "cape.png"))
	if err != nil {
		return fmt.Errorf("failed to write Cape %s: %s", output_path, err)
	}
	defer f.Close()
	cape_tex := image.NewRGBA(image.Rect(0, 0, int(skin.CapeImageWidth), int(skin.CapeImageHeight)))
	cape_tex.Pix = skin.CapeData

	if err := png.Encode(f, cape_tex); err != nil {
		return fmt.Errorf("error writing skin: %s", err)
	}
	return nil
}

func (skin *Skin) WriteAnimations(output_path string) error {
	os.Mkdir(output_path, 0755)
	return fmt.Errorf("%s has animations (unimplemented)", output_path)
}

func (skin *Skin) WriteTexture(output_path string) error {
	f, err := os.Create(output_path + ".png")
	if err != nil {
		return fmt.Errorf("error writing Texture: %s", err)
	}
	defer f.Close()
	skin_tex := image.NewRGBA(image.Rect(0, 0, int(skin.SkinImageWidth), int(skin.SkinImageHeight)))
	skin_tex.Pix = skin.SkinData

	if err := png.Encode(f, skin_tex); err != nil {
		return fmt.Errorf("error writing Texture: %s", err)
	}
	return nil
}

func (skin *Skin) Write(output_path, name string) error {
	complex := false
	skin_dir := path.Join(output_path, name)
	if len(skin.SkinGeometry) > 0 {
		if err := skin.WriteGeometry(skin_dir); err != nil {
			return err
		}
		complex = true
	}
	if len(skin.CapeData) > 0 {
		if err := skin.WriteCape(skin_dir); err != nil {
			return err
		}
		complex = true
	}
	if len(skin.Animations) > 0 {
		if err := skin.WriteAnimations(skin_dir); err != nil {
			return err
		}
		complex = true
	}

	var err error
	if complex {
		err = skin.WriteTexture(path.Join(skin_dir, "skin"))
	} else {
		err = skin.WriteTexture(skin_dir)
	}
	return err
}

func init() {
	register_command("skins", "skin stealer", skin_main)
}

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

func write_skin(output_path, name string, skin protocol.Skin) {
	if !strings.HasPrefix(name, skin_filter_player) {
		return
	}
	fmt.Printf("Writing skin for %s\n", name)
	_skin := &Skin{skin}
	if err := _skin.Write(output_path, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing skin: %s\n", err)
	}
}

var skin_filter_player string // who to filter
var skin_players = make(map[string]string)
var skin_player_counts = make(map[string]int)
var processed_skins = make(map[string]bool)

func process_packet_skins(conn *minecraft.Conn, out_path string, pk packet.Packet) {
	switch _pk := pk.(type) {
	case *packet.PlayerSkin:
		name := skin_players[_pk.UUID.String()]
		if name == "" {
			name = _pk.UUID.String()
		}
		skin_player_counts[name]++
		name = fmt.Sprintf("%s_%d", name, skin_player_counts[name])
		write_skin(out_path, name, _pk.Skin)
	case *packet.PlayerList:
		if _pk.ActionType == 1 { // remove
			return
		}
		for _, player := range _pk.Entries {
			name := cleanup_name(player.Username)
			if name == "" {
				name = player.UUID.String()
			}
			if _, ok := processed_skins[name]; ok {
				continue
			}
			write_skin(out_path, name, player.Skin)
			skin_players[player.UUID.String()] = name
			processed_skins[name] = true
			if conn != nil {
				send_popup(conn, fmt.Sprintf("%s Skin was Saved", name))
			}
		}
	}
}

func skin_main(ctx context.Context, args []string) error {
	var server string
	flag.StringVar(&server, "server", "", "target server")
	flag.StringVar(&skin_filter_player, "player", "", "only download the skin of this player")
	flag.CommandLine.Parse(args)
	if G_help {
		flag.Usage()
		return nil
	}

	hostname, serverConn, err := connect_server(ctx, server)
	if err != nil {
		return err
	}
	defer serverConn.Close()

	out_path := fmt.Sprintf("skins/%s", hostname)

	if err := serverConn.DoSpawnContext(ctx); err != nil {
		return err
	}

	println("Connected")
	println("Press ctrl+c to exit")

	os.MkdirAll(out_path, 0755)

	for {
		pk, err := serverConn.ReadPacket()
		if err != nil {
			return err
		}
		process_packet_skins(nil, out_path, pk)
	}
}
