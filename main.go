package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"golang.org/x/oauth2"
)

const TOKEN_FILE = "token.json"
const KEYS_FILE = "keys.db"

func get_token() oauth2.Token {
	var token oauth2.Token
	var err error

	if _, err = os.Stat(TOKEN_FILE); err == nil {
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
		token = *_token

		buf, err := json.Marshal(token)
		if err != nil {
			panic(err)
		}
		os.WriteFile(TOKEN_FILE, buf, 0666)
	}
	return token
}

func dump_keys(keys map[string]string) {
	f, err := os.OpenFile(KEYS_FILE, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for uuid, key := range keys {
		f.WriteString(uuid + "=" + key + "\n")
	}
}

func download_pack(pack *resource.Pack) ([]byte, error) {
	buf := make([]byte, pack.Len())
	off := 0
	for {
		n, err := pack.ReadAt(buf[off:], int64(off))
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		off += n
	}
	return buf, nil
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

func main() {
	// get target server ip
	var target string
	var save_encrypted bool
	var debug bool
	flag.StringVar(&target, "target", "", "[serverip:port]")
	flag.BoolVar(&save_encrypted, "save_encrypted", false, "save encrypted zips")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()
	if target == "" {
		fmt.Printf("Enter Server: ")
		reader := bufio.NewReader(os.Stdin)
		target, _ = reader.ReadString('\n')
		target = strings.Replace(target, "\n", "", -1)
		target = strings.Replace(target, "\r", "", -1)
	}
	if len(strings.Split(target, ":")) == 1 { // add default port if not set
		target += ":19132"
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	var serverConn *minecraft.Conn

	go func() {
		<-sigs
		if serverConn != nil {
			serverConn.Close()
			serverConn = nil
		}
		cancel()
		os.Exit(0)
	}()

	// authenticate
	token := get_token()
	src := auth.RefreshTokenSource(&token)

	var packet_func func(header packet.Header, payload []byte, src, dst net.Addr)
	if debug {
		packet_func = PacketLogger
	} else {
		packet_func = nil
	}

	// connect
	fmt.Printf("Connecting to %s\n", target)
	serverConn, err := minecraft.Dialer{
		TokenSource: src,
		PacketFunc:  packet_func,
	}.DialContext(ctx, "raknet", target)
	if err != nil {
		panic(err)
	}

	defer func() {
		if serverConn != nil {
			serverConn.Close()
			serverConn = nil
		}
	}()

	if err := serverConn.DoSpawnContext(ctx); err != nil {
		panic(err)
	}

	println("Connected")
	println("ripping Resource Packs")

	// dump keys, download and decrypt the packs
	keys := make(map[string]string)
	for _, pack := range serverConn.ResourcePacks() {
		keys[pack.UUID()] = pack.ContentKey()
		fmt.Printf("ResourcePack(Id: %s Key: %s | Name: %s Version: %s)\n", pack.UUID(), keys[pack.UUID()], pack.Name(), pack.Version())

		fmt.Printf("Downloading...\n")
		pack_data, err := download_pack(pack)
		if err != nil {
			panic(err)
		}
		if save_encrypted {
			os.WriteFile(pack.Name()+".ENCRYPTED.zip", pack_data, 0666)
		}
		fmt.Printf("Decrypting...\n")
		if err := decrypt_pack(pack_data, pack.Name()+".mcpack", keys[pack.UUID()]); err != nil {
			panic(fmt.Errorf("failed to decrypt %s: %s", pack.Name(), err))
		}
	}

	if len(keys) > 0 {
		fmt.Printf("Writing keys to %s\n", KEYS_FILE)
		dump_keys(keys)
	} else {
		fmt.Printf("No Resourcepack sent\n")
	}
	fmt.Printf("Done!\n")
}
