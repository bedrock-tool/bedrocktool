package skins

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type SkinMeta struct {
	SkinID        string
	PlayFabID     string
	PremiumSkin   bool
	PersonaSkin   bool
	CapeID        string
	SkinColour    string
	ArmSize       string
	Trusted       bool
	PersonaPieces []protocol.PersonaPiece
}

type skinsSession struct {
	PlayerNameFilter  string
	OnlyIfHasGeometry bool
	ServerName        string
	Proxy             *utils.ProxyContext
	fpath             string

	playerSkinPacks map[uuid.UUID]*SkinPack
	playerNames     map[uuid.UUID]string
}

func NewSkinsSession(proxy *utils.ProxyContext, serverName, fpath string) *skinsSession {
	return &skinsSession{
		ServerName: serverName,
		Proxy:      proxy,
		fpath:      fpath,

		playerSkinPacks: make(map[uuid.UUID]*SkinPack),
		playerNames:     make(map[uuid.UUID]string),
	}
}

func (s *skinsSession) AddPlayerSkin(playerID uuid.UUID, playerName string, skin *Skin) (added bool) {
	p, ok := s.playerSkinPacks[playerID]
	if !ok {
		creating := fmt.Sprintf("Creating Skinpack for %s", playerName)
		s.Proxy.SendPopup(creating)
		logrus.Info(creating)
		p = NewSkinPack(playerName, s.fpath)
		s.playerSkinPacks[playerID] = p
	}
	if p.AddSkin(skin) {
		if ok {
			addedStr := fmt.Sprintf("Added a skin to %s", playerName)
			s.Proxy.SendPopup(addedStr)
			logrus.Info(addedStr)
		}
		added = true
	}
	if err := p.Save(path.Join(s.fpath, playerName), s.ServerName); err != nil {
		logrus.Error(err)
	}
	return added
}

func (s *skinsSession) AddSkin(playerName string, playerID uuid.UUID, playerSkin *protocol.Skin) (string, *Skin, bool) {
	if playerName == "" {
		playerName = s.playerNames[playerID]
		if playerName == "" {
			playerName = playerID.String()
		}
	}
	if !strings.HasPrefix(playerName, s.PlayerNameFilter) {
		return "", nil, false
	}
	s.playerNames[playerID] = playerName

	skin := &Skin{playerSkin}
	if s.OnlyIfHasGeometry && !skin.HaveGeometry() {
		return "", nil, false
	}
	wasAdded := s.AddPlayerSkin(playerID, playerName, skin)

	return playerName, skin, wasAdded
}

type skinAdd struct {
	PlayerName string
	Skin       *protocol.Skin
}

func (s *skinsSession) ProcessPacket(pk packet.Packet) (out []skinAdd) {
	switch pk := pk.(type) {
	case *packet.PlayerList:
		if pk.ActionType == 1 { // remove
			return nil
		}
		for _, player := range pk.Entries {
			playerName, skin, wasAdded := s.AddSkin(utils.CleanupName(player.Username), player.UUID, &player.Skin)
			if wasAdded {
				out = append(out, skinAdd{
					PlayerName: playerName,
					Skin:       skin.Skin,
				})
			}
		}
	case *packet.AddPlayer:
		if _, ok := s.playerNames[pk.UUID]; !ok {
			s.playerNames[pk.UUID] = utils.CleanupName(pk.Username)
		}
	}
	return out
}

type SkinCMD struct {
	ServerAddress string
	Filter        string
	NoProxy       bool
}

func (*SkinCMD) Name() string     { return "skins" }
func (*SkinCMD) Synopsis() string { return locale.Loc("skins_synopsis", nil) }

func (c *SkinCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.Filter, "filter", "", locale.Loc("name_prefix", nil))
	f.BoolVar(&c.NoProxy, "no-proxy", false, "use headless version")
}

func (c *SkinCMD) Execute(ctx context.Context, ui utils.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	proxy, _ := utils.NewProxy()
	proxy.WithClient = !c.NoProxy
	proxy.OnClientConnect = func(proxy *utils.ProxyContext, hasClient bool) {
		ui.Message(messages.SetUIState, messages.UIStateConnecting)
	}
	proxy.ConnectCB = func(proxy *utils.ProxyContext, err error) bool {
		if err != nil {
			return false
		}
		ui.Message(messages.SetUIState, messages.UIStateMain)
		logrus.Info(locale.Loc("ctrl_c_to_exit", nil))
		return true
	}

	outPathBase := fmt.Sprintf("skins/%s", hostname)
	os.MkdirAll(outPathBase, 0o755)

	s := NewSkinsSession(proxy, hostname, outPathBase)

	proxy.PacketCB = func(pk packet.Packet, _ *utils.ProxyContext, toServer bool, _ time.Time) (packet.Packet, error) {
		if !toServer {
			for _, s := range s.ProcessPacket(pk) {
				ui.Message(messages.NewSkin, messages.NewSkinPayload{
					PlayerName: s.PlayerName,
					Skin:       s.Skin,
				})
			}
		}
		return pk, nil
	}

	if proxy.WithClient {
		ui.Message(messages.SetUIState, messages.UIStateConnect)
	} else {
		ui.Message(messages.SetUIState, messages.UIStateConnecting)
	}
	err = proxy.Run(ctx, address)
	return err
}

func init() {
	utils.RegisterCommand(&SkinCMD{})
}
