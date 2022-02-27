package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
)

const TOKEN_FILE = "token.json"

var G_src oauth2.TokenSource
var G_debug bool

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

var pool = packet.NewPool()

func PacketLogger(header packet.Header, payload []byte, src, dst net.Addr) {
	var pk packet.Packet
	buf := bytes.NewBuffer(payload)
	r := protocol.NewReader(buf, 0)
	pkFunc, ok := pool[header.PacketID]
	if !ok {
		pk = &packet.Unknown{PacketID: header.PacketID}
	}
	pk = pkFunc()
	pk.Unmarshal(r)
	dir := "<-C"
	if strings.HasPrefix(strings.Split(src.String(), ":")[1], "19132") {
		dir = "S->"
	}
	fmt.Printf("P: %s 0x%x, %s\n", dir, pk.ID(), reflect.TypeOf(pk))
	switch p := pk.(type) {
	case *packet.ResourcePackDataInfo:
		fmt.Printf("info %s\n", p.UUID)
	}
}

type CMD struct {
	Name string
	Desc string
	Main func([]string) error
}

var cmds map[string]CMD = make(map[string]CMD)

func register_command(name, desc string, main_func func([]string) error) {
	cmds[name] = CMD{
		Name: name,
		Desc: desc,
		Main: main_func,
	}
}

func input_server() string {
	fmt.Printf("Enter Server: ")
	reader := bufio.NewReader(os.Stdin)
	target, _ := reader.ReadString('\n')
	r, _ := regexp.Compile(`[^\n\r]`)
	target = string(r.ReplaceAll([]byte(target), []byte("")))
	return target
}

func main() {
	flag.BoolVar(&G_debug, "debug", false, "debug mode")
	flag.Parse()

	// authenticate
	token := get_token()
	G_src = auth.RefreshTokenSource(&token)

	if len(os.Args) < 2 {
		fmt.Println("Available commands:")
		for name, cmd := range cmds {
			fmt.Printf("\t%s\t%s\n", name, cmd.Desc)
		}
		fmt.Printf("Use '%s <command>' to run a command\n", os.Args[0])
		return
	}

	cmd := cmds[os.Args[1]]
	if cmd.Main == nil {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
	if err := cmd.Main(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
