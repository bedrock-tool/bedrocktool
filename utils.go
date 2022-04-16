package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
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

func server_input(ctx context.Context, server string) (string, string) {
	if server == "" {
		fmt.Printf("Enter Server: ")
		reader := bufio.NewReader(os.Stdin)
		server, _ = reader.ReadString('\n')
		r, _ := regexp.Compile(`[\n\r]`)
		server = string(r.ReplaceAll([]byte(server), []byte("")))
	}
	if len(strings.Split(server, ":")) == 1 {
		server += ":19132"
	}
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid server: %s\n", err)
		os.Exit(1)
	}
	return host, server
}

func connect_server(ctx context.Context, server string) (hostname string, conn *minecraft.Conn, err error) {
	if strings.HasPrefix(server, "realm:") {
		hostname, server, err = get_realm(strings.Split(server, ":")[1])
		if err != nil {
			return "", nil, err
		}
	} else {
		hostname, server = server_input(ctx, server)
	}

	var packet_func func(header packet.Header, payload []byte, src, dst net.Addr) = nil
	if G_debug {
		packet_func = PacketLogger
	}

	fmt.Printf("Connecting to %s\n", server)
	conn, err = minecraft.Dialer{
		TokenSource: G_src,
		PacketFunc:  packet_func,
	}.DialContext(ctx, "raknet", server)
	if err != nil {
		return "", nil, err
	}
	return hostname, conn, nil
}
