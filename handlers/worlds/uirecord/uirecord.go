// Package uirecord watches the forms and scoreboards a server shows the
// player and turns them into a Script API script for the saved world's
// behavior pack: using the item (or touching the entity) that opened a menu
// opens the same menu, the buttons pressed lead to the same next menus, and
// the scoreboards come back as they were last seen.
package uirecord

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// triggerWindow is how long after using an item or touching an entity a
// form still counts as opened by it, and how long after a button press the
// next form counts as that button's.
const triggerWindow = 3 * time.Second

// EntityInfo gives an entity's type and name tag by runtime ID.
type EntityInfo func(rid uint64) (typeID, nameTag string, ok bool)

// Form is a form the server sent, as its JSON.
type Form struct {
	Key  string         `json:"key"`
	Data map[string]any `json:"data"`
}

type objective struct {
	Name        string           `json:"name"`
	DisplayName string           `json:"displayName"`
	Slots       map[string]int32 `json:"slots"` // display slot -> sort order
	Scores      map[string]int32 `json:"scores"`
}

type pendingResponse struct {
	formKey   string
	selection string
	at        time.Time
}

type trigger struct {
	kind string // "item" or "entity"
	id   string
	at   time.Time
}

type Recorder struct {
	mu sync.Mutex

	entityInfo EntityInfo
	itemNames  map[int32]string

	forms     map[string]*Form
	formOrder []string
	formIDs   map[uint32]string            // live form ID -> form key
	next      map[string]map[string]string // form key -> selection -> form key
	items     map[string]string            // item identifier -> form key
	entities  map[string]string            // "type|nameTag" -> form key

	lastTrigger  *trigger
	lastResponse *pendingResponse

	objectives map[string]*objective
	entryNames map[int64]string // scoreboard entry ID -> fake player name
	// boards is the scoreboard of each saved world (FinishWorld), by the
	// dimension it is in the saved world ("" for the main world's own).
	boards map[string][]*objective
}

func New(entityInfo EntityInfo) *Recorder {
	return &Recorder{
		entityInfo: entityInfo,
		itemNames:  make(map[int32]string),
		forms:      make(map[string]*Form),
		formIDs:    make(map[uint32]string),
		next:       make(map[string]map[string]string),
		items:      make(map[string]string),
		entities:   make(map[string]string),
		objectives: make(map[string]*objective),
		entryNames: make(map[int64]string),
		boards:     make(map[string][]*objective),
	}
}

// HasContent says whether anything was recorded.
func (r *Recorder) HasContent() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.forms) > 0 || len(r.objectives) > 0 || len(r.boards) > 0
}

// FinishWorld files the scoreboard seen since the last world under the
// dimension the world being saved has in the saved world ("" for a world
// saved on its own, or the main one of merged worlds; a merged world's
// custom dimension otherwise), and starts the next world's afresh: a
// server shows a scoreboard per world (Hive's game maps have their own).
func (r *Recorder) FinishWorld(dimension string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.boards[dimension] = sortedObjectives(r.objectives)
	// The game keeps showing an objective across worlds until the server
	// changes it (Hive shows hive_ui once): the next world keeps which
	// objectives are shown where, with its own scores.
	next := make(map[string]*objective, len(r.objectives))
	for name, o := range r.objectives {
		next[name] = &objective{Name: o.Name, DisplayName: o.DisplayName, Slots: maps.Clone(o.Slots), Scores: map[string]int32{}}
	}
	r.objectives = next
	r.entryNames = make(map[int64]string)
}

func sortedObjectives(m map[string]*objective) []*objective {
	out := make([]*objective, 0, len(m))
	for _, o := range m {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// OnPacket records what a packet adds. It is called for every packet, both
// ways.
func (r *Recorder) OnPacket(pk packet.Packet, toServer bool, now time.Time) {
	// looked up before taking the lock: entityInfo takes the world's locks
	var entityTrigger string
	if it, ok := pk.(*packet.InventoryTransaction); ok && toServer && r.entityInfo != nil {
		// Hive's NPCs open their menus on a hit as well as on interaction
		// (Hive Recode opens them on both too).
		if td, ok := it.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok && (td.ActionType == protocol.UseItemOnEntityActionInteract || td.ActionType == protocol.UseItemOnEntityActionAttack) {
			// An unnamed vanilla mob can't stand for one NPC: Hive's are
			// invisible sheep hitboxes, which aren't saved, and every other
			// sheep would open the menu.
			if typeID, nameTag, ok := r.entityInfo(td.TargetEntityRuntimeID); ok && (nameTag != "" || !strings.HasPrefix(typeID, "minecraft:")) {
				entityTrigger = typeID + "|" + nameTag
			}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	switch pk := pk.(type) {
	case *packet.ItemRegistry:
		for _, it := range pk.Items {
			r.itemNames[int32(it.RuntimeID)] = it.Name
		}

	case *packet.InventoryTransaction:
		if !toServer {
			return
		}
		switch td := pk.TransactionData.(type) {
		case *protocol.UseItemTransactionData:
			if td.ActionType != protocol.UseItemActionClickAir && td.ActionType != protocol.UseItemActionClickBlock {
				return
			}
			if name, ok := r.itemNames[td.HeldItem.Stack.NetworkID]; ok && td.HeldItem.Stack.NetworkID != 0 {
				r.lastTrigger = &trigger{kind: "item", id: name, at: now}
			}
		case *protocol.UseItemOnEntityTransactionData:
			if entityTrigger != "" {
				r.lastTrigger = &trigger{kind: "entity", id: entityTrigger, at: now}
			}
		}

	case *packet.ModalFormRequest:
		if toServer {
			return
		}
		r.onFormRequest(pk, now)

	case *packet.ModalFormResponse:
		if !toServer {
			return
		}
		key, ok := r.formIDs[pk.FormID]
		if !ok {
			return
		}
		data, ok := pk.ResponseData.Value()
		if !ok {
			r.lastResponse = nil // closed
			return
		}
		sel := selectionOf(r.forms[key], data)
		if sel == "" {
			r.lastResponse = nil
			return
		}
		r.lastResponse = &pendingResponse{formKey: key, selection: sel, at: now}

	case *packet.SetDisplayObjective:
		if toServer {
			return
		}
		o := r.objective(pk.ObjectiveName)
		o.DisplayName = pk.DisplayName
		if pk.DisplaySlot != "" {
			// a slot shows one objective at a time
			for _, other := range r.objectives {
				delete(other.Slots, pk.DisplaySlot)
			}
			o.Slots[pk.DisplaySlot] = pk.SortOrder
		}

	case *packet.RemoveObjective:
		if !toServer {
			delete(r.objectives, pk.ObjectiveName)
		}

	case *packet.SetScore:
		if toServer {
			return
		}
		for _, e := range pk.Entries {
			switch e.IdentityType {
			case protocol.ScoreboardIdentityFakePlayer:
				o := r.objective(e.ObjectiveName)
				if old, ok := r.entryNames[e.EntryID]; ok && old != e.DisplayName {
					delete(o.Scores, old)
				}
				r.entryNames[e.EntryID] = e.DisplayName
				o.Scores[e.DisplayName] = e.Score
			case protocol.ScoreboardIdentityRemove:
				name, ok := r.entryNames[e.EntryID]
				if !ok {
					continue
				}
				for _, o := range r.objectives {
					if e.ObjectiveName == "" || o.Name == e.ObjectiveName {
						delete(o.Scores, name)
					}
				}
				delete(r.entryNames, e.EntryID)
			}
			// real players and entities aren't in the saved world by name
		}
	}
}

func (r *Recorder) objective(name string) *objective {
	o, ok := r.objectives[name]
	if !ok {
		o = &objective{Name: name, DisplayName: name, Slots: map[string]int32{}, Scores: map[string]int32{}}
		r.objectives[name] = o
	}
	return o
}

func (r *Recorder) onFormRequest(pk *packet.ModalFormRequest, now time.Time) {
	var data map[string]any
	if err := json.Unmarshal(pk.FormData, &data); err != nil {
		return
	}
	switch data["type"] {
	case "form", "modal", "custom_form":
	default:
		return
	}

	// the same menu sent again is the same node
	canon, _ := json.Marshal(data)
	sum := sha1.Sum(canon)
	key := "f" + hex.EncodeToString(sum[:6])
	if _, ok := r.forms[key]; !ok {
		r.forms[key] = &Form{Key: key, Data: data}
		r.formOrder = append(r.formOrder, key)
	}
	r.formIDs[pk.FormID] = key

	switch {
	case r.lastResponse != nil && now.Sub(r.lastResponse.at) < triggerWindow:
		m := r.next[r.lastResponse.formKey]
		if m == nil {
			m = make(map[string]string)
			r.next[r.lastResponse.formKey] = m
		}
		if _, seen := m[r.lastResponse.selection]; !seen {
			m[r.lastResponse.selection] = key
		}
	case r.lastTrigger != nil && now.Sub(r.lastTrigger.at) < triggerWindow:
		target := r.items
		if r.lastTrigger.kind == "entity" {
			target = r.entities
		}
		if _, seen := target[r.lastTrigger.id]; !seen {
			target[r.lastTrigger.id] = key
		}
	}
	r.lastResponse = nil
	r.lastTrigger = nil
}

// selectionOf turns a response into the key the script looks the next form
// up by: the button index for menus, "0"/"1" for the two buttons of a modal,
// and "submit" for custom forms.
func selectionOf(f *Form, data []byte) string {
	s := strings.TrimSpace(string(data))
	if s == "" || s == "null" {
		return ""
	}
	if f == nil {
		return ""
	}
	switch f.Data["type"] {
	case "form":
		if _, err := strconv.Atoi(s); err == nil {
			return s
		}
	case "modal":
		switch s {
		case "true":
			return "0"
		case "false":
			return "1"
		}
	case "custom_form":
		return "submit"
	}
	return ""
}

// Script is the recorded menus and scoreboards as a Script API script.
func (r *Recorder) Script() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	forms := make(map[string]any, len(r.forms))
	for _, k := range r.formOrder {
		forms[k] = r.forms[k].Data
	}
	// a world not finished yet shows as the main one
	boards := maps.Clone(r.boards)
	if len(r.objectives) > 0 || len(boards) == 0 {
		boards[""] = sortedObjectives(r.objectives)
	}

	js := func(v any) string {
		b, _ := json.MarshalIndent(v, "", "  ")
		return string(b)
	}

	var sb strings.Builder
	sb.WriteString("// Recorded by bedrocktool: the server's menus and scoreboards, rebuilt.\n")
	sb.WriteString("// Menus that weren't opened by an item or an entity can be opened with\n")
	sb.WriteString("//   /scriptevent bedrocktool:form <key>\n\n")
	sb.WriteString(scriptHeader)
	fmt.Fprintf(&sb, "const FORMS = %s;\n\n", js(forms))
	// Scripts only show images from texture paths: images the server gave
	// as URLs are saved in the world's resource pack at these paths.
	fmt.Fprintf(&sb, "const IMAGES = %s;\n\n", js(r.imagesLocked()))
	fmt.Fprintf(&sb, "const NEXT = %s;\n\n", js(r.next))
	fmt.Fprintf(&sb, "const ITEM_TRIGGERS = %s;\n\n", js(r.items))
	fmt.Fprintf(&sb, "const ENTITY_TRIGGERS = %s;\n\n", js(r.entities))
	fmt.Fprintf(&sb, "const SCOREBOARDS = %s;\n\n", js(boards))
	sb.WriteString(scriptBody)
	return sb.String()
}

const scriptHeader = `import { world, system, DisplaySlotId, ObjectiveSortOrder } from "@minecraft/server";
import { ActionFormData, MessageFormData, ModalFormData } from "@minecraft/server-ui";

`

const scriptBody = `function text(t) {
  if (t === undefined || t === null) return "";
  if (typeof t === "string") return t;
  return { rawtext: Array.isArray(t.rawtext) ? t.rawtext : [{ text: JSON.stringify(t) }] };
}

function build(f) {
  switch (f.type) {
    case "form": {
      const form = new ActionFormData().title(text(f.title)).body(text(f.content));
      // Newer forms (Hive's cosmetics, lockers...) list their buttons with
      // headers, labels and dividers in "elements" instead of "buttons".
      const elements = f.elements ?? f.buttons ?? [];
      for (const e of elements) {
        switch (e.type ?? "button") {
          case "button": {
            const icon = !e.image ? undefined : e.image.type === "path" ? e.image.data : IMAGES[e.image.data];
            form.button(text(e.text), icon);
            break;
          }
          // only in newer @minecraft/server-ui versions
          case "header":
            if (typeof form.header === "function") form.header(text(e.text));
            break;
          case "label":
            if (typeof form.label === "function") form.label(text(e.text));
            break;
          case "divider":
            if (typeof form.divider === "function") form.divider();
            break;
        }
      }
      return form;
    }
    case "modal":
      return new MessageFormData()
        .title(text(f.title))
        .body(text(f.content))
        .button1(text(f.button1))
        .button2(text(f.button2));
    case "custom_form": {
      const form = new ModalFormData().title(text(f.title));
      let pending = "";
      for (const e of f.content ?? []) {
        const label = pending + (typeof e.text === "string" ? e.text : "");
        pending = "";
        switch (e.type) {
          case "label":
            pending = label + "\n";
            break;
          case "input":
            form.textField(label, e.placeholder ?? "", e.default ?? "");
            break;
          case "toggle":
            form.toggle(label, !!e.default);
            break;
          case "slider":
            form.slider(label, e.min ?? 0, e.max ?? 100, e.step ?? 1, e.default ?? e.min ?? 0);
            break;
          case "dropdown":
            form.dropdown(label, (e.options ?? []).map(text), e.default ?? 0);
            break;
          case "step_slider":
            form.dropdown(label, (e.steps ?? []).map(text), e.default ?? 0);
            break;
        }
      }
      return form;
    }
  }
  return undefined;
}

function showForm(player, key) {
  const f = FORMS[key];
  if (!f) return;
  const form = build(f);
  if (!form) return;
  form.show(player).then((r) => {
    if (r.canceled) return;
    const sel = f.type === "custom_form" ? "submit" : String(r.selection);
    const next = NEXT[key] && NEXT[key][sel];
    if (next) system.run(() => showForm(player, next));
  }).catch(() => {});
}

world.afterEvents.itemUse.subscribe((ev) => {
  const key = ITEM_TRIGGERS[ev.itemStack.typeId];
  if (key) system.run(() => showForm(ev.source, key));
});

function entityForm(player, t) {
  const key = ENTITY_TRIGGERS[t.typeId + "|" + (t.nameTag ?? "")] ?? ENTITY_TRIGGERS[t.typeId + "|"];
  if (key) system.run(() => showForm(player, key));
}

world.afterEvents.playerInteractWithEntity.subscribe((ev) => entityForm(ev.player, ev.target));

// Hive's NPCs open their menus when hit, too.
world.afterEvents.entityHitEntity.subscribe((ev) => {
  if (ev.damagingEntity?.typeId === "minecraft:player") entityForm(ev.damagingEntity, ev.hitEntity);
});

system.afterEvents.scriptEventReceive.subscribe((ev) => {
  if (ev.id !== "bedrocktool:form") return;
  const player = ev.sourceEntity ?? world.getAllPlayers()[0];
  if (player) system.run(() => showForm(player, ev.message.trim()));
});

const SLOTS = { sidebar: DisplaySlotId.Sidebar, list: DisplaySlotId.List, belowname: DisplaySlotId.BelowName };

// SCOREBOARDS has each saved world's scoreboard by its dimension: "" is
// the main world's (overworld, nether, end), merged worlds have their own.
// Scoreboards are the whole world's, so the one shown follows the player.
const RECORDED = new Set(Object.values(SCOREBOARDS).flat().map((o) => o.name));

function showScoreboard(dimensionId) {
  const board = SCOREBOARDS[dimensionId] ?? (dimensionId.startsWith("minecraft:") ? SCOREBOARDS[""] : undefined) ?? [];
  for (const id of Object.values(SLOTS)) {
    try { world.scoreboard.clearObjectiveAtDisplaySlot(id); } catch (e) {}
  }
  // the recorded objectives are rebuilt, so another world's don't linger
  for (const name of RECORDED) {
    const obj = world.scoreboard.getObjective(name);
    if (obj) {
      try { world.scoreboard.removeObjective(obj); } catch (e) {}
    }
  }
  for (const o of board) {
    let obj;
    try {
      obj = world.scoreboard.addObjective(o.name, o.displayName);
    } catch (e) {
      continue;
    }
    for (const [name, score] of Object.entries(o.scores)) {
      try { obj.setScore(name, score); } catch (e) {}
    }
    for (const [slot, order] of Object.entries(o.slots)) {
      const id = SLOTS[slot];
      if (!id) continue;
      try {
        world.scoreboard.setObjectiveAtDisplaySlot(id, {
          objective: obj,
          sortOrder: order === 1 ? ObjectiveSortOrder.Descending : ObjectiveSortOrder.Ascending,
        });
      } catch (e) {}
    }
  }
}

world.afterEvents.playerSpawn.subscribe((ev) => {
  if (ev.initialSpawn) system.run(() => showScoreboard(ev.player.dimension.id));
});

world.afterEvents.playerDimensionChange.subscribe((ev) => {
  system.run(() => showScoreboard(ev.toDimension.id));
});
`

// imagePathChars is what a texture path may hold.
var imagePathChars = regexp.MustCompile(`^[A-Za-z0-9_\-./]+$`)

// ImagePath is where an image a form button shows from a URL goes in the
// saved world's resource pack, as a texture path without extension: the
// URL's path, so Hive's https://cdn.playhive.com/icons/locker/x.png is
// icons/locker/x, the layout Hive Recode's pack uses for the same images.
func ImagePath(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", false
	}
	p := path.Clean("/" + u.Path)[1:]
	p = strings.TrimSuffix(p, path.Ext(p))
	if p == "" || strings.Contains(p, "..") || !imagePathChars.MatchString(p) {
		return "", false
	}
	return p, true
}

// Images is the image URLs of the recorded forms' buttons, with the texture
// path (ImagePath) each is shown from in the saved world.
func (r *Recorder) Images() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.imagesLocked()
}

func (r *Recorder) imagesLocked() map[string]string {
	images := make(map[string]string)
	for _, f := range r.forms {
		buttons, _ := f.Data["buttons"].([]any)
		// newer forms keep their buttons among "elements"
		elements, _ := f.Data["elements"].([]any)
		for _, b := range append(buttons, elements...) {
			bm, _ := b.(map[string]any)
			img, _ := bm["image"].(map[string]any)
			if img == nil || img["type"] != "url" {
				continue
			}
			u, _ := img["data"].(string)
			if p, ok := ImagePath(u); ok {
				images[u] = p
			}
		}
	}
	return images
}
