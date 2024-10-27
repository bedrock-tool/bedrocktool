package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type Session struct {
	Server    minecraft.IConn
	Client    minecraft.IConn
	listener  *minecraft.Listener
	Player    Player
	rpHandler *rpHandler
	blobCache *Blobcache

	packetLogger       *packetLogger
	packetLoggerClient *packetLogger

	isReplay         bool
	expectDisconnect bool
	dimensionData    *packet.DimensionData
	clientConnecting chan struct{}
	haveClientData   chan struct{}
	clientData       login.ClientData
	clientAddr       net.Addr
	spawned          bool
	disconnectReason string
	commands         map[string]ingameCommand

	// from proxy
	withClient    bool
	extraDebug    bool
	addedPacks    []resource.Pack
	listenAddress string
	handlers      Handlers
	OnHitBlobs    func(hitBlobs []protocol.CacheBlob)
}

func NewSession() *Session {
	return &Session{
		clientConnecting: make(chan struct{}),
		haveClientData:   make(chan struct{}),
		disconnectReason: "Connection Lost",
		commands:         make(map[string]ingameCommand),
	}
}

// AddCommand adds a command to the command handler
func (s *Session) AddCommand(exec func([]string) bool, cmd protocol.Command) {
	cmd.AliasesOffset = 0xffffffff
	s.commands[cmd.Name] = ingameCommand{exec, cmd}
}

// ClientWritePacket sends a packet to the client, nop if no client connected
func (s *Session) ClientWritePacket(pk packet.Packet) error {
	if s.Client == nil {
		return nil
	}
	return s.Client.WritePacket(pk)
}

// SendMessage sends a chat message to the client
func (s *Session) SendMessage(text string) {
	_ = s.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypeSystem,
		Message:  "§8[§bBedrocktool§8]§r " + text,
	})
}

// SendPopup sends a toolbar popup to the client
func (s *Session) SendPopup(text string) {
	_ = s.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text,
	})
}

// Disconnect disconnects the client
func (s *Session) DisconnectClient() {
	if s.Client == nil {
		return
	}
	_ = s.Client.Close()
}

// Disconnect disconnects from the server
func (s *Session) DisconnectServer() {
	if s.Server == nil {
		return
	}
	s.expectDisconnect = true
	_ = s.Server.Close()
}

// Disconnect disconnects both the client and server
func (s *Session) Disconnect() {
	s.DisconnectClient()
	s.DisconnectServer()
}

func (s *Session) Run(ctx context.Context, connect *utils.ConnectInfo) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	listenIP, _listenPort, _ := net.SplitHostPort(s.listenAddress)
	listenPort, _ := strconv.Atoi(_listenPort)

	messages.Router.Handle(&messages.Message{
		Source: "proxy",
		Target: "ui",
		Data: messages.ConnectStateUpdate{
			State:      messages.ConnectStateBegin,
			ListenIP:   listenIP,
			ListenPort: listenPort,
		},
	})

	onResourcePacksInfo := func() {
		messages.Router.Handle(&messages.Message{
			Source: "proxy",
			Target: "ui",
			Data: messages.ConnectStateUpdate{
				State: messages.ConnectStateReceivingResources,
			},
		})
	}

	rpHandler := newRpHandler(ctx, s.addedPacks)
	s.rpHandler = rpHandler
	rpHandler.OnResourcePacksInfoCB = onResourcePacksInfo
	rpHandler.OnFinishedPack = s.handlers.OnFinishedPack
	rpHandler.filterDownloadResourcePacks = s.handlers.FilterResourcePack

	var err error
	s.blobCache, err = NewBlobCache(s)
	if err != nil {
		return err
	}
	s.blobCache.OnHitBlobs = s.OnHitBlobs
	s.blobCache.processPacket = s.processBlobPacket
	defer s.blobCache.Close()

	if utils.Options.Debug || utils.Options.ExtraDebug {
		s.packetLogger, err = NewPacketLogger(utils.Options.ExtraDebug, false)
		if err != nil {
			return err
		}
		defer s.packetLogger.Close()

		s.packetLoggerClient, err = NewPacketLogger(utils.Options.ExtraDebug, true)
		if err != nil {
			return err
		}
		defer s.packetLogger.Close()
	}

	if connect.Replay != "" {
		replay, err := CreateReplayConnector(ctx, connect.Replay, s.packetFunc, rpHandler)
		if err != nil {
			return err
		}
		s.Server = replay
		s.isReplay = true
		err = replay.ReadUntilLogin()
		if err != nil {
			return err
		}
	} else {
		var wg sync.WaitGroup
		if s.withClient {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := s.connectClient(ctx, connect)
				if err != nil {
					cancel(err)
					return
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.connectServer(ctx, connect)
			if err != nil {
				cancel(err)
				return
			}
		}()
		wg.Wait()
	}

	if s.Server != nil {
		defer s.Server.Close()
	}

	if s.listener != nil {
		defer func() {
			if s.Client != nil {
				_ = s.listener.Disconnect(s.Client.(*minecraft.Conn), s.disconnectReason)
			}
			_ = s.listener.Close()
		}()
	}

	if ctx.Err() != nil {
		err := context.Cause(ctx)
		if errors.Is(err, errCancelConnect) {
			err = nil
		}
		if err != nil {
			s.disconnectReason = err.Error()
		} else {
			s.disconnectReason = "Disconnect"
		}

		if s.expectDisconnect {
			return nil
		}
		return err
	}

	disconnect, err := s.handlers.OnServerConnect()
	if disconnect {
		err = errCancelConnect
	}
	if err != nil {
		cancel(err)
		return err
	}

	gameData := s.Server.GameData()
	s.handlers.GameDataModifier(&gameData)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.Server.DoSpawnContext(ctx)
		if err != nil {
			cancel(err)
			return
		}
	}()

	if s.Client != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.dimensionData != nil {
				s.Client.WritePacket(s.dimensionData)
			}
			err := s.Client.StartGameContext(ctx, gameData)
			if err != nil {
				cancel(err)
				return
			}
		}()
	}

	wg.Wait()

	err = context.Cause(ctx)
	if err != nil {
		s.disconnectReason = err.Error()
		if s.expectDisconnect {
			return nil
		}
		return err
	}

	if s.handlers.OnConnect() {
		logrus.Info("Disconnecting")
		return nil
	}

	messages.Router.Handle(&messages.Message{
		Source: "proxy",
		Target: "ui",
		Data: messages.ConnectStateUpdate{
			State: messages.ConnectStateDone,
		},
	})

	doProxy := func(client bool) {
		defer wg.Done()
		if err := s.proxyLoop(ctx, client); err != nil {
			if !errors.Is(err, context.Canceled) {
				cancel(err)
			}
		}
	}

	// server to client
	wg.Add(1)
	go doProxy(false)

	// client to server
	if s.Client != nil {
		wg.Add(1)
		go doProxy(true)
	}

	wg.Wait()
	err = context.Cause(ctx)
	if !errors.Is(err, &errTransfer{}) {
		if s.Client != nil {
			s.Client.Close()
		}
	}
	if err != nil {
		s.disconnectReason = err.Error()
		return err
	}

	return nil
}

func (s *Session) connectServer(ctx context.Context, connect *utils.ConnectInfo) (err error) {
	if s.withClient {
		select {
		case <-s.clientConnecting:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	messages.Router.Handle(&messages.Message{
		Source: "proxy",
		Target: "ui",
		Data: messages.ConnectStateUpdate{
			State: messages.ConnectStateServerConnecting,
		},
	})

	address, err := connect.Address(ctx)
	if err != nil {
		return err
	}

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": address}))
	d := minecraft.Dialer{
		TokenSource:       utils.Auth,
		ErrorLog:          slog.Default(),
		PacketFunc:        s.packetFunc,
		EnableClientCache: true,
		GetClientData: func() login.ClientData {
			if s.withClient {
				select {
				case <-s.haveClientData:
				case <-ctx.Done():
				}
			}
			return s.clientData
		},
		EarlyConnHandler: func(c *minecraft.Conn) {
			s.Server = c
			s.rpHandler.SetServer(c)
			c.ResourcePackHandler = s.rpHandler
		},
	}
	for retry := 0; retry < 3; retry++ {
		d.ChainKey, d.ChainData, err = utils.Auth.Chain(ctx)
		if err != nil {
			continue
		}
		break
	}
	if err != nil {
		return err
	}

	_, err = d.DialContext(ctx, "raknet", address, 20*time.Second)
	if err != nil {
		if s.expectDisconnect {
			return nil
		}
		return err
	}

	messages.Router.Handle(&messages.Message{
		Source: "proxy",
		Target: "ui",
		Data: messages.ConnectStateUpdate{
			State: messages.ConnectStateEstablished,
		},
	})
	logrus.Debug(locale.Loc("connected", nil))
	return nil
}

func (s *Session) connectClient(ctx context.Context, connect *utils.ConnectInfo) (err error) {
	s.listener, err = minecraft.ListenConfig{
		AuthenticationDisabled: true,
		StatusProvider:         minecraft.NewStatusProvider(fmt.Sprintf("%s Proxy", connect.Name()), "Bedrocktool"),
		PacketFunc: func(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
			pk, ok := DecodePacket(header, payload, s.Client.ShieldID())
			if !ok {
				return
			}
			drop, err := s.blobPacketsFromClient(pk)
			if err != nil {
				logrus.Error(err)
				return
			}
			_ = drop
			if s.packetLoggerClient != nil {
				if src == s.listener.Addr() {
					err = s.packetLoggerClient.PacketSend(pk, timeReceived)
				} else {
					err = s.packetLoggerClient.PacketReceive(pk, timeReceived)
				}
			}
			if err != nil {
				logrus.Error(err)
				return
			}
		},
		OnClientData: func(c *minecraft.Conn) {
			s.clientData = c.ClientData()
			close(s.haveClientData)
		},
		EarlyConnHandler: func(c *minecraft.Conn) {
			if s.Client != nil {
				s.listener.Disconnect(c, "You are Already connected!")
				return
			}
			s.Client = c
			s.rpHandler.SetClient(c)
			c.ResourcePackHandler = s.rpHandler
			close(s.clientConnecting)
		},
	}.Listen("raknet", s.listenAddress)
	if err != nil {
		return err
	}

	messages.Router.Handle(&messages.Message{
		Source: "proxy",
		Target: "ui",
		Data: messages.ConnectStateUpdate{
			State: messages.ConnectStateListening,
		},
	})
	logrus.Infof(locale.Loc("listening_on", locale.Strmap{"Address": s.listener.Addr()}))
	logrus.Infof(locale.Loc("help_connect", nil))

	err = utils.Netisolation()
	if err != nil {
		logrus.Warnf("Failed to Enable Loopback for Minecraft: %s", err)
	}

	var accepted = false
	go func() {
		<-ctx.Done()
		if !accepted {
			_ = s.listener.Close()
		}
	}()

	_, err = s.listener.Accept()
	if err != nil {
		return err
	}
	accepted = true
	logrus.Info("Client Connected")
	return nil
}

func (s *Session) processBlobPacket(pk packet.Packet, timeReceived time.Time, preLogin bool) error {
	_, err := s.handlers.PacketCallback(pk, false, timeReceived, preLogin)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) proxyLoop(ctx context.Context, toServer bool) (err error) {
	var c1, c2 minecraft.IConn
	if toServer {
		c1 = s.Client
		c2 = s.Server
	} else {
		c1 = s.Server
		c2 = s.Client
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		pk, timeReceived, err := c1.ReadPacketWithTime()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				err = nil
			}
			return err
		}

		pkName := reflect.TypeOf(pk).String()

		var forward = pk
		var process = true

		if !toServer {
			forward, process, err = s.blobPacketsFromServer(pk, timeReceived, false)
			if err != nil {
				return err
			}
			if pk.ID() == packet.IDCompressedBiomeDefinitionList {
				forward = nil
			}
		} else {
			if pk.ID() == packet.IDClientCacheBlobStatus {
				forward = nil
			}
		}

		if process {
			pk, err = s.handlers.PacketCallback(pk, toServer, timeReceived, false)
			if err != nil {
				return err
			}
			if pk == nil {
				logrus.Tracef("Dropped Packet: %s", pkName)
				continue
			}
		}

		if forward == nil {
			continue
		}

		var transfer *packet.Transfer
		switch _pk := pk.(type) {
		case *packet.Transfer:
			transfer = _pk
			if s.Client != nil {
				host, port, err := net.SplitHostPort(s.Client.ClientData().ServerAddress)
				if err != nil {
					return err
				}
				// transfer to self
				_port, _ := strconv.Atoi(port)
				pk = &packet.Transfer{Address: host, Port: uint16(_port)}
			}
		case *packet.InventoryContent:
			if _pk.StorageItem.Stack.NetworkID == -1 {
				_pk.StorageItem.Stack.NetworkID = 0
			}
		case *packet.InventorySlot:
			if _pk.StorageItem.Stack.NetworkID == -1 {
				_pk.StorageItem.Stack.NetworkID = 0
			}
		}

		if pk != nil && c2 != nil {
			if err := c2.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					s.disconnectReason = disconnect.Error()
				}
				if errors.Is(err, net.ErrClosed) {
					err = nil
				}
				return err
			}
		}

		if transfer != nil {
			return &errTransfer{transfer: transfer}
		}
	}
}

func (s *Session) packetFunc(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
	defer func() {
		if err, ok := recover().(error); ok {
			utils.ErrorHandler(err)
		}
	}()

	if header.PacketID == packet.IDRequestNetworkSettings {
		s.clientAddr = src
	}
	if header.PacketID == packet.IDSetLocalPlayerAsInitialised {
		s.spawned = true
	}

	s.handlers.PacketRaw(header, payload, src, dst, timeReceived)

	pk, ok := DecodePacket(header, payload, s.Server.ShieldID())
	if !ok {
		return
	}

	if s.packetLogger != nil {
		if s.IsClient(src) {
			s.packetLogger.PacketSend(pk, timeReceived)
		} else {
			s.packetLogger.PacketReceive(pk, timeReceived)
		}
	}

	if !s.spawned {
		switch pk := pk.(type) {
		case *packet.DimensionData:
			s.dimensionData = pk
		}

		var err error
		toServer := s.IsClient(src)
		pk, err = s.handlers.PacketCallback(pk, toServer, timeReceived, true)
		if err != nil {
			logrus.Error(err)
		}
		if pk == nil {
			return
		}

		if !toServer {
			forward, _, err := s.blobPacketsFromServer(pk, timeReceived, true)
			if err != nil {
				logrus.Error(err)
			}
			_ = forward
		}
	}
}

func (s *Session) IsClient(addr net.Addr) bool {
	return s.clientAddr.String() == addr.String()
}

func (s *Session) blobPacketsFromServer(pk packet.Packet, timeReceived time.Time, preLogin bool) (forward packet.Packet, process bool, err error) {
	forward = pk
	process = true
	switch pk := pk.(type) {
	case *packet.LevelChunk:
		if pk.CacheEnabled {
			process = false
			forward, err = s.blobCache.HandleLevelChunk(pk, timeReceived, preLogin)
		}

	case *packet.SubChunk:
		if pk.CacheEnabled {
			process = false
			forward, err = s.blobCache.HandleSubChunk(pk, timeReceived, preLogin)
		}

	case *packet.ClientCacheMissResponse:
		forward = nil
		process = false
		if !preLogin {
			err = s.blobCache.HandleClientCacheMissResponse(pk, timeReceived, preLogin)
		}
	}
	return
}

func (s *Session) blobPacketsFromClient(pk packet.Packet) (forward bool, err error) {
	switch pk := pk.(type) {
	case *packet.ClientCacheBlobStatus:
		forward = false
		err = s.blobCache.HandleClientCacheBlobStatus(pk)
	default:
		forward = true
	}
	return
}
