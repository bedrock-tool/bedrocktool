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

	"github.com/flytam/filenamify"
	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Skin struct {
	protocol.Skin
}

// WriteGeometry writes the geometry json for the skin to output_path
func (skin *Skin) WriteGeometry(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("failed to write Geometry %s: %s", output_path, err)
	}
	defer f.Close()
	io.Copy(f, bytes.NewReader(skin.SkinGeometry))
	return nil
}

// WriteCape writes the cape as a png at output_path
func (skin *Skin) WriteCape(output_path string) error {
	f, err := os.Create(output_path)
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

// WriteAnimations writes skin animations to the folder
func (skin *Skin) WriteAnimations(output_path string) error {
	fmt.Printf("Warn: %s has animations (unimplemented)", output_path)
	return nil
}

// WriteTexture writes the main texture for this skin to a file
func (skin *Skin) WriteTexture(output_path string) error {
	f, err := os.Create(output_path)
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

func (skin *Skin) WriteTint(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("failed to write Tint %s: %s", output_path, err)
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(skin.PieceTintColours)
	if err != nil {
		return fmt.Errorf("failed to write Tint %s: %s", output_path, err)
	}
	return nil
}

func (skin *Skin) WriteMeta(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("failed to write Tint %s: %s", output_path, err)
	}
	defer f.Close()
	d, err := json.MarshalIndent(struct {
		SkinID        string
		PlayFabID     string
		PremiumSkin   bool
		PersonaSkin   bool
		CapeID        string
		SkinColour    string
		ArmSize       string
		Trusted       bool
		PersonaPieces []protocol.PersonaPiece
	}{
		skin.SkinID,
		skin.PlayFabID,
		skin.PremiumSkin,
		skin.PersonaSkin,
		skin.CapeID,
		skin.SkinColour,
		skin.ArmSize,
		skin.Trusted,
		skin.PersonaPieces,
	}, "", "    ")
	if err != nil {
		return err
	}
	f.Write(d)
	return nil
}

// Write writes all data for this skin to a folder
func (skin *Skin) Write(output_path, name string) error {
	name, _ = filenamify.FilenamifyV2(name)
	skin_dir := path.Join(output_path, name)

	have_geometry, have_cape, have_animations, have_tint := len(skin.SkinGeometry) > 0, len(skin.CapeData) > 0, len(skin.Animations) > 0, len(skin.PieceTintColours) > 0

	os.MkdirAll(skin_dir, 0755)
	if have_geometry {
		if err := skin.WriteGeometry(path.Join(skin_dir, "geometry.json")); err != nil {
			return err
		}
	}
	if have_cape {
		if err := skin.WriteCape(path.Join(skin_dir, "cape.png")); err != nil {
			return err
		}
	}
	if have_animations {
		if err := skin.WriteAnimations(skin_dir); err != nil {
			return err
		}
	}
	if have_tint {
		if err := skin.WriteTint(path.Join(skin_dir, "tint.json")); err != nil {
			return err
		}
	}

	if err := skin.WriteMeta(path.Join(skin_dir, "metadata.json")); err != nil {
		return err
	}

	return skin.WriteTexture(path.Join(skin_dir, "skin.png"))
}

var name_regexp = regexp.MustCompile(`\||(?:§.?)`)

// cleans name so it can be used as a filename
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

// puts the skin at output_path if the filter matches it
// internally converts the struct so it can use the extra methods
func write_skin(output_path, name string, skin protocol.Skin, filter string) {
	if !strings.HasPrefix(name, filter) {
		return
	}
	fmt.Printf("Writing skin for %s\n", name)
	_skin := &Skin{skin}
	if err := _skin.Write(output_path, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing skin: %s\n", err)
	}
}

var skin_players = make(map[string]string)
var skin_player_counts = make(map[string]int)
var processed_skins = make(map[string]bool)

func process_packet_skins(conn *minecraft.Conn, out_path string, pk packet.Packet, filter string) {
	switch _pk := pk.(type) {
	case *packet.PlayerSkin:
		name := skin_players[_pk.UUID.String()]
		if name == "" {
			name = _pk.UUID.String()
		}
		skin_player_counts[name]++
		name = fmt.Sprintf("%s_%d", name, skin_player_counts[name])
		write_skin(out_path, name, _pk.Skin, filter)
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
			write_skin(out_path, name, player.Skin, filter)
			skin_players[player.UUID.String()] = name
			processed_skins[name] = true
			if conn != nil {
				send_popup(conn, fmt.Sprintf("%s Skin was Saved", name))
			}
		}
	}
}

type SkinCMD struct {
	server_address string
	filter         string
}

func (*SkinCMD) Name() string     { return "skins" }
func (*SkinCMD) Synopsis() string { return "download all skins from players on a server" }

func (c *SkinCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.server_address, "address", "", "remote server address")
	f.StringVar(&c.filter, "filter", "", "player name filter prefix")
}
func (c *SkinCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *SkinCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, hostname, err := server_input(c.server_address)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		return 1
	}

	serverConn, err := connect_server(ctx, address, nil, false)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		return 1
	}
	defer serverConn.Close()

	out_path := fmt.Sprintf("skins/%s", hostname)

	if err := serverConn.DoSpawnContext(ctx); err != nil {
		fmt.Fprint(os.Stderr, err)
		return 1
	}

	println("Connected")
	println("Press ctrl+c to exit")

	os.MkdirAll(out_path, 0755)

	for {
		pk, err := serverConn.ReadPacket()
		if err != nil {
			return 1
		}
		process_packet_skins(nil, out_path, pk, c.filter)
	}
}

func init() {
	register_command(&SkinCMD{})
}
