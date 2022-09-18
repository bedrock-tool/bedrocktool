package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/bedrock-skin-bot/utils"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

// Bot is an instance that connects to a server and sends skins it receives to the api server.
type Bot struct {
	// Name is the username of this bot
	Name string
	// Address is the server address this bot will connect to
	Address string
	// ServerName is the readable name of the server
	ServerName string
	// serverConn is the connection to the server
	serverConn *minecraft.Conn
	ctx        context.Context
	log        func() *logrus.Entry

	// map of uuids to player entries
	players map[uuid.UUID]protocol.PlayerListEntry
}

// NewBot creates a new bot
func NewBot(name, address, serverName string) *Bot {
	if !strings.Contains(address, ":") {
		address = address + ":19132"
	}

	b := &Bot{
		Name:       name,
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
		players: map[uuid.UUID]protocol.PlayerListEntry{},
	}

	return b
}

// Start runs the bot indefinitely
func (b *Bot) Start(ctx context.Context) {
	b.ctx = ctx

	t1 := time.Now()
	for {
		if err := b.do(); err != nil {
			b.log().Error(err)
		}
		if time.Since(t1) < 10*time.Second {
			b.log().Error("Failed to fast, exiting this instance")
			break
		}
		time.Sleep(30 * time.Second)
	}
}

func findAllIps(Address string) (ret []string, err error) {
	host, _, err := net.SplitHostPort(Address)
	if err != nil {
		return nil, err
	}
	tmp := map[string]bool{}
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		ips, err := net.LookupHost(host)
		if err != nil {
			return nil, err
		}
		for _, v := range ips {
			tmp[v] = true
		}
	}
	for k := range tmp {
		ret = append(ret, k)
	}
	return ret, nil
}

// do runs until error
func (b *Bot) do() (err error) {
	b.serverConn, err = utils.ConnectServer(b.ctx, b.Address, b.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s", err)
	}
	defer b.serverConn.Close()

	b.log().Info("Spawning")
	if err := b.serverConn.DoSpawnContext(b.ctx); err != nil {
		return fmt.Errorf("failed to spawn: %s", err)
	}

	b.log().Info("Spawned")
	for {
		pk, err := b.serverConn.ReadPacket()
		if err != nil {
			return err
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
		player.Skin = pk.Skin
		b.maybeSubmitPlayer(player)

	case *packet.PlayerList:
		if pk.ActionType == 1 { // remove
			return
		}
		for _, entry := range pk.Entries {
			b.maybeSubmitPlayer(entry)
		}
	}
}

func (b *Bot) maybeSubmitPlayer(entry protocol.PlayerListEntry) {
	b.players[entry.UUID] = entry

	if entry.XUID == b.serverConn.IdentityData().XUID {
		return
	}

	username := utils.CleanupName(entry.Username)
	if len(entry.XUID) < 5 || username == "" { // only xbox logged in users (maybe bad)
		return
	}

	go utils.APIClient.UploadSkin(
		&utils.Skin{entry.Skin},
		username,
		entry.XUID,
		b.ServerName,
	)
}
