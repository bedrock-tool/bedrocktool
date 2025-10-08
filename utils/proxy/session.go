package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/bedrock-tool/bedrocktool/utils/proxy/blobcache"
	"github.com/bedrock-tool/bedrocktool/utils/proxy/pcap2"
	"github.com/bedrock-tool/bedrocktool/utils/proxy/resourcepacks"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type Session struct {
	log       *logrus.Entry
	ctx       context.Context
	cancelCtx context.CancelCauseFunc
	commands  map[string]ingameCommand
	settings  ProxySettings

	// from proxy
	addedPacks  []resource.Pack
	withClient  bool
	handlers    Handlers
	connectInfo *connectinfo.ConnectInfo

	packetLogger       *packetLogger
	packetLoggerClient *packetLogger

	listener  *minecraft.Listener
	blobCache *blobcache.Blobcache

	Server minecraft.IConn
	Client minecraft.IConn
	Player Player

	isReplay         bool
	expectDisconnect bool
	dimensionData    *packet.DimensionData
	clientConnecting chan struct{}
	haveClientData   chan struct{}
	clientData       login.ClientData
	clientAddr       net.Addr
	spawned          bool
	disconnectReason string
}

func NewSession(ctx context.Context, settings ProxySettings, addedPacks []resource.Pack, connectInfo *connectinfo.ConnectInfo, withClient bool) *Session {
	sctx, cancelCtx := context.WithCancelCause(ctx)
	return &Session{
		ctx:         sctx,
		cancelCtx:   cancelCtx,
		log:         logrus.StandardLogger().WithContext(ctx),
		settings:    settings,
		addedPacks:  addedPacks,
		withClient:  withClient,
		connectInfo: connectInfo,

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

func (s *Session) newResourcePackHandler(ctx context.Context) *resourcepacks.ResourcePackHandler {
	rpHandler := resourcepacks.NewResourcePackHandler(ctx, s.addedPacks)
	rpHandler.OnResourcePacksInfoCB = func() {
		messages.SendEvent(&messages.EventConnectStateUpdate{
			State: messages.ConnectStateReceivingResources,
		})
	}
	rpHandler.OnFinishedPack = func(p resource.Pack) error {
		return s.handlers.OnFinishedPack(s, p)
	}
	rpHandler.FilterDownloadResourcePacks = func(id string) bool {
		return s.handlers.FilterResourcePack(s, id)
	}
	return rpHandler
}

func (s *Session) Run() error {
	defer s.cancelCtx(errors.New("done"))

	messages.SendEvent(&messages.EventConnectStateUpdate{
		State:      messages.ConnectStateBegin,
		ListenAddr: s.settings.ListenAddress,
	})

	var err error
	s.blobCache, err = blobcache.NewBlobCache(s.log, func(pk packet.Packet) error {
		return s.Server.WritePacket(pk)
	}, func() minecraft.IConn {
		return s.Client
	}, func(pk packet.Packet, timeReceived time.Time, preLogin bool) error {
		_, err := s.handlers.PacketCallback(s, pk, false, timeReceived, preLogin)
		return err
	}, func(blobs []protocol.CacheBlob) {
		s.handlers.OnBlobs(s, blobs)
	}, s.isReplay)
	if err != nil {
		return err
	}
	defer s.blobCache.Close()

	if s.settings.Debug || s.settings.ExtraDebug {
		s.packetLogger, err = NewPacketLogger(s.settings.ExtraDebug, false)
		if err != nil {
			return err
		}
		defer s.packetLogger.Close()

		s.packetLoggerClient, err = NewPacketLogger(s.settings.ExtraDebug, true)
		if err != nil {
			return err
		}
		defer s.packetLoggerClient.Close()
	}

	if s.connectInfo.IsReplay() {
		replayName, err := s.connectInfo.Address(s.ctx)
		if err != nil {
			return err
		}
		rpHandler := s.newResourcePackHandler(s.ctx)
		replay, err := pcap2.CreateReplayConnector(s.ctx, utils.PathData(replayName), s.packetFunc, rpHandler)
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
		if err = s.connect(); err != nil {
			return err
		}
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

	if s.ctx.Err() != nil {
		err := context.Cause(s.ctx)
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

	disconnect, err := s.handlers.OnServerConnect(s)
	if disconnect {
		err = errCancelConnect
	}
	if err != nil {
		s.cancelCtx(err)
		return err
	}

	gameData := s.Server.GameData()
	s.handlers.GameDataModifier(s, &gameData)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.Server.DoSpawnContext(s.ctx)
		if err != nil {
			s.cancelCtx(err)
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
			err := s.Client.StartGameContext(s.ctx, gameData)
			if err != nil {
				s.cancelCtx(err)
				return
			}
		}()
	}

	wg.Wait()

	err = context.Cause(s.ctx)
	if err != nil {
		s.disconnectReason = err.Error()
		if s.expectDisconnect {
			return nil
		}
		return err
	}

	if s.handlers.OnConnect(s) {
		logrus.Info("Disconnecting")
		return nil
	}
	messages.SendEvent(&messages.EventConnectStateUpdate{
		State: messages.ConnectStateDone,
	})

	doProxy := func(client bool) {
		defer wg.Done()
		if err := s.proxyLoop(s.ctx, client); err != nil {
			if !errors.Is(err, context.Canceled) {
				s.cancelCtx(err)
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
	err = context.Cause(s.ctx)
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

func (s *Session) connect() error {
	var wg sync.WaitGroup
	ctx, cancelCause := context.WithCancelCause(s.ctx)
	defer cancelCause(nil)

	rpHandler := s.newResourcePackHandler(ctx)
	if s.withClient {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.connectClient(ctx, rpHandler)
			if err != nil && !errors.Is(err, context.Canceled) {
				cancelCause(err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.connectServer(ctx, rpHandler)
		if err != nil && !errors.Is(err, context.Canceled) {
			cancelCause(err)
		}
	}()
	wg.Wait()
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Session) connectServer(ctx context.Context, rpHandler *resourcepacks.ResourcePackHandler) (err error) {
	if s.withClient {
		select {
		case <-s.clientConnecting:
		case <-s.ctx.Done():
			return s.ctx.Err()
		}
	}

	messages.SendEvent(&messages.EventConnectStateUpdate{
		State: messages.ConnectStateServerConnecting,
	})

	address, err := s.connectInfo.Address(s.ctx)
	if err != nil {
		return err
	}

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": address}))
	dialer := minecraft.Dialer{
		AuthSource:                 s.connectInfo.Account,
		DisconnectOnUnknownPackets: false,
		ErrorLog:                   slog.Default(),
		PacketFunc:                 s.packetFunc,
		EnableClientCache:          s.settings.ClientCache,
		GetClientData: func() login.ClientData {
			if s.withClient {
				select {
				case <-s.haveClientData:
				case <-ctx.Done():
				}
			}
			return s.clientData
		},
		EarlyConnHandler: func(conn *minecraft.Conn) {
			s.Server = conn
			rpHandler.SetServer(conn)
			conn.ResourcePackHandler = rpHandler
			conn.SetPrePlayPacketHandler(s.serverPrePlayHandler)
		},
	}

	_, err = dialer.DialContext(ctx, "raknet", address, 20*time.Second)
	if err != nil {
		if s.expectDisconnect {
			return nil
		}
		return err
	}

	messages.SendEvent(&messages.EventConnectStateUpdate{
		State: messages.ConnectStateEstablished,
	})
	logrus.Debug(locale.Loc("connected", nil))
	return nil
}

func (s *Session) connectClient(ctx context.Context, rpHandler *resourcepacks.ResourcePackHandler) (err error) {
	clientPacketFunc := func(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
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
	}

	serverName, err := s.connectInfo.Name(ctx)
	if err != nil {
		return err
	}

	s.listener, err = minecraft.ListenConfig{
		AuthenticationDisabled: true,
		AllowUnknownPackets:    true,
		StatusProvider:         minecraft.NewStatusProvider(fmt.Sprintf("%s Proxy", serverName), "Bedrocktool"),
		ErrorLog:               slog.Default(),
		PacketFunc:             clientPacketFunc,
		OnClientData: func(c *minecraft.Conn) {
			s.clientData = c.ClientData()
			ident := c.IdentityData()
			s.handlers.PlayerDataModifier(s, &ident, &s.clientData)
			close(s.haveClientData)
		},
		EarlyConnHandler: func(c *minecraft.Conn) {
			if s.Client != nil {
				s.listener.Disconnect(c, "You are Already connected!")
				return
			}
			s.Client = c
			rpHandler.SetClient(c)
			c.ResourcePackHandler = rpHandler
			close(s.clientConnecting)
		},
	}.Listen("raknet", s.settings.ListenAddress)
	if err != nil {
		return err
	}

	messages.SendEvent(&messages.EventConnectStateUpdate{
		State: messages.ConnectStateListening,
	})
	logrus.Info(locale.Loc("listening_on", locale.Strmap{"Address": s.listener.Addr()}))
	logrus.Info(locale.Loc("help_connect", nil))

	var accepted = false
	go func() {
		<-ctx.Done()
		if !accepted {
			_ = s.listener.Close()
		}
	}()

	_, err = s.listener.AcceptMinecraft()
	if err != nil {
		return err
	}
	accepted = true
	logrus.Info("Client Connected")
	return nil
}

func (s *Session) serverPrePlayHandler(conn *minecraft.Conn, pk packet.Packet, timeReceived time.Time) (handled bool, err error) {
	switch pk := pk.(type) {
	case *packet.BiomeDefinitionList:
		err = s.ClientWritePacket(pk)
		return true, err
	}
	return false, nil
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
		} else {
			if pk.ID() == packet.IDClientCacheBlobStatus {
				forward = nil
			}
		}

		pk, err = s.commandHandlerPacketCB(pk, toServer, timeReceived, false)
		if err != nil {
			return err
		}

		if process {
			pk, err = s.handlers.PacketCallback(s, pk, toServer, timeReceived, false)
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
		if errRec := recover(); errRec != nil {
			switch err := errRec.(type) {
			case error:
				utils.ErrorHandler(err)
			case string:
				utils.ErrorHandler(errors.New(err))
			default:
				panic(errRec)
			}
		}
	}()

	if header.PacketID == packet.IDRequestNetworkSettings {
		s.clientAddr = src
	}
	if header.PacketID == packet.IDSetLocalPlayerAsInitialised {
		s.spawned = true
	}

	s.handlers.PacketRaw(s, header, payload, src, dst, timeReceived)

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
		pk, err = s.handlers.PacketCallback(s, pk, toServer, timeReceived, true)
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

func (s *Session) commandHandlerPacketCB(pk packet.Packet, _ bool, _ time.Time, _ bool) (packet.Packet, error) {
	switch _pk := pk.(type) {
	case *packet.CommandRequest:
		cmd := strings.Split(_pk.CommandLine, " ")
		name := cmd[0][1:]
		if h, ok := s.commands[name]; ok {
			pk = nil
			h.Exec(cmd[1:])
		}
	case *packet.AvailableCommands:
		cmds := make([]protocol.Command, 0, len(s.commands))
		for _, ic := range s.commands {
			cmds = append(cmds, ic.Cmd)
		}
		_pk.Commands = append(_pk.Commands, cmds...)
	}
	return pk, nil
}
