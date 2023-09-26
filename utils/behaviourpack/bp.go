package behaviourpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type BehaviourPack struct {
	formatVersion string
	Manifest      *resource.Manifest
	blocks        map[string]*blockBehaviour
	items         map[string]*itemBehaviour
	entities      map[string]*entityBehaviour
	biomes        []biomeBehaviour
}

func New(name string) *BehaviourPack {
	return &BehaviourPack{
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

func (bp *BehaviourPack) AddDependency(id string, ver [3]int) {
	bp.Manifest.Dependencies = append(bp.Manifest.Dependencies, resource.Dependency{
		UUID:    id,
		Version: ver,
	})
}

func (bp *BehaviourPack) CheckAddLink(pack utils.Pack) {
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

func (bp *BehaviourPack) HasBlocks() bool {
	return len(bp.blocks) > 0
}

func (bp *BehaviourPack) HasItems() bool {
	return len(bp.items) > 0
}

func (bp *BehaviourPack) HasEntities() bool {
	return len(bp.entities) > 0
}

func (bp *BehaviourPack) HasContent() bool {
	return bp.HasBlocks() || bp.HasItems()
}

func ns_name_split(identifier string) (ns, name string) {
	ns_name := strings.Split(identifier, ":")
	return ns_name[0], ns_name[len(ns_name)-1]
}

func (bp *BehaviourPack) Save(fpath string, blockStates []world.BlockState) error {
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

		var blockStatesJson = map[string][]any{}
		for _, be := range blockStates {
			if len(be.Properties) == 0 {
				continue
			}
			blockStatesJson[be.Name] = append(blockStatesJson[be.Name], be.Properties)
		}

		f, err := os.Create(filepath.Join(fpath, "blockstates.json"))
		if err != nil {
			return err
		}
		defer f.Close()
		e := json.NewEncoder(f)
		err = e.Encode(blockStatesJson)
		if err != nil {
			return err
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
