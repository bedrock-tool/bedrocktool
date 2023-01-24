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
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type BehaviourPack struct {
	Manifest *resource.Manifest
	blocks   []blockBehaviour
}

type description struct {
	Identifier             string `json:"identifier"`
	IsExperimental         bool   `json:"is_experimental"`
	RegisterToCreativeMenu bool   `json:"register_to_creative_menu"`
}

type menu_category struct {
	Category string `json:"category"`
	Group    string `json:"group"`
}

func menu_category_from_map(in map[string]any) menu_category {
	return menu_category{
		Category: in["category"].(string),
		Group:    in["group"].(string),
	}
}

type permutation struct {
	Components map[string]any `json:"components"`
	Condition  string         `json:"condition"`
}

func permutations_from_list(in []map[string]any) (out []permutation) {
	for _, v := range in {
		out = append(out, permutation{
			Components: v["components"].(map[string]any),
			Condition:  v["condition"].(string),
		})
	}
	return
}

type property struct {
	Enum []any  `json:"enum"`
	Name string `json:"name"`
}

func properties_from_list(in []map[string]any) (out []property) {
	for _, v := range in {
		out = append(out, property{
			Enum: v["enum"].([]any),
			Name: v["name"].(string),
		})
	}
	return
}

type minecraftBlock struct {
	Description  description    `json:"description"`
	Components   map[string]any `json:"components,omitempty"`
	MenuCategory menu_category  `json:"menu_category,omitempty"`
	Permutations []permutation  `json:"permutations,omitempty"`
	Properties   []property     `json:"properties,omitempty"`
}

type blockBehaviour struct {
	FormatVersion  string         `json:"format_version"`
	MinecraftBlock minecraftBlock `json:"minecraft:block"`
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
		FormatVersion: "1.16.0",
		MinecraftBlock: minecraftBlock{
			Description: description{
				Identifier:             block.Name,
				IsExperimental:         true,
				RegisterToCreativeMenu: true,
			},
		},
	}

	v2 := false

	if perms, ok := block.Properties["permutations"].([]map[string]any); ok {
		entry.MinecraftBlock.Permutations = permutations_from_list(perms)
		v2 = true
	}

	if comps, ok := block.Properties["components"].(map[string]any); ok {
		delete(comps, "minecraft:creative_category")

		for k, v := range comps {
			if v, ok := v.(map[string]any); ok {
				// fix {"value": 0.1} -> 0.1
				if v, ok := v["value"]; ok {
					comps[k] = v
				}
				// fix {"lightLevel": 15} -> 15
				if v, ok := v["lightLevel"]; ok {
					comps[k] = v
				}
				// fix missing * instance
				if k == "minecraft:material_instances" {
					comps[k] = v["materials"].(map[string]any)
				}
			}
		}
		entry.MinecraftBlock.Components = comps
		v2 = true
	}

	if menu, ok := block.Properties["menu_category"].(map[string]any); ok {
		entry.MinecraftBlock.MenuCategory = menu_category_from_map(menu)
		v2 = true
	}
	if props, ok := block.Properties["properties"].([]map[string]any); ok {
		entry.MinecraftBlock.Properties = properties_from_list(props)
		v2 = true
	}
	if !v2 {
		entry.MinecraftBlock.Components = block.Properties
	}

	bp.blocks = append(bp.blocks, entry)
}

func (bp *BehaviourPack) CheckAddLink(pack utils.Pack) {
	z, err := zip.NewReader(pack, int64(pack.Len()))
	if err != nil {
		logrus.Error(err)
	}
	_, err = z.Open("blocks.json")
	if err != nil {
		return
	}
	h := pack.Manifest().Header
	bp.AddDependency(h.UUID, h.Version)
}

func (bp *BehaviourPack) Save(fpath string) error {
	{ // write manifest
		w, err := os.Create(path.Join(fpath, "manifest.json"))
		if err != nil {
			return err
		}
		check(json.NewEncoder(w).Encode(bp.Manifest))
	}
	{ // blocks
		block_dir := path.Join(fpath, "blocks")
		os.Mkdir(block_dir, 0o755)
		for _, be := range bp.blocks {
			ns := strings.Split(be.MinecraftBlock.Description.Identifier, ":")
			name := ns[len(ns)-1]
			w, err := os.Create(path.Join(block_dir, name+".json"))
			if err != nil {
				return err
			}
			check(json.NewEncoder(w).Encode(be))
		}
	}
	return nil
}
