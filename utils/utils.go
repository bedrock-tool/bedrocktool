package utils

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"

	//"github.com/sandertv/gophertunnel/minecraft/gatherings"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const SERVER_ADDRESS_HELP = `accepted server address formats:
  123.234.123.234
  123.234.123.234:19132
  realm:<Realmname>
  realm:<Realmname>:<Id>

`

var (
	G_debug         bool
	G_preload_packs bool
	G_interactive   bool
)

var A string

func init() {
	b, _ := base64.RawStdEncoding.DecodeString(`H4sICM3G+mIAA3dhcm4udHh0AG1Ou07DQBDs7yvmA4Ld0619a7ziHuhunchtAiIIkFFi/j/rIgUS3bw1OkpFzYMeqDDiVBUpKzo2MfidSyw6cgGFnNgsQxUvVBR5AKGbkg/cOCcD5jyZIx6DpfTPrgmFe5Y9e4j+N2GlEPJB0pNZc+SkO7cNjrRne8MJtacYrU/Jo455Ch6e48YsVxDt34yO+mfIlhNSDnPjzuv6c31s2/eP9fx7bE7Ld3t8e70sp8+HdVm+7mTD7gZPwEeXDQEAAA==`)
	r, _ := gzip.NewReader(bytes.NewBuffer(b))
	d, _ := io.ReadAll(r)
	A = string(d)
}

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

	logrus.Infof("Connecting to %s\n", address)
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

	logrus.Debug("Connected.")
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
				return fmt.Errorf("failed to start game: %s", err)
			}
		case <-ctx.Done():
			return fmt.Errorf("connection cancelled")
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
