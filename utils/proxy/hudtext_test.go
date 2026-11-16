package proxy

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

const testManifest = `{"format_version":2,"header":{"name":"test","description":"","uuid":"8052c7e3-a2bb-3038-a1fe-5558ba5f4657","version":[1,0,0],"min_engine_version":[1,21,0]},"modules":[{"type":"resources","uuid":"1052c7e3-a2bb-3038-a1fe-5558ba5f4657","version":[1,0,0]}]}`

func testPack(t *testing.T, files ...string) resource.Pack {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, data := range map[string]string{"manifest.json": testManifest} {
		w, _ := z.Create(name)
		w.Write([]byte(data))
	}
	for _, name := range files {
		w, _ := z.Create(name)
		// encrypted packs' files are unreadable; only the name matters
		w.Write([]byte{0x9b, 0xcf, 0xb9, 0xfc})
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	pack, err := resource.FromReaderAt(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

func TestPacksStyleHUD(t *testing.T) {
	plain := testPack(t, "textures/blocks/stone.png")
	hud := testPack(t, "ui/_ui_defs.json", "ui/hud_screen.json")
	if packsStyleHUD(nil) || packsStyleHUD([]resource.Pack{plain}) {
		t.Error("packs without a HUD file count as styling the HUD")
	}
	if !packsStyleHUD([]resource.Pack{plain, hud}) {
		t.Error("a pack with ui/hud_screen.json doesn't count as styling the HUD")
	}
}

func TestHudTextPacket(t *testing.T) {
	if pk, ok := hudTextPacket("10 Chunks", false).(*packet.Text); !ok || pk.TextType != packet.TextTypeTip || pk.Message != "10 Chunks" {
		t.Errorf("plain HUD: got %#v, want a tip", hudTextPacket("10 Chunks", false))
	}
	if pk, ok := hudTextPacket("10 Chunks", true).(*packet.SetTitle); !ok || pk.ActionType != packet.TitleActionSetActionBar || pk.Text != "10 Chunks" {
		t.Errorf("styled HUD: got %#v, want the action bar", hudTextPacket("10 Chunks", true))
	}
}
