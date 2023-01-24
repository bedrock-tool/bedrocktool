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
	Manifest *resource.Manifest
	blocks   []blockBehaviour
}

type blockBehaviour struct {
	FormatVersion  string               `json:"format_version"`
	MinecraftBlock world.MinecraftBlock `json:"minecraft:block"`
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
		FormatVersion:  "1.16.0",
		MinecraftBlock: world.ParseBlock(block),
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
