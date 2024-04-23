package behaviourpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Pack struct {
	formatVersion string
	Manifest      *resource.Manifest
	blocks        map[string]*blockBehaviour
	items         map[string]*itemBehaviour
	entities      map[string]*entityBehaviour
	biomes        []biomeBehaviour
}

func New(name string) *Pack {
	return &Pack{
		formatVersion: "1.16.0",
		Manifest: &resource.Manifest{
			FormatVersion: 2,
			Header: resource.Header{
				Name:               name,
				Description:        "Adds Blocks, Items and Entities from the server to this world",
				UUID:               utils.RandSeededUUID(name + "_datapack"),
				Version:            [3]int{1, 0, 0},
				MinimumGameVersion: [3]int{1, 19, 50},
			},
			Modules: []resource.Module{
				{
					Type:        "data",
					UUID:        utils.RandSeededUUID(name + "_data_module"),
					Description: "Datapack",
					Version:     [3]int{1, 0, 0},
				},
			},
			Dependencies: []resource.Dependency{},
			Capabilities: []resource.Capability{},
		},
		blocks:   make(map[string]*blockBehaviour),
		items:    make(map[string]*itemBehaviour),
		entities: make(map[string]*entityBehaviour),
	}
}

func (bp *Pack) AddDependency(id string, ver [3]int) {
	bp.Manifest.Dependencies = append(bp.Manifest.Dependencies, resource.Dependency{
		UUID:    id,
		Version: ver,
	})
}

func (bp *Pack) CheckAddLink(pack utils.Pack) {
	_, names, err := pack.FS()
	if err != nil {
		logrus.Error(err)
		return
	}

	var hasBlocksJson = false
	if bp.HasBlocks() {
		_, hasBlocksJson = slices.BinarySearch(names, "blocks.json")
		if err == nil {
			hasBlocksJson = true
		}
	}

	var hasEntitiesFolder = false
	if bp.HasEntities() {
		_, hasEntitiesFolder = slices.BinarySearch(names, "entity")
	}

	var hasItemsFolder = false
	if bp.HasItems() {
		_, hasItemsFolder = slices.BinarySearch(names, "items")
	}

	// has no assets needed
	if !(hasBlocksJson || hasEntitiesFolder || hasItemsFolder) {
		return
	}

	h := pack.Base().Manifest().Header
	bp.AddDependency(h.UUID, h.Version)
}

func (bp *Pack) HasBlocks() bool {
	return len(bp.blocks) > 0
}

func (bp *Pack) HasItems() bool {
	return len(bp.items) > 0
}

func (bp *Pack) HasEntities() bool {
	return len(bp.entities) > 0
}

func (bp *Pack) HasContent() bool {
	return bp.HasBlocks() || bp.HasItems()
}

func ns_name_split(identifier string) (ns, name string) {
	ns_name := strings.Split(identifier, ":")
	return ns_name[0], ns_name[len(ns_name)-1]
}

func (bp *Pack) Save(fpath string) error {
	if err := utils.WriteManifest(bp.Manifest, fpath); err != nil {
		return err
	}

	_add_thing := func(base, identifier string, thing any) error {
		ns, name := ns_name_split(identifier)
		dir := filepath.Join(base, ns)
		_ = os.Mkdir(dir, 0o755)
		w, err := os.Create(filepath.Join(dir, name+".json"))
		if err != nil {
			return err
		}
		e := json.NewEncoder(w)
		e.SetIndent("", "\t")
		return e.Encode(thing)
	}

	for k := range bp.items {
		_, ok := bp.blocks[k]
		if ok {
			delete(bp.items, k)
		}
	}

	if bp.HasBlocks() { // blocks
		blocksDir := filepath.Join(fpath, "blocks")
		_ = os.Mkdir(blocksDir, 0o755)
		for _, be := range bp.blocks {
			err := _add_thing(blocksDir, be.MinecraftBlock.Description.Identifier, be)
			if err != nil {
				return err
			}
		}
	}
	if bp.HasItems() { // items
		itemsDir := filepath.Join(fpath, "items")
		_ = os.Mkdir(itemsDir, 0o755)
		for _, ib := range bp.items {
			err := _add_thing(itemsDir, ib.MinecraftItem.Description.Identifier, ib)
			if err != nil {
				return err
			}
		}
	}
	if bp.HasEntities() { // entities
		entitiesDir := filepath.Join(fpath, "entities")
		_ = os.Mkdir(entitiesDir, 0o755)
		for _, eb := range bp.entities {
			err := _add_thing(entitiesDir, eb.MinecraftEntity.Description.Identifier, eb)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
