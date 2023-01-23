package utils

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path"
	"regexp"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"

	//"github.com/sandertv/gophertunnel/minecraft/gatherings"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var (
	G_debug         bool
	G_preload_packs bool
	G_interactive   bool
)

var name_regexp = regexp.MustCompile(`\||(?:§.?)`)

// cleans name so it can be used as a filename
func CleanupName(name string) string {
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

// connections

func ConnectServer(ctx context.Context, address string, ClientData *login.ClientData, want_packs bool, packetFunc PacketFunc) (serverConn *minecraft.Conn, err error) {
	packet_func := func(header packet.Header, payload []byte, src, dst net.Addr) {
		if G_debug {
			PacketLogger(header, payload, src, dst)
		}
		if packetFunc != nil {
			packetFunc(header, payload, src, dst)
		}
	}

	cd := login.ClientData{}
	if ClientData != nil {
		cd = *ClientData
	}

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": address}))
	serverConn, err = minecraft.Dialer{
		TokenSource: GetTokenSource(),
		ClientData:  cd,
		PacketFunc:  packet_func,
		DownloadResourcePack: func(id uuid.UUID, version string) bool {
			return want_packs
		},
	}.DialContext(ctx, "raknet", address)
	if err != nil {
		return nil, err
	}

	logrus.Debug(locale.Loc("connected", nil))
	Client_addr = serverConn.LocalAddr()
	return serverConn, nil
}

func spawn_conn(ctx context.Context, clientConn *minecraft.Conn, serverConn *minecraft.Conn) error {
	errs := make(chan error, 2)
	go func() {
		errs <- clientConn.StartGame(serverConn.GameData())
	}()
	go func() {
		errs <- serverConn.DoSpawn()
	}()

	// wait for both to finish
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				return errors.New(locale.Loc("failed_start_game", locale.Strmap{"Err": err}))
			}
		case <-ctx.Done():
			return errors.New(locale.Loc("connection_cancelled", nil))
		}
	}
	return nil
}

// get longest line length
func max_len(lines []string) int {
	o := 0
	for _, line := range lines {
		if o < len(line) {
			o = len(line)
		}
	}
	return o
}

// make text centered
func MarginLines(lines []string) string {
	ret := ""
	max := max_len(lines)
	for _, line := range lines {
		if len(line) != max {
			ret += strings.Repeat(" ", max/2-len(line)/4)
		}
		ret += line + "\n"
	}
	return ret
}

// SplitExt splits path to filename and extension
func SplitExt(filename string) (name, ext string) {
	name, ext = path.Base(filename), path.Ext(filename)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return
}

func Clamp(a, b int) int {
	if a > b {
		return b
	}
	if a < 0 {
		return 0
	}
	return a
}
