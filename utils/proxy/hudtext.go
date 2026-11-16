package proxy

import (
	"io/fs"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

// hudScreenFile is the game's HUD layout. A server pack that has its own
// may draw the tip box as something else: The Hive's makes it a second copy
// of the held item's name, so a tip showed "Game Selector" twice instead of
// the text sent.
const hudScreenFile = "ui/hud_screen.json"

// packsStyleHUD says whether any of the server's resource packs changes the
// HUD. Only file names are looked at; they aren't encrypted.
func packsStyleHUD(packs []resource.Pack) bool {
	for _, pack := range packs {
		if _, err := fs.Stat(pack, hudScreenFile); err == nil {
			return true
		}
	}
	return false
}

// hudTextPacket is text shown above the hotbar for a few seconds: a tip,
// which the game draws on its own (a popup gets a blank line kept for a
// tip above it), or the action bar when the server's packs restyle the HUD
// and so maybe the tip box with it.
func hudTextPacket(text string, styledHUD bool) packet.Packet {
	if styledHUD {
		return &packet.SetTitle{ActionType: packet.TitleActionSetActionBar, Text: text}
	}
	return &packet.Text{TextType: packet.TextTypeTip, Message: text}
}
