package behaviourpack

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path"
	"strings"

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

type minecraftBlock struct {
	Description description            `json:"description"`
	Components  map[string]interface{} `json:"components"`
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
				MinimumGameVersion: [3]int{1, 16, 0},
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

func (bp *BehaviourPack) AddBlock(block protocol.BlockEntry) {
	entry := blockBehaviour{
		FormatVersion: "1.12.0",
		MinecraftBlock: minecraftBlock{
			Description: description{
				Identifier:             block.Name,
				IsExperimental:         false,
				RegisterToCreativeMenu: true,
			},
			Components: block.Properties,
		},
	}
	bp.blocks = append(bp.blocks, entry)
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
