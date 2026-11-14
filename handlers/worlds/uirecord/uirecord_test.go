package uirecord

import (
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestRecordMenuChain(t *testing.T) {
	r := New(func(rid uint64) (string, string, bool) {
		return "minecraft:npc", "§aShop", rid == 7
	})
	now := time.Now()
	step := func() time.Time { now = now.Add(200 * time.Millisecond); return now }

	r.OnPacket(&packet.ItemRegistry{Items: []protocol.ItemEntry{{Name: "minecraft:compass", RuntimeID: 400}}}, false, step())

	// using the compass opens the main menu
	r.OnPacket(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{
		ActionType: protocol.UseItemActionClickAir,
		HeldItem:   protocol.ItemInstance{Stack: protocol.ItemStack{ItemType: protocol.ItemType{NetworkID: 400}}},
	}}, true, step())
	r.OnPacket(&packet.ModalFormRequest{FormID: 1, FormData: []byte(`{"type":"form","title":"Games","content":"","buttons":[{"text":"SkyWars"},{"text":"Settings"}]}`)}, false, step())

	// pressing "Settings" opens a custom form
	r.OnPacket(&packet.ModalFormResponse{FormID: 1, ResponseData: protocol.Option([]byte("1"))}, true, step())
	r.OnPacket(&packet.ModalFormRequest{FormID: 2, FormData: []byte(`{"type":"custom_form","title":"Settings","content":[{"type":"label","text":"Hi"},{"type":"toggle","text":"Music","default":true}]}`)}, false, step())

	// touching the NPC opens a modal
	r.OnPacket(&packet.InventoryTransaction{TransactionData: &protocol.UseItemOnEntityTransactionData{
		TargetEntityRuntimeID: 7,
		ActionType:            protocol.UseItemOnEntityActionInteract,
	}}, true, step())
	r.OnPacket(&packet.ModalFormRequest{FormID: 3, FormData: []byte(`{"type":"modal","title":"Shop","content":"Buy?","button1":"Yes","button2":"No"}`)}, false, step())

	r.OnPacket(&packet.SetDisplayObjective{DisplaySlot: "sidebar", ObjectiveName: "side", DisplayName: "§eHIVE", SortOrder: 1}, false, step())
	r.OnPacket(&packet.SetScore{Entries: []protocol.ScoreboardEntry{
		{EntryID: 1, ObjectiveName: "side", Score: 3, IdentityType: protocol.ScoreboardIdentityFakePlayer, DisplayName: "Line A"},
		{EntryID: 2, ObjectiveName: "side", Score: 2, IdentityType: protocol.ScoreboardIdentityFakePlayer, DisplayName: "Line B"},
	}}, false, step())
	r.OnPacket(&packet.SetScore{Entries: []protocol.ScoreboardEntry{{EntryID: 2, IdentityType: protocol.ScoreboardIdentityRemove}}}, false, step())

	if len(r.forms) != 3 {
		t.Fatalf("forms: got %d, want 3", len(r.forms))
	}
	main := r.items["minecraft:compass"]
	if main == "" {
		t.Fatal("compass opens no form")
	}
	if r.next[main]["1"] != r.formIDs[2] {
		t.Fatalf("button 1 of the main menu should open the settings form: %v", r.next)
	}
	if r.entities["minecraft:npc|§aShop"] != r.formIDs[3] {
		t.Fatalf("NPC trigger: %v", r.entities)
	}
	side := r.objectives["side"]
	if side == nil || side.Scores["Line A"] != 3 || len(side.Scores) != 1 || side.Slots["sidebar"] != 1 {
		t.Fatalf("scoreboard: %+v", side)
	}

	// the script parses (without its module imports, which goja lacks)
	script := r.Script()
	var body []string
	for _, line := range strings.Split(script, "\n") {
		if !strings.HasPrefix(line, "import ") {
			body = append(body, line)
		}
	}
	if _, err := goja.Compile("main.js", strings.Join(body, "\n"), true); err != nil {
		t.Fatalf("generated script doesn't parse: %s", err)
	}
}

func TestFormOpenedLateIsUntriggered(t *testing.T) {
	r := New(nil)
	now := time.Now()
	r.OnPacket(&packet.ItemRegistry{Items: []protocol.ItemEntry{{Name: "minecraft:clock", RuntimeID: 9}}}, false, now)
	r.OnPacket(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{
		ActionType: protocol.UseItemActionClickAir,
		HeldItem:   protocol.ItemInstance{Stack: protocol.ItemStack{ItemType: protocol.ItemType{NetworkID: 9}}},
	}}, true, now)
	r.OnPacket(&packet.ModalFormRequest{FormID: 1, FormData: []byte(`{"type":"form","title":"x","content":"","buttons":[]}`)}, false, now.Add(10*time.Second))
	if len(r.items) != 0 {
		t.Fatalf("a form long after the item use shouldn't be tied to it: %v", r.items)
	}
}
