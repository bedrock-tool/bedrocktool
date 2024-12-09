package behaviourpack

import (
	"encoding/json"
	"io/fs"
	"path"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type Pack struct {
	formatVersion string
	Manifest      *resource.Manifest
	blocks        map[string]*BlockBehaviour
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
				UUID:               uuid.MustParse(utils.RandSeededUUID(name + "_datapack")),
				Version:            resource.Version{1, 0, 0},
				MinimumGameVersion: resource.Version{1, 19, 50},
			},
			Modules: []resource.Module{
				{
					Type:        "data",
					UUID:        utils.RandSeededUUID(name + "_data_module"),
					Description: "Datapack",
					Version:     resource.Version{1, 0, 0},
				},
			},
			Dependencies: []resource.Dependency{},
			Capabilities: []resource.Capability{},
		},
		blocks:   make(map[string]*BlockBehaviour),
		items:    make(map[string]*itemBehaviour),
		entities: make(map[string]*entityBehaviour),
	}
}

func (bp *Pack) AddDependency(id string, ver resource.Version) {
	bp.Manifest.Dependencies = append(bp.Manifest.Dependencies, resource.Dependency{
		UUID:    id,
		Version: ver,
	})
}

func fsFileExists(f fs.FS, name string) bool {
	file, err := f.Open(name)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func (bp *Pack) CheckAddLink(pack resource.Pack) {
	hasBlocksJson := bp.HasBlocks() && fsFileExists(pack, "blocks.json")
	hasEntitiesFolder := bp.HasEntities() && fsFileExists(pack, "entity")
	hasItemsFolder := bp.HasItems() && fsFileExists(pack, "items")

	// has no assets needed
	if !(hasBlocksJson || hasEntitiesFolder || hasItemsFolder) {
		return
	}

	h := pack.Manifest().Header
	bp.AddDependency(h.UUID.String(), h.Version)
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

func (bp *Pack) Save(fs utils.WriterFS, fpath string) error {
	if err := utils.WriteManifest(bp.Manifest, fs, fpath); err != nil {
		return err
	}

	_add_thing := func(base, identifier string, thing any) error {
		ns, name := ns_name_split(identifier)
		dir := path.Join(base, ns)
		w, err := fs.Create(path.Join(dir, name+".json"))
		if err != nil {
			return err
		}
		defer w.Close()
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
		blocksDir := path.Join(fpath, "blocks")
		for _, be := range bp.blocks {
			err := _add_thing(blocksDir, be.MinecraftBlock.Description.Identifier, be)
			if err != nil {
				return err
			}
		}
	}
	if bp.HasItems() { // items
		itemsDir := path.Join(fpath, "items")
		for _, ib := range bp.items {
			err := _add_thing(itemsDir, ib.MinecraftItem.Description.Identifier, ib)
			if err != nil {
				return err
			}
		}
	}
	if bp.HasEntities() { // entities
		entitiesDir := path.Join(fpath, "entities")
		for _, eb := range bp.entities {
			err := _add_thing(entitiesDir, eb.MinecraftEntity.Description.Identifier, eb)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
