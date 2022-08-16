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

var G_debug bool
var G_preload_packs bool
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

var G_token_src oauth2.TokenSource

func GetTokenSource() oauth2.TokenSource {
	if G_token_src != nil {
		return G_token_src
	}
	token := get_token()
	G_token_src = auth.RefreshTokenSource(&token)
	new_token, err := G_token_src.Token()
	if err != nil {
		panic(err)
	}
	if !token.Valid() {
		fmt.Println("Refreshed token")
		write_token(new_token)
	}

	return G_token_src
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	flag.BoolVar(&G_debug, "debug", false, "debug mode")
	flag.BoolVar(&G_preload_packs, "preload", false, "preload resourcepacks for proxy")
	enable_dns := flag.Bool("dns", false, "enable dns server for consoles")
	println(a)
	println("")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.ImportantFlag("debug")
	subcommands.ImportantFlag("dns")
	subcommands.ImportantFlag("preload")
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

	ret := subcommands.Execute(ctx)
	exit()
	os.Exit(int(ret))
}

type TransCMD struct{}

func (*TransCMD) Name() string     { return "trans" }
func (*TransCMD) Synopsis() string { return "" }

func (c *TransCMD) SetFlags(f *flag.FlagSet) {}
func (c *TransCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *TransCMD) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	const (
		BLACK_FG = "\033[30m"
		BOLD     = "\033[1m"
		BLUE     = "\033[46m"
		PINK     = "\033[45m"
		WHITE    = "\033[47m"
		RESET    = "\033[0m"
	)
	fmt.Println(BLACK_FG + BOLD + BLUE + " Trans " + PINK + " Rights " + WHITE + " Are " + PINK + " Human " + BLUE + " Rights " + RESET)
	return 0
}
func init() {
	register_command(&TransCMD{})
}
