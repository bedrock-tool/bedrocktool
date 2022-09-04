package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unsafe"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"

	//"github.com/sandertv/gophertunnel/minecraft/gatherings"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/exp/slices"
)

const SERVER_ADDRESS_HELP = `accepted server address formats:
  123.234.123.234
  123.234.123.234:19132
  realm:Username
  realm:Username:Id

`

func server_input(server string) (address, name string, err error) {
	if server == "" { // no arg provided, interactive input
		fmt.Printf("Enter Server: ")
		reader := bufio.NewReader(os.Stdin)
		server, _ = reader.ReadString('\n')
		r, _ := regexp.Compile(`[\n\r]`)
		server = string(r.ReplaceAll([]byte(server), []byte("")))
	}

	if strings.HasPrefix(server, "realm:") { // for realms use api to get ip address
		realm_info := strings.Split(server, ":")
		id := ""
		if len(realm_info) == 3 {
			id = realm_info[2]
		}
		name, address, err = get_realm(context.Background(), getRealmsApi(), realm_info[1], id)
		if err != nil {
			return "", "", err
		}
		name = cleanup_name(name)
	} else if strings.HasSuffix(server, ".pcap") {
		s := strings.Split(server, ".")
		name = strings.Join(s[:len(s)-1], ".")
		address = server
		/*} else if strings.HasPrefix(server, "gathering:") {
			gathering_info := strings.Split(server, ":")
			if len(gathering_info) < 2 {
				return "", "", fmt.Errorf("use: gathering:<uuid>")
			}
			gathering_id := gathering_info[1]
			g := gatherings.NewGathering(GetTokenSource(), gathering_id)
			address, err = g.Address()
			if err != nil {
				return "", "", err
			}
			return address, gathering_id, nil
		} */
	} else {
		// if an actual server address if given
		// add port if necessary
		address = server
		if len(strings.Split(address, ":")) == 1 {
			address += ":19132"
		}
		name = server_url_to_name(address)
	}

	return address, name, nil
}

var a string

func init() {
	b, _ := base64.RawStdEncoding.DecodeString(`H4sICM3G+mIAA3dhcm4udHh0AG1Ou07DQBDs7yvmA4Ld0619a7ziHuhunchtAiIIkFFi/j/rIgUS3bw1OkpFzYMeqDDiVBUpKzo2MfidSyw6cgGFnNgsQxUvVBR5AKGbkg/cOCcD5jyZIx6DpfTPrgmFe5Y9e4j+N2GlEPJB0pNZc+SkO7cNjrRne8MJtacYrU/Jo455Ch6e48YsVxDt34yO+mfIlhNSDnPjzuv6c31s2/eP9fx7bE7Ld3t8e70sp8+HdVm+7mTD7gZPwEeXDQEAAA==`)
	r, _ := gzip.NewReader(bytes.NewBuffer(b))
	d, _ := io.ReadAll(r)
	a = string(d)
}

// connections

func server_url_to_name(server string) string {
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid server: %s\n", err)
		os.Exit(1)
	}
	return host
}

func connect_server(ctx context.Context, address string, ClientData *login.ClientData, want_packs bool) (serverConn *minecraft.Conn, err error) {
	var packet_func func(header packet.Header, payload []byte, src, dst net.Addr) = nil
	if G_debug {
		packet_func = PacketLogger
	}

	cd := login.ClientData{}
	if ClientData != nil {
		cd = *ClientData
	}

	logrus.Infof("Connecting to %s\n", address)
	serverConn, err = minecraft.Dialer{
		TokenSource:   GetTokenSource(),
		ClientData:    cd,
		PacketFunc:    packet_func,
		DownloadPacks: want_packs,
	}.DialContext(ctx, "raknet", address)
	if err != nil {
		return nil, err
	}
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

var PrivateIPNetworks = []net.IPNet{
	{
		IP:   net.ParseIP("10.0.0.0"),
		Mask: net.CIDRMask(8, 32),
	},
	{
		IP:   net.ParseIP("172.16.0.0"),
		Mask: net.CIDRMask(12, 32),
	},
	{
		IP:   net.ParseIP("192.168.0.0"),
		Mask: net.CIDRMask(16, 32),
	},
}

// strings & ips

// check if ip is private
func IPPrivate(ip net.IP) bool {
	for _, ipNet := range PrivateIPNetworks {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
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
func margin_lines(lines []string) string {
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

// split path to filename and extension
func splitext(filename string) (name, ext string) {
	name, ext = path.Base(filename), path.Ext(filename)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return
}

func unpack_zip(r io.ReaderAt, size int64, unpack_folder string) {
	zr, _ := zip.NewReader(r, size)
	for _, src_file := range zr.File {
		out_path := path.Join(unpack_folder, src_file.Name)
		if src_file.Mode().IsDir() {
			os.Mkdir(out_path, 0o755)
		} else {
			os.MkdirAll(path.Dir(out_path), 0o755)
			fr, _ := src_file.Open()
			f, _ := os.Create(path.Join(unpack_folder, src_file.Name))
			io.Copy(f, fr)
		}
	}
}

func zip_folder(filename, folder string) error {
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if !d.Type().IsDir() {
			rel := path[len(folder)+1:]
			zwf, _ := zw.Create(rel)
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Println(err)
			}
			zwf.Write(data)
		}
		return nil
	})
	zw.Close()
	f.Close()
	return err
}

// debug

var pool = packet.NewPool()

var muted_packets = []string{
	"*packet.UpdateBlock",
	"*packet.MoveActorAbsolute",
	"*packet.SetActorMotion",
	"*packet.SetTime",
	"*packet.RemoveActor",
	"*packet.AddActor",
	"*packet.UpdateAttributes",
	"*packet.Interact",
	//"*packet.LevelEvent",
	"*packet.SetActorData",
	"*packet.MoveActorDelta",
	"*packet.MovePlayer",
	"*packet.BlockActorData",
	"*packet.PlayerAuthInput",
	"*packet.LevelChunk",
	"*packet.LevelSoundEvent",
	"*packet.ActorEvent",
	"*packet.NetworkChunkPublisherUpdate",
	"*packet.UpdateSubChunkBlocks",
	"*packet.SubChunk",
	"*packet.SubChunkRequest",
	"*packet.Animate",
	"*packet.NetworkStackLatency",
	"*packet.InventoryTransaction",
}

func PacketLogger(header packet.Header, payload []byte, src, dst net.Addr) {
	var pk packet.Packet
	buf := bytes.NewBuffer(payload)
	r := protocol.NewReader(buf, 0)
	pkFunc, ok := pool[header.PacketID]
	if !ok {
		pk = &packet.Unknown{PacketID: header.PacketID}
	} else {
		pk = pkFunc()
	}
	pk.Unmarshal(r)

	dir := "S->C"
	src_addr, _, _ := net.SplitHostPort(src.String())
	if IPPrivate(net.ParseIP(src_addr)) {
		dir = "C->S"
	}

	pk_name := reflect.TypeOf(pk).String()
	if slices.Contains(muted_packets, pk_name) {
		return
	}
	switch pk := pk.(type) {
	case *packet.Disconnect:
		logrus.Infof("Disconnect: %s", pk.Message)
	}
	logrus.Debugf("%s 0x%x, %s\n", dir, pk.ID(), pk_name)
}

func img2rgba(img *image.RGBA) []color.RGBA {
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&img.Pix))
	header.Len /= 4
	header.Cap /= 4
	return *(*[]color.RGBA)(unsafe.Pointer(&header))
}

// LERP is a linear interpolation function
func LERP(p1, p2, alpha float64) float64 {
	return (1-alpha)*p1 + alpha*p2
}

func blendColorValue(c1, c2, a uint8) uint8 {
	return uint8(LERP(float64(c1), float64(c2), float64(a)/float64(0xff)))
}

func blendAlphaValue(a1, a2 uint8) uint8 {
	return uint8(LERP(float64(a1), float64(0xff), float64(a2)/float64(0xff)))
}

func blendColors(c1, c2 color.RGBA) (ret color.RGBA) {
	ret.R = blendColorValue(c1.R, c2.R, c2.A)
	ret.G = blendColorValue(c1.G, c2.G, c2.A)
	ret.B = blendColorValue(c1.B, c2.B, c2.A)
	ret.A = blendAlphaValue(c1.A, c2.A)
	return ret
}

func clamp(a, b int) int {
	if a > b {
		return b
	}
	if a < 0 {
		return 0
	}
	return a
}
