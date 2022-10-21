package skins

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
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/flytam/filenamify"
	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type Skin struct {
	protocol.Skin
}

type SkinMeta struct {
	SkinID        string
	PlayFabID     string
	PremiumSkin   bool
	PersonaSkin   bool
	CapeID        string
	SkinColour    string
	ArmSize       string
	Trusted       bool
	PersonaPieces []protocol.PersonaPiece
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
	logrus.Warnf("%s has animations (unimplemented)", output_path)
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
	d, err := json.MarshalIndent(SkinMeta{
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

func (skin *Skin) Complex() bool {
	have_geometry, have_cape, have_animations, have_tint := len(skin.SkinGeometry) > 0, len(skin.CapeData) > 0, len(skin.Animations) > 0, len(skin.PieceTintColours) > 0
	return have_geometry || have_cape || have_animations || have_tint
}

// Write writes all data for this skin to a folder
func (skin *Skin) Write(output_path, name string) error {
	name, _ = filenamify.FilenamifyV2(name)
	skin_dir := path.Join(output_path, name)

	have_geometry, have_cape, have_animations, have_tint := len(skin.SkinGeometry) > 0, len(skin.CapeData) > 0, len(skin.Animations) > 0, len(skin.PieceTintColours) > 0
	os.MkdirAll(skin_dir, 0o755)
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

	return skin.WriteTexture(skin_dir + "/skin.png")
}

// puts the skin at output_path if the filter matches it
// internally converts the struct so it can use the extra methods
func write_skin(output_path, name string, skin *protocol.Skin) {
	logrus.Infof("Writing skin for %s\n", name)
	_skin := &Skin{*skin}
	if err := _skin.Write(output_path, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing skin: %s\n", err)
	}
}

var (
	skin_players       = make(map[string]string)
	skin_player_counts = make(map[string]int)
)

func popup_skin_saved(conn *minecraft.Conn, name string) {
	if conn != nil {
		(&utils.ProxyContext{Client: conn}).SendPopup(fmt.Sprintf("%s Skin was Saved", name))
	}
}

func skin_meta_get_skinid(path string) string {
	cont, err := os.ReadFile(fmt.Sprintf("%s/metadata.json", path))
	if err != nil {
		return ""
	}
	var meta SkinMeta
	if err := json.Unmarshal(cont, &meta); err != nil {
		return ""
	}
	return meta.SkinID
}

func save_player_skin(conn *minecraft.Conn, out_path, player_name string, skin *protocol.Skin) {
	count := skin_player_counts[player_name]
	if count > 0 {
		meta_id := skin_meta_get_skinid(fmt.Sprintf("%s/%s_%d", out_path, player_name, count-1))
		if meta_id == skin.SkinID {
			return // skin same as before
		}
	}

	skin_player_counts[player_name]++
	count++
	write_skin(out_path, fmt.Sprintf("%s_%d", player_name, count), skin)
	popup_skin_saved(conn, player_name)
}

func process_packet_skins(conn *minecraft.Conn, out_path string, pk packet.Packet, filter string, only_if_geom bool) {
	switch _pk := pk.(type) {
	case *packet.PlayerSkin:
		player_name := skin_players[_pk.UUID.String()]
		if player_name == "" {
			player_name = _pk.UUID.String()
		}
		if !strings.HasPrefix(player_name, filter) {
			return
		}
		if only_if_geom && len(_pk.Skin.SkinGeometry) == 0 {
			return
		}

		save_player_skin(conn, out_path, player_name, &_pk.Skin)
	case *packet.PlayerList:
		if _pk.ActionType == 1 { // remove
			return
		}
		for _, player := range _pk.Entries {
			player_name := utils.CleanupName(player.Username)
			if player_name == "" {
				player_name = player.UUID.String()
			}
			if !strings.HasPrefix(player_name, filter) {
				return
			}
			if only_if_geom && len(player.Skin.SkinGeometry) == 0 {
				return
			}
			skin_players[player.UUID.String()] = player_name
			save_player_skin(conn, out_path, player_name, &player.Skin)
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
	address, hostname, err := utils.ServerInput(ctx, c.server_address)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		return 1
	}

	serverConn, err := utils.ConnectServer(ctx, address, nil, false, nil)
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

	logrus.Info("Connected")
	logrus.Info("Press ctrl+c to exit")

	os.MkdirAll(out_path, 0o755)

	for {
		pk, err := serverConn.ReadPacket()
		if err != nil {
			return 1
		}
		process_packet_skins(nil, out_path, pk, c.filter, false)
	}
}

func init() {
	utils.RegisterCommand(&SkinCMD{})
}
