package subcommands

import (
	"context"
	"flag"
	"fmt"
	"net"
	"sync"

	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sandertv/go-raknet"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

type BlindProxyCMD struct {
	ServerAddress string
}

func (*BlindProxyCMD) Name() string     { return "blind-proxy" }
func (*BlindProxyCMD) Synopsis() string { return "raknet proxy" }
func (c *BlindProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "server address")
}

func packet_forward(src, dst *raknet.Conn) error {
	for {
		data, err := src.ReadPacket()
		if err != nil {
			return err
		}
		_, err = dst.Write(data)
		if err != nil {
			return err
		}
	}
}

func (c *BlindProxyCMD) Execute(ctx context.Context, ui ui.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	listener, err := raknet.Listen("0.0.0.0:19132")
	if err != nil {
		return err
	}
	defer listener.Close()
	logrus.Info("Listening on 0.0.0.0:19132")

	listener.PongData([]byte(fmt.Sprintf("MCPE;%v;%v;%v;%v;%v;%v;Gophertunnel;%v;%v;%v;%v;",
		"Proxy For "+hostname, protocol.CurrentProtocol, protocol.CurrentVersion, 0, 1,
		listener.ID(), "Creative", 1, listener.Addr().(*net.UDPAddr).Port, listener.Addr().(*net.UDPAddr).Port,
	)))

	clientConn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	logrus.Info("Client Connected")

	serverConn, err := raknet.DialContext(ctx, address)
	if err != nil {
		return err
	}
	defer serverConn.Close()
	logrus.Info("Server Connected")

	logrus.Info("Forwarding Packets")
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_err := packet_forward(clientConn.(*raknet.Conn), serverConn)
		if _err != nil {
			err = _err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_err := packet_forward(serverConn, clientConn.(*raknet.Conn))
		if _err != nil {
			err = _err
		}
	}()

	wg.Wait()
	if err != nil {
		return err
	}

	return nil
}

func init() {
	commands.RegisterCommand(&BlindProxyCMD{})
}
