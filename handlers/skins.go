package handlers

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type SkinSaver struct {
	PlayerNameFilter  string
	OnlyIfHasGeometry bool
	ServerName        string
	Proxy             *proxy.Context
	fpath             string

	playerSkinPacks map[uuid.UUID]*utils.SkinPack
	playerNames     map[uuid.UUID]string
}

func (s *SkinSaver) AddPlayerSkin(playerID uuid.UUID, playerName string, skin *utils.Skin) (added bool) {
	p, ok := s.playerSkinPacks[playerID]
	if !ok {
		creating := fmt.Sprintf("Creating Skinpack for %s", playerName)
		s.Proxy.SendPopup(creating)
		logrus.Info(creating)
		p = utils.NewSkinPack(playerName, s.fpath)
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

func (s *SkinSaver) AddSkin(playerName string, playerID uuid.UUID, playerSkin *protocol.Skin) (string, *utils.Skin, bool) {
	if playerName == "" {
		playerName = s.playerNames[playerID]
		if playerName == "" {
			playerName = playerID.String()
		}
	}
	if !strings.HasPrefix(playerName, s.PlayerNameFilter) {
		return playerName, nil, false
	}
	s.playerNames[playerID] = playerName

	skin := &utils.Skin{Skin: playerSkin}
	if s.OnlyIfHasGeometry && !skin.HaveGeometry() {
		return playerName, nil, false
	}
	wasAdded := s.AddPlayerSkin(playerID, playerName, skin)

	return playerName, skin, wasAdded
}

type SkinAdd struct {
	PlayerName string
	Skin       *protocol.Skin
}

func (s *SkinSaver) ProcessPacket(pk packet.Packet) (out []SkinAdd) {
	switch pk := pk.(type) {
	case *packet.PlayerList:
		if pk.ActionType == 1 { // remove
			return nil
		}
		for _, player := range pk.Entries {
			playerName, skin, wasAdded := s.AddSkin(utils.CleanupName(player.Username), player.UUID, &player.Skin)
			if wasAdded {
				out = append(out, SkinAdd{
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

func NewSkinSaver(skinCB func(SkinAdd)) *proxy.Handler {
	s := &SkinSaver{
		playerSkinPacks: make(map[uuid.UUID]*utils.SkinPack),
		playerNames:     make(map[uuid.UUID]string),
	}
	return &proxy.Handler{
		Name: "Skin Saver",
		ProxyRef: func(pc *proxy.Context) {
			s.Proxy = pc
		},
		AddressAndName: func(address, hostname string) error {
			outPathBase := fmt.Sprintf("skins/%s", hostname)
			os.MkdirAll(outPathBase, 0o755)
			s.fpath = outPathBase
			return nil
		},
		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			for _, s := range s.ProcessPacket(pk) {
				if skinCB != nil {
					skinCB(s)
				}
			}
			return pk, nil
		},
	}
}
