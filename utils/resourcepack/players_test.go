package resourcepack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func clientEntity(t *testing.T, p *Pack, id uuid.UUID) *ClientEntityDescription {
	t.Helper()
	var f ClientEntityFile
	if err := json.Unmarshal(p.Files["entity/players/"+id.String()+".json"], &f); err != nil {
		t.Fatal(err)
	}
	return f.ClientEntity.Description
}

func TestMakePlayersGeometry(t *testing.T) {
	skin := func(patch, geometry string) *protocol.Skin {
		return &protocol.Skin{
			SkinImageWidth: 64, SkinImageHeight: 64, SkinData: make([]byte, 64*64*4),
			SkinResourcePatch: []byte(patch), SkinGeometry: []byte(geometry),
			// a cape ID but no picture
			CapeID: "cape",
		}
	}
	missing, slim, custom := uuid.New(), uuid.New(), uuid.New()
	skins := map[uuid.UUID]*protocol.Skin{
		// names a model it doesn't include
		missing: skin(`{"geometry":{"default":"geometry.persona_1234"}}`, ""),
		slim:    skin(`{"geometry":{"default":"geometry.humanoid.customSlim"}}`, ""),
		custom: skin(`{"geometry":{"default":"geometry.n-pc"}}`,
			`{"format_version":"1.12.0","minecraft:geometry":[{"description":{"identifier":"geometry.n-pc","texture_width":64,"texture_height":64},"bones":[]}]}`),
	}
	p := New()
	var players []EntityPlayer
	for id := range skins {
		players = append(players, EntityPlayer{Identifier: "bedrocktool_player:p_" + id.String(), UUID: id})
	}
	if err := p.MakePlayers(players, skins); err != nil {
		t.Fatal(err)
	}

	if g := clientEntity(t, p, missing).Geometry["default"]; g != "geometry.humanoid.custom" {
		t.Errorf("missing model drawn with %q, want the game's player model", g)
	}
	if g := clientEntity(t, p, slim).Geometry["default"]; g != "geometry.humanoid.customSlim" {
		t.Errorf("slim drawn with %q", g)
	}
	g := clientEntity(t, p, custom).Geometry["default"]
	if !strings.HasPrefix(g, "geometry.bedrocktool_player.") || strings.Contains(g, "-") {
		t.Errorf("own model named %q, want a unique name", g)
	}
	if !strings.Contains(string(p.Files["models/entity/bedrocktool_player/"+g+".geo.json"]), `"`+g+`"`) {
		t.Errorf("no model file defines %q", g)
	}
	if !strings.Contains(string(p.Files["models/entity/bedrocktool_player/"+g+".geo.json"]), `"format_version":"1.12.0"`) {
		t.Error("model file isn't format 1.12.0")
	}
	if rc := clientEntity(t, p, custom).RenderControllers; len(rc) != 1 || rc[0] != "controller.render.default" {
		t.Errorf("render controllers %v", rc)
	}
	d := clientEntity(t, p, custom)
	if !d.EnableAttachables {
		t.Error("attachables (worn cosmetics) not enabled")
	}
	if _, ok := d.Animations["riding.legs"]; !ok || d.Scripts["animate"] == nil {
		t.Errorf("no riding/walking animations: %v", d.Animations)
	}
	if _, ok := d.Animations["root"]; ok {
		t.Error("player root controller is used (needs player-only variables)")
	}
}

func TestMakePlayersCape(t *testing.T) {
	withCape, idOnly := uuid.New(), uuid.New()
	skin := func() *protocol.Skin {
		return &protocol.Skin{SkinImageWidth: 64, SkinImageHeight: 64, SkinData: make([]byte, 64*64*4), CapeID: "cape"}
	}
	caped := skin()
	caped.CapeImageWidth, caped.CapeImageHeight, caped.CapeData = 64, 32, make([]byte, 64*32*4)
	skins := map[uuid.UUID]*protocol.Skin{withCape: caped, idOnly: skin()}

	p := New()
	err := p.MakePlayers([]EntityPlayer{
		{Identifier: "bedrocktool_player:p_a", UUID: withCape},
		{Identifier: "bedrocktool_player:p_b", UUID: idOnly},
	}, skins)
	if err != nil {
		t.Fatal(err)
	}

	d := clientEntity(t, p, withCape)
	if d.Textures["cape"] != "textures/player/cape_"+withCape.String() || d.Geometry["cape"] != capeGeometryID {
		t.Errorf("cape not set up: %v %v", d.Textures, d.Geometry)
	}
	if len(d.RenderControllers) != 2 || d.RenderControllers[1] != capeRenderController {
		t.Errorf("render controllers %v", d.RenderControllers)
	}
	if d.Animations["cape"] != capeAnimation || d.Scripts["scale"] != "0.9375" {
		t.Errorf("cape tilt or scale missing: %v %v", d.Animations, d.Scripts["scale"])
	}
	// the shared animate list isn't changed by a caped player
	for _, a := range playerScripts["animate"].([]any) {
		if a == "cape" {
			t.Error("cape added to every player's animations")
		}
	}
	for _, f := range []string{
		"textures/player/cape_" + withCape.String() + ".png",
		"models/entity/bedrocktool_player/cape.geo.json",
		"render_controllers/bedrocktool_player.render_controllers.json",
	} {
		if len(p.Files[f]) == 0 {
			t.Errorf("missing %s", f)
		}
	}

	// a cape ID with no picture: no cape
	if d := clientEntity(t, p, idOnly); len(d.RenderControllers) != 1 || d.Textures["cape"] != "" {
		t.Errorf("cape drawn without a picture: %v", d)
	}
	if _, ok := p.Files["textures/player/cape_"+idOnly.String()+".png"]; ok {
		t.Error("empty cape png written")
	}
}
