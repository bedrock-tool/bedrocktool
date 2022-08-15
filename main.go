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
	"syscall"

	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2"
)

const TOKEN_FILE = "token.json"

var G_src oauth2.TokenSource
var G_xbl_token *auth.XBLToken
var G_debug bool
var G_exit []func() = []func(){}

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
	"*packet.LevelEvent",
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
		fmt.Printf("Disconnect: %s", pk.Message)
	}
	fmt.Printf("%s 0x%x, %s\n", dir, pk.ID(), pk_name)
}

func exit() {
	fmt.Printf("\nExiting\n")
	for i := len(G_exit) - 1; i >= 0; i-- { // go through cleanup functions reversed
		G_exit[i]()
	}
	os.Exit(0)
}

var valid_cmds = make(map[string]string, 0)

func register_command(sub subcommands.Command) {
	subcommands.Register(sub, "")
	valid_cmds[sub.Name()] = sub.Synopsis()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	flag.BoolVar(&G_debug, "debug", false, "debug mode")
	enable_dns := flag.Bool("dns", false, "enable dns server for consoles")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.ImportantFlag("debug")
	subcommands.ImportantFlag("dns")
	subcommands.HelpCommand()

	{ // interactive input
		if len(os.Args) < 2 {
			select {
			case <-ctx.Done():
				return
			default:
				fmt.Println("Available commands:")
				for name, desc := range valid_cmds {
					fmt.Printf("\t%s\t%s\n", name, desc)
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
	}

	flag.Parse()

	if *enable_dns {
		init_dns()
	}

	// exit cleanup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
		exit()
	}()

	{ // authenticate
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
	}

	ret := subcommands.Execute(ctx)
	exit()
	os.Exit(int(ret))
}
