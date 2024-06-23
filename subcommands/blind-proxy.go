package subcommands

import (
	"context"
	"flag"
	"fmt"
	"net"
	"sync"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sandertv/go-raknet"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

type BlindProxyCMD struct {
	ServerAddress string
	ListenAddress string
}

func (*BlindProxyCMD) Name() string     { return "blind-proxy" }
func (*BlindProxyCMD) Synopsis() string { return "raknet proxy" }
func (c *BlindProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "server address")
	f.StringVar(&c.ListenAddress, "listen", "", "example :19132 or 127.0.0.1:19132")
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

func (c *BlindProxyCMD) Execute(ctx context.Context) error {
	server, err := utils.ParseServer(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	if c.ListenAddress == "" {
		c.ListenAddress = "0.0.0.0:19132"
	}

	listener, err := raknet.Listen(c.ListenAddress)
	if err != nil {
		return err
	}
	defer listener.Close()
	logrus.Infof("Listening on %s", c.ListenAddress)

	listener.PongData([]byte(fmt.Sprintf("MCPE;%v;%v;%v;%v;%v;%v;Gophertunnel;%v;%v;%v;%v;",
		"Proxy For "+server.Name, protocol.CurrentProtocol, protocol.CurrentVersion, 0, 1,
		listener.ID(), "Creative", 1, listener.Addr().(*net.UDPAddr).Port, listener.Addr().(*net.UDPAddr).Port,
	)))

	clientConn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	logrus.Info("Client Connected")

	serverConn, err := raknet.DialContext(ctx, server.Address+":"+server.Port)
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
