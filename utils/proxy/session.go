package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/bedrock-tool/bedrocktool/utils/proxy/blobcache"
	"github.com/bedrock-tool/bedrocktool/utils/proxy/pcap2"
	"github.com/bedrock-tool/bedrocktool/utils/proxy/resourcepacks"
	"github.com/df-mc/go-nethernet"
	"github.com/df-mc/go-nethernet/endpoint"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/p2p"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sandertv/gophertunnel/minecraft/service/signaling"
	"github.com/sandertv/gophertunnel/minecraft/service/signaling/messaging"
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

	signaling  signalingCloser
	p2pSession *p2p.Session
	listener   *minecraft.Listener
	blobCache  *blobcache.Blobcache

	Server minecraft.IConn
	Client minecraft.IConn
	Player Player

	ServerName string

	expectDisconnect bool
	dimensionData    *packet.DimensionData
	clientConnecting chan struct{}
	haveClientData   chan struct{}
	clientData       login.ClientData
	clientAddr       net.Addr
	spawned          bool
	disconnectReason atomic.Pointer[string]
	lastPacketTime   atomic.Pointer[time.Time]
}

type signalingCloser interface {
	nethernet.Signaling
	io.Closer
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
		commands:         make(map[string]ingameCommand),
	}
}

func (s *Session) Now() time.Time {
	return *s.lastPacketTime.Load()
}

func (s *Session) resolveServerName(ctx context.Context) error {
	if s.connectInfo == nil {
		return errors.New("no connect info")
	}
	parsed, err := s.connectInfo.Parsed()
	if err != nil {
		return err
	}

	switch parsed.Kind {
	case connectinfo.KindReplay:
		s.ServerName = filepath.Base(parsed.Replay)
	case connectinfo.KindXbox:
		id, err := uuid.Parse(parsed.Xbox)
		if err != nil {
			return fmt.Errorf("xbox: %w", err)
		}
		p2pSession, err := s.joinP2P(ctx, id)
		if err != nil {
			return err
		}
		world := p2pSession.World()
		switch {
		case world.WorldName != "":
			s.ServerName = world.WorldName
		case world.HostName != "":
			s.ServerName = world.HostName
		default:
			s.ServerName = id.String()
		}
	case connectinfo.KindRealm:
		realm, err := s.connectInfo.Realm(ctx)
		if err != nil {
			return err
		}
		if realm == nil {
			return fmt.Errorf("realm %q not found", parsed.RealmName)
		}
		s.ServerName = realm.Name
	case connectinfo.KindExperience:
		s.ServerName = parsed.Experience
	case connectinfo.KindServer:
		if name, err := s.pingName(ctx, parsed.Address); err == nil && name != "" {
			s.ServerName = name
		} else {
			s.ServerName = addressHost(parsed.Address)
		}
	default:
		return fmt.Errorf("invalid connect info kind %q", parsed.Kind)
	}
	return nil
}

func (s *Session) joinP2P(ctx context.Context, id uuid.UUID) (*p2p.Session, error) {
	if s.p2pSession != nil {
		return s.p2pSession, nil
	}
	xblClient, err := s.connectInfo.Account.XBLClient(ctx)
	if err != nil {
		return nil, err
	}
	session, err := p2p.NewClient(xblClient).Join(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("joining session: %w", err)
	}
	s.p2pSession = session
	return session, nil
}

func (s *Session) pingName(ctx context.Context, address string) (string, error) {
	pong, err := (minecraft.NetherNet{Log: slog.Default()}).PingContext(ctx, address)
	if err != nil {
		return "", err
	}
	return minecraft.ParsePongData(pong).ServerName, nil
}

func addressHost(address string) string {
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" {
		return address
	}
	switch port := u.Port(); port {
	case "", "80", "443", "19132":
		return u.Hostname()
	default:
		return u.Hostname() + "_" + port
	}
}

// AddCommand adds a command to the command handler
func (s *Session) AddCommand(exec func([]string) bool, cmd protocol.Command) {
	cmd.AliasesOffset = 0xffffffff
	cmd.PermissionLevel = protocol.CommandPermissionLevelAny
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
	return rpHandler
}

func (s *Session) newBlobCache(isReplay bool) (*blobcache.Blobcache, error) {
	return blobcache.NewBlobCache(s.log, func(pk packet.Packet) error {
		return s.Server.WritePacket(pk)
	}, func() minecraft.IConn {
		return s.Client
	}, func(pk packet.Packet, timeReceived time.Time, preLogin bool) error {
		_, err := s.handlers.PacketCallback(s, pk, false, timeReceived, preLogin)
		return err
	}, func(blobs []protocol.CacheBlob) {
		s.handlers.OnBlobs(s, blobs)
	}, isReplay)
}

func (s *Session) Run() (err error) {
	defer s.cancelCtx(errors.New("done"))
	defer func() {
		if s.signaling != nil {
			_ = s.signaling.Close()
		}
	}()
	defer func() {
		if s.p2pSession != nil {
			_ = s.p2pSession.Close()
		}
	}()

	parsed, err := s.connectInfo.Parsed()
	if err != nil {
		return err
	}
	isReplay := parsed.Kind == connectinfo.KindReplay

	if isReplay {
		if err := s.resolveServerName(s.ctx); err != nil {
			return err
		}
		if err := s.handlers.SessionStart(s, s.ServerName); err != nil {
			return err
		}
	}

	messages.SendEvent(&messages.EventConnectStateUpdate{
		State:      messages.ConnectStateBegin,
		ListenAddr: s.settings.ListenAddress,
	})

	s.blobCache, err = s.newBlobCache(isReplay)
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

	if isReplay {
		rpHandler := s.newResourcePackHandler(s.ctx)
		replay, err := pcap2.CreateReplayConnector(s.ctx, utils.PathData(parsed.Replay), s.packetFunc, rpHandler)
		if err != nil {
			return err
		}
		s.Server = replay
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
				var reason = "Disconnected"
				if disconnectReason := s.disconnectReason.Load(); disconnectReason != nil {
					reason = *disconnectReason
				}
				_ = s.listener.Disconnect(s.Client.(*minecraft.Conn), reason)
			}
			_ = s.listener.Close()
		}()
	}

	if s.ctx.Err() != nil {
		err := context.Cause(s.ctx)
		if errors.Is(err, errCancelConnect) {
			err = nil
		}
		reason := "Disconnected"
		if err != nil {
			reason = err.Error()
		}
		s.disconnectReason.Store(&reason)

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
	wg.Go(func() {
		err := s.Server.DoSpawnContext(s.ctx)
		if err != nil {
			s.cancelCtx(err)
			return
		}
	})

	if s.Client != nil {
		wg.Go(func() {
			if s.dimensionData != nil {
				s.Client.WritePacket(s.dimensionData)
			}
			err := s.Client.StartGameContext(s.ctx, gameData)
			if err != nil {
				s.cancelCtx(err)
				return
			}
		})
	}

	wg.Wait()

	err = context.Cause(s.ctx)
	if err != nil {
		reason := err.Error()
		s.disconnectReason.Store(&reason)
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
		reason := err.Error()
		s.disconnectReason.Store(&reason)
		return err
	}

	return nil
}

func (s *Session) connect() error {
	if err := s.resolveServerName(s.ctx); err != nil {
		return err
	}
	if err := s.handlers.SessionStart(s, s.ServerName); err != nil {
		return err
	}

	var wg sync.WaitGroup
	ctx, cancelCause := context.WithCancelCause(s.ctx)
	defer cancelCause(nil)

	rpHandler := s.newResourcePackHandler(ctx)
	if s.withClient {
		wg.Go(func() {
			err := s.connectClient(ctx, rpHandler)
			if err != nil && !errors.Is(err, context.Canceled) {
				cancelCause(err)
			}
		})
	}
	wg.Go(func() {
		err := s.connectServer(ctx, rpHandler)
		if err != nil && !errors.Is(err, context.Canceled) {
			cancelCause(err)
		}
	})
	wg.Wait()
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Session) resolveServerAddress(ctx context.Context, parsed *connectinfo.ParsedConnectInfo) (address, clientNonce string, signalingConn signalingCloser, err error) {
	switch parsed.Kind {
	case connectinfo.KindServer:
		return parsed.Address, "", nil, nil

	case connectinfo.KindRealm:
		realm, err := s.connectInfo.Realm(ctx)
		if err != nil {
			return "", "", nil, err
		}
		addr, err := realm.Address(ctx)
		if err != nil {
			return "", "", nil, err
		}

		tokenSource, err := s.connectInfo.Account.TokenSource(ctx)
		if err != nil {
			return "", "", nil, fmt.Errorf("creating TokenSource: %s", err)
		}

		networkProto := realms.ParseNetworkProtocol(string(addr.NetworkProtocol))
		var signalingConn signalingCloser
		switch networkProto {
		case realms.NetworkProtocolNetherNetJSONRPC:
			var d messaging.Dialer
			signalingConn, err = d.DialContext(ctx, tokenSource)
		case realms.NetworkProtocolNetherNet:
			var d signaling.Dialer
			signalingConn, err = d.DialContext(ctx, tokenSource)
		default:
			return "", "", nil, fmt.Errorf("unimplemented signaling type %q", networkProto)
		}
		return addr.Address, "", signalingConn, nil

	case connectinfo.KindExperience:
		experience, err := s.connectInfo.Experience(ctx)
		if err != nil {
			return "", "", nil, err
		}
		addr, err := experience.Join(ctx)
		if err != nil {
			return "", "", nil, err
		}
		address, err = connectinfo.NormalizeAddress(addr.String())
		if err != nil {
			return "", "", nil, err
		}
		return address, "", nil, nil

	case connectinfo.KindReplay:
		panic("invalid")

	case connectinfo.KindXbox:
		id, err := uuid.Parse(parsed.Xbox)
		if err != nil {
			return "", "", nil, err
		}
		session, err := s.joinP2P(ctx, id)
		if err != nil {
			return "", "", nil, err
		}
		tokenSource, err := s.connectInfo.Account.TokenSource(ctx)
		if err != nil {
			return "", "", nil, fmt.Errorf("creating TokenSource: %s", err)
		}
		var signalingConn signalingCloser
		switch session.Connection().Type {
		case p2p.ConnectionTypeSignalingOverJSONRPC:
			var d messaging.Dialer
			signalingConn, err = d.DialContext(ctx, tokenSource)
		case p2p.ConnectionTypeSignalingOverWebSocket:
			var d signaling.Dialer
			signalingConn, err = d.DialContext(ctx, tokenSource)
		default:
			return "", "", nil, fmt.Errorf("unimplemented signaling type %d", session.Connection().Type)
		}
		if err != nil {
			return "", "", nil, fmt.Errorf("error dialing messaging conn: %s", err)
		}
		return session.Connection().Address(), session.Nonce(), signalingConn, nil

	default:
		panic("unhandled connection type: " + parsed.Kind)
	}
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

	xblClient, err := s.connectInfo.Account.XBLClient(ctx)
	if err != nil {
		return err
	}

	playfabClient, err := s.connectInfo.Account.PlayFabClient(ctx)
	if err != nil {
		return err
	}

	var clientNonce string
	dialer := minecraft.Dialer{
		TokenSource:   s.connectInfo.Account,
		XBLClient:     xblClient,
		PlayFabClient: playfabClient,

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
			clientData := s.clientData
			clientData.Nonce = clientNonce
			return clientData
		},
		EarlyConnHandler: func(conn *minecraft.Conn) {
			s.Server = conn
			rpHandler.SetServer(conn)
			conn.ResourcePackHandler = rpHandler
			conn.SetPrePlayPacketHandler(s.serverPrePlayHandler)
		},
	}

	parsed, err := s.connectInfo.Parsed()
	if err != nil {
		return err
	}
	var address string
	var signalingConn signalingCloser
	address, clientNonce, signalingConn, err = s.resolveServerAddress(ctx, parsed)
	if err != nil {
		return err
	}
	s.signaling = signalingConn

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": address}))

	if _, err := dialer.DialContextNetwork(ctx, minecraft.NetherNet{
		Signaling: s.signaling,
		Log:       slog.Default(),
	}, address); err != nil {
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

	serverName := s.ServerName

	epl, err := net.Listen("tcp", s.settings.ListenAddress)
	if err != nil {
		return err
	}
	defer epl.Close()
	ep := endpoint.NewHandler()
	go http.Serve(epl, ep)

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
	}.ListenNetwork(minecraft.NetherNet{
		Signaling: ep,
		Log:       slog.Default(),
		Dialer: nethernet.Dialer{
			Log: slog.Default(),
		},
	}, s.settings.ListenAddress)
	if err != nil {
		return err
	}

	messages.SendEvent(&messages.EventConnectStateUpdate{
		State: messages.ConnectStateListening,
	})
	logrus.Info(locale.Loc("listening_on", locale.Strmap{"Address": epl.Addr()}))
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
	ep.Close()
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
		s.lastPacketTime.Store(&timeReceived)

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

		pk, err = s.playerPacketCB(pk, toServer, timeReceived, false)
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
			if item, ok := _pk.StorageItem.Value(); ok {
				if item.Stack.NetworkID == -1 {
					item.Stack.NetworkID = 0
					_pk.StorageItem = protocol.Option(item)
				}
			}
		}

		if pk != nil && c2 != nil {
			if err := c2.WritePacket(pk); err != nil {
				reason := err.Error()
				s.disconnectReason.Store(&reason)
				if errors.Is(err, net.ErrClosed) {
					err = nil
				}
				return err
			}
		}

		if transfer != nil {
			return &errTransfer{Transfer: transfer}
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

func (s *Session) playerPacketCB(pk packet.Packet, _ bool, _ time.Time, _ bool) (packet.Packet, error) {
	if pk, ok := pk.(*packet.PacketViolationWarning); ok {
		logrus.Infof("%+#v\n", pk)
	}

	haveMoved := s.Player.handlePackets(pk)
	if haveMoved {
		s.handlers.OnPlayerMove(s)
	}
	return pk, nil
}
