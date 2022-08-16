package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"golang.org/x/oauth2"
)

const SERVER_ADDRESS_HELP = `accepted server address formats:
  123.234.123.234
  123.234.123.234:19132
  realm:Username
  realm:Username:Id

`

func send_popup(conn *minecraft.Conn, text string) {
	conn.WritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text,
	})
}

func write_token(token *oauth2.Token) {
	buf, err := json.Marshal(token)
	if err != nil {
		panic(err)
	}
	os.WriteFile(TOKEN_FILE, buf, 0755)
}

func get_token() oauth2.Token {
	var token oauth2.Token
	if _, err := os.Stat(TOKEN_FILE); err == nil {
		f, err := os.Open(TOKEN_FILE)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&token); err != nil {
			panic(err)
		}
	} else {
		_token, err := auth.RequestLiveToken()
		if err != nil {
			panic(err)
		}
		write_token(_token)
		token = *_token
	}
	return token
}

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
		name, address, err = get_realm(realm_info[1], id)
		if err != nil {
			return "", "", err
		}
	} else if strings.HasSuffix(server, ".pcap") {
		s := strings.Split(server, ".")
		name = strings.Join(s[:len(s)-1], ".")
		address = server
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

func server_url_to_name(server string) string {
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid server: %s\n", err)
		os.Exit(1)
	}
	return host
}

func connect_server(ctx context.Context, address string, ClientData *login.ClientData) (serverConn *minecraft.Conn, err error) {
	var packet_func func(header packet.Header, payload []byte, src, dst net.Addr) = nil
	if G_debug {
		packet_func = PacketLogger
	}

	cd := login.ClientData{}
	if ClientData != nil {
		cd = *ClientData
	}

	fmt.Printf("Connecting to %s\n", address)
	serverConn, err = minecraft.Dialer{
		TokenSource: GetTokenSource(),
		ClientData:  cd,
		PacketFunc:  packet_func,
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

func create_proxy(ctx context.Context, server_address string) (l *minecraft.Listener, clientConn, serverConn *minecraft.Conn, err error) {
	GetTokenSource() // ask for login before listening

	var packs []*resource.Pack
	if G_preload_packs {
		fmt.Println("Preloading resourcepacks")
		serverConn, err = connect_server(ctx, server_address, nil)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to connect to %s: %s", server_address, err)
		}
		serverConn.Close()
		packs = serverConn.ResourcePacks()
		fmt.Printf("%d packs loaded\n", len(packs))
	}

	_status := minecraft.NewStatusProvider("Server")
	listener, err := minecraft.ListenConfig{
		StatusProvider: _status,
		ResourcePacks:  packs,
	}.Listen("raknet", ":19132")
	if err != nil {
		return nil, nil, nil, err
	}
	l = listener

	fmt.Printf("Listening on %s\n", listener.Addr())

	c, err := listener.Accept()
	if err != nil {
		log.Fatal(err)
	}
	clientConn = c.(*minecraft.Conn)

	cd := clientConn.ClientData()
	serverConn, err = connect_server(ctx, server_address, &cd)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to %s: %s", server_address, err)
	}

	if err := spawn_conn(ctx, clientConn, serverConn); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to spawn: %s", err)
	}

	G_exit = append(G_exit, func() {
		serverConn.Close()
		l.Disconnect(clientConn, "Closing")
		clientConn.Close()
		l.Close()
	})

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			clientConn.WritePacket(&packet.Text{
				TextType: packet.TextTypeTip,
				Message:  a + "\n\n\n\n\n\n",
			})
		}
	}()

	return l, clientConn, serverConn, nil
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
