package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
)

func send_popup(conn *minecraft.Conn, text string) {
	conn.WritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text,
	})
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
		token, err := auth.RequestLiveToken()
		if err != nil {
			panic(err)
		}

		buf, err := json.Marshal(token)
		if err != nil {
			panic(err)
		}
		os.WriteFile(TOKEN_FILE, buf, 0666)
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
		name, address, err = get_realm(strings.Split(server, ":")[1])
		if err != nil {
			return "", "", err
		}
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
		TokenSource: G_src,
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
	/*
		if strings.HasSuffix(server_address, ".pcap") {
			return create_replay_connection(server_address)
		}
	*/

	_status := minecraft.NewStatusProvider("Server")
	listener, err := minecraft.ListenConfig{
		StatusProvider: _status,
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
		listener.Disconnect(clientConn, "Closing")
		clientConn.Close()
		listener.Close()
	})

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
