package main

import (
	"context"
	"fmt"
	"time"

	"github.com/bedrock-tool/bedrocktool/bedrock-skin-bot/utils"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type cachedPlayer struct {
	xuid     string
	uuid     uuid.UUID
	username string
}

// Bot is an instance that connects to a server and sends skins it receives to the api server.
type Bot struct {
	// Username is the username of this bot
	Username string
	// Address is the server address this bot will connect to
	Address string
	// ServerName is the readable name of the server
	ServerName string
	// serverConn is the connection to the server
	serverConn *minecraft.Conn
	ctx        context.Context
	log        func() *logrus.Entry

	// player uuid -> xuid
	players map[uuid.UUID]cachedPlayer
	spawned bool
}

// NewBot creates a new bot
func NewBot(name, address, serverName string) *Bot {
	b := &Bot{
		Username:   name,
		Address:    address,
		ServerName: serverName,
		log: func() *logrus.Entry {
			fields := logrus.Fields{
				"Bot":     name,
				"Address": address,
			}
			if serverName != address {
				fields["ServerName"] = serverName
			}
			return logrus.StandardLogger().WithFields(fields)
		},
		players: map[uuid.UUID]cachedPlayer{},
	}

	return b
}

// Start runs the bot until it errors hard
func (b *Bot) Start(ctx context.Context) {
	b.ctx = ctx

	bots_lock.Lock()
	bots[b.Address] = b
	bots_lock.Unlock()
	defer func() {
		bots_lock.Lock()
		delete(bots, b.Address)
		bots_lock.Unlock()
	}()

	G_metrics.RunningBots.WithLabelValues(b.ServerName).Inc()
	defer G_metrics.RunningBots.WithLabelValues(b.ServerName).Dec()

	for {
		tstart := time.Now()
		if ctx.Err() != nil {
			break
		}
		err := b.do()
		if ctx.Err() != nil {
			break
		}

		shortRun := time.Since(tstart) < 10*time.Second
		if !b.spawned || shortRun {
			G_metrics.Deaths.WithLabelValues(b.ServerName, b.Address).Inc()
			b.log().Warn("Failed to fast, adding ip to waitlist for 15 minutes")
			ip_waitlist[b.Address] = time.Now().Add(15 * time.Minute)
		}

		if err != nil {
			b.log().Warn(err)
			G_metrics.DisconnectEvents.WithLabelValues(b.ServerName, b.Address).Inc()
		}

		time.Sleep(30 * time.Second)
	}
}

// do runs until error
func (b *Bot) do() (err error) {
	b.spawned = false
	b.players = make(map[uuid.UUID]cachedPlayer)

	// connect
	b.serverConn, err = utils.ConnectServer(b.ctx, b.Address, b.Username, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s", err)
	}
	defer b.serverConn.Close()

	// spawn
	if err := b.serverConn.DoSpawnContext(b.ctx); err != nil {
		return fmt.Errorf("failed to spawn: %s", err)
	}
	b.log().Info("Spawned")
	b.spawned = true

	for {
		// reconnect if no packets for 2 minutes
		b.serverConn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		pk, err := b.serverConn.ReadPacket()
		if err != nil {
			return err
		}

		switch pk := pk.(type) {
		case *packet.Disconnect:
			return fmt.Errorf("disconnected from server: %s", pk.Message)
		}

		b.processSkinsPacket(pk)
	}
}

// processSkinsPacket logic with packets to decide if it should upload
func (b *Bot) processSkinsPacket(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.PlayerSkin:
		player, ok := b.players[pk.UUID]
		if !ok {
			b.log().Warnf("%s not found in player list", pk.UUID.String())
			return
		}
		b.maybeSubmitPlayer(player, &utils.Skin{Skin: pk.Skin})

	case *packet.PlayerList:
		if pk.ActionType == 1 { // remove
			return
		}
		for _, entry := range pk.Entries {
			b.maybeSubmitPlayer(cachedPlayer{
				xuid:     entry.XUID,
				uuid:     entry.UUID,
				username: entry.Username,
			}, &utils.Skin{Skin: entry.Skin})
		}
	}
}

func (b *Bot) maybeSubmitPlayer(player cachedPlayer, skin *utils.Skin) {
	b.players[player.uuid] = player

	if player.xuid == b.serverConn.IdentityData().XUID {
		return
	}

	username := utils.CleanupName(player.username)
	if len(player.xuid) < 5 || username == "" { // only xbox logged in users (maybe bad)
		return
	}

	go utils.APIClient.UploadSkin(skin, username, player.xuid, b.ServerName)
}
