package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"strings"
	"syscall"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
)

const TOKEN_FILE = "token.json"

var G_src oauth2.TokenSource
var G_xbl_token *auth.XBLToken
var G_debug bool
var G_help bool
var G_exit func() = func() { os.Exit(0) }

var pool = packet.NewPool()

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
	dir := "<-C"
	if strings.HasPrefix(strings.Split(src.String(), ":")[1], "19132") {
		dir = "S->"
	}
	switch pk := pk.(type) {
	case *packet.UpdateBlock:
		return
	case *packet.MoveActorAbsolute:
		return
	case *packet.SetActorMotion:
		return
	case *packet.SetTime:
		return
	case *packet.RemoveActor:
		return
	case *packet.AddActor:
		return
	case *packet.UpdateAttributes:
		return
	case *packet.Interact:
		return
	case *packet.LevelEvent:
		return
	case *packet.SetActorData:
		return
	case *packet.MoveActorDelta:
		return
	case *packet.MovePlayer:
		return
	case *packet.BlockActorData:
		return
	case *packet.ResourcePacksInfo:
		/*
			for _, pack := range pk.TexturePacks {
				fmt.Printf("%s %s\n", pack.ContentIdentity, pack.ContentKey)
			}
			fmt.Printf("writing keys file")
			var keys map[string]string = make(map[string]string)
			for _, pack := range pk.TexturePacks {
				keys[pack.ContentIdentity] = pack.ContentKey
			}
			dump_keys(keys)
		*/
		return
	}

	fmt.Printf("P: %s 0x%x, %s\n", dir, pk.ID(), reflect.TypeOf(pk))
}

type CMD struct {
	Name string
	Desc string
	Main func(context.Context, []string) error
}

var cmds map[string]CMD = make(map[string]CMD)

func register_command(name, desc string, main_func func(context.Context, []string) error) {
	cmds[name] = CMD{
		Name: name,
		Desc: desc,
		Main: main_func,
	}
}

func main() {
	flag.BoolVar(&G_debug, "debug", false, "debug mode")
	flag.BoolVar(&G_help, "help", false, "show help")

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Printf("\nExiting\n")
		cancel()
		G_exit()
	}()

	// authenticate
	token := get_token()
	G_src = auth.RefreshTokenSource(&token)
	{
		_token, err := G_src.Token()
		if err != nil {
			panic(err)
		}
		G_xbl_token, err = auth.RequestXBLToken(ctx, _token, "https://pocket.realms.minecraft.net/")
		if err != nil {
			panic(err)
		}
	}

	if len(os.Args) < 2 {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Println("Available commands:")
			for name, cmd := range cmds {
				fmt.Printf("\t%s\t%s\n", name, cmd.Desc)
			}
			fmt.Printf("Use '%s <command>' to run a command\n", os.Args[0])

			fmt.Printf("Input Command: ")
			reader := bufio.NewReader(os.Stdin)
			target, _ := reader.ReadString('\n')
			r, _ := regexp.Compile(`[\n\r]`)
			target = string(r.ReplaceAll([]byte(target), []byte("")))
			os.Args = append(os.Args, target)
		}
	}

	cmd := cmds[os.Args[1]]
	if cmd.Main == nil {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
	if err := cmd.Main(ctx, os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func token_main(ctx context.Context, args []string) error {
	fmt.Printf("%s\n", G_xbl_token.AuthorizationToken.Token)
	return nil
}

func init() {
	register_command("realms-token", "get xbl3.0 token for pocket.realms.minecraft.net", token_main)
}
