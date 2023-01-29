package behaviourpack

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type BehaviourPack struct {
	formatVersion string
	Manifest      *resource.Manifest
	blocks        []blockBehaviour
	items         map[string]itemBehaviour
}

type blockBehaviour struct {
	FormatVersion  string               `json:"format_version"`
	MinecraftBlock world.MinecraftBlock `json:"minecraft:block"`
}

type itemDescription struct {
	Category       string `json:"category"`
	Identifier     string `json:"identifier"`
	IsExperimental bool   `json:"is_experimental"`
}

type minecraftItem struct {
	Description itemDescription `json:"description"`
	Components  map[string]any  `json:"components,omitempty"`
}

type itemBehaviour struct {
	FormatVersion string        `json:"format_version"`
	MinecraftItem minecraftItem `json:"minecraft:item"`
}

func check(err error) {
	if err != nil {
		logrus.Fatal(err)
	}
}

func rand_seeded_uuid(str string) string {
	h := sha256.Sum256([]byte(str))
	id, _ := uuid.NewRandomFromReader(bytes.NewBuffer(h[:]))
	return id.String()
}

func New(name string) *BehaviourPack {
	return &BehaviourPack{
		formatVersion: "1.16.0",
		Manifest: &resource.Manifest{
			FormatVersion: 2,
			Header: resource.Header{
				Name:               "pack.name",
				Description:        "pack.description",
				UUID:               rand_seeded_uuid(name + "_datapack"),
				Version:            [3]int{1, 0, 0},
				MinimumGameVersion: [3]int{1, 19, 50},
			},
			Modules: []resource.Module{
				{
					Type:    "data",
					UUID:    rand_seeded_uuid(name + "_data_module"),
					Version: [3]int{1, 0, 0},
				},
			},
			Dependencies: []resource.Dependency{},
			Capabilities: []resource.Capability{},
		},
		blocks: []blockBehaviour{},
		items:  make(map[string]itemBehaviour),
	}
}

func (bp *BehaviourPack) AddDependency(id string, ver [3]int) {
	bp.Manifest.Dependencies = append(bp.Manifest.Dependencies, resource.Dependency{
		UUID:    id,
		Version: ver,
	})
}

func (bp *BehaviourPack) AddBlock(block protocol.BlockEntry) {
	entry := blockBehaviour{
		FormatVersion:  bp.formatVersion,
		MinecraftBlock: world.ParseBlock(block),
	}

	bp.blocks = append(bp.blocks, entry)
}

func (bp *BehaviourPack) AddItem(item protocol.ItemEntry) {
	entry := itemBehaviour{
		FormatVersion: bp.formatVersion,
		MinecraftItem: minecraftItem{
			Description: itemDescription{
				Identifier:     item.Name,
				IsExperimental: true,
			},
			Components: make(map[string]any),
		},
	}
	bp.items[item.Name] = entry
}

func (bp *BehaviourPack) ApplyComponentEntries(entries []protocol.ItemComponentEntry) {
	for _, ice := range entries {
		item, ok := bp.items[ice.Name]
		if !ok {
			continue
		}
		item.MinecraftItem.Components = ice.Data
	}
}

func (bp *BehaviourPack) CheckAddLink(pack utils.Pack) {
	z, err := zip.NewReader(pack, int64(pack.Len()))
	if err != nil {
		logrus.Error(err)
		return
	}
	_, err = z.Open("blocks.json")
	if err != nil {
		return
	}
	h := pack.Manifest().Header
	bp.AddDependency(h.UUID, h.Version)
}

func (bp *BehaviourPack) HasContent() bool {
	return len(bp.blocks) > 0 || len(bp.items) > 0
}

func (bp *BehaviourPack) Save(fpath string) error {
	{ // write manifest
		w, err := os.Create(path.Join(fpath, "manifest.json"))
		if err != nil {
			return err
		}
		e := json.NewEncoder(w)
		e.SetIndent("", "\t")
		check(e.Encode(bp.Manifest))
	}
	if len(bp.blocks) > 0 { // blocks
		blocks_dir := path.Join(fpath, "blocks")
		os.Mkdir(blocks_dir, 0o755)
		for _, be := range bp.blocks {
			ns_name := strings.Split(be.MinecraftBlock.Description.Identifier, ":")
			ns := ns_name[0]
			name := ns_name[len(ns_name)-1]
			block_dir := path.Join(blocks_dir, ns)
			os.Mkdir(block_dir, 0o755)
			w, err := os.Create(path.Join(block_dir, name+".json"))
			if err != nil {
				return err
			}
			e := json.NewEncoder(w)
			e.SetIndent("", "\t")
			check(e.Encode(be))
		}
	}
	if len(bp.items) > 0 { // items
		items_dir := path.Join(fpath, "items")
		os.Mkdir(items_dir, 0o755)
		for _, ib := range bp.items {
			ns_name := strings.Split(ib.MinecraftItem.Description.Identifier, ":")
			ns := ns_name[0]
			name := ns_name[len(ns_name)-1]
			item_dir := path.Join(items_dir, ns)
			os.Mkdir(item_dir, 0o755)
			w, err := os.Create(path.Join(item_dir, name+".json"))
			if err != nil {
				return err
			}
			e := json.NewEncoder(w)
			e.SetIndent("", "\t")
			check(e.Encode(ib))
		}
	}
	return nil
}
