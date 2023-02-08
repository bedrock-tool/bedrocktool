package behaviourpack

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type BehaviourPack struct {
	formatVersion string
	Manifest      *resource.Manifest
	blocks        []blockBehaviour
	items         map[string]itemBehaviour
	entities      map[string]entityBehaviour
}

func New(name string) *BehaviourPack {
	return &BehaviourPack{
		formatVersion: "1.16.0",
		Manifest: &resource.Manifest{
			FormatVersion: 2,
			Header: resource.Header{
				Name:               "pack.name",
				Description:        "pack.description",
				UUID:               utils.RandSeededUUID(name + "_datapack"),
				Version:            [3]int{1, 0, 0},
				MinimumGameVersion: [3]int{1, 19, 50},
			},
			Modules: []resource.Module{
				{
					Type:    "data",
					UUID:    utils.RandSeededUUID(name + "_data_module"),
					Version: [3]int{1, 0, 0},
				},
			},
			Dependencies: []resource.Dependency{},
			Capabilities: []resource.Capability{},
		},
		blocks:   []blockBehaviour{},
		items:    make(map[string]itemBehaviour),
		entities: make(map[string]entityBehaviour),
	}
}

func (bp *BehaviourPack) AddDependency(id string, ver [3]int) {
	bp.Manifest.Dependencies = append(bp.Manifest.Dependencies, resource.Dependency{
		UUID:    id,
		Version: ver,
	})
}

func (bp *BehaviourPack) CheckAddLink(pack utils.Pack) {
	z, err := zip.NewReader(pack, int64(pack.Len()))
	if err != nil {
		logrus.Error(err)
		return
	}
	if len(bp.blocks) > 0 {
		_, err = z.Open("blocks.json")
		if err != nil {
			return
		}
	}
	h := pack.Manifest().Header
	bp.AddDependency(h.UUID, h.Version)
}

func (bp *BehaviourPack) HasBlocks() bool {
	return len(bp.blocks) > 0
}

func (bp *BehaviourPack) HasItems() bool {
	return len(bp.items) > 0
}

func (bp *BehaviourPack) HasContent() bool {
	return bp.HasBlocks() || bp.HasItems()
}

func ns_name_split(identifier string) (ns, name string) {
	ns_name := strings.Split(identifier, ":")
	return ns_name[0], ns_name[len(ns_name)-1]
}

func (bp *BehaviourPack) Save(fpath string) error {
	{ // write manifest
		w, err := os.Create(path.Join(fpath, "manifest.json"))
		if err != nil {
			return err
		}
		e := json.NewEncoder(w)
		e.SetIndent("", "\t")
		if err = e.Encode(bp.Manifest); err != nil {
			return err
		}
	}

	_add_thing := func(base, identifier string, thing any) error {
		ns, name := ns_name_split(identifier)
		thing_dir := path.Join(base, ns)
		os.Mkdir(thing_dir, 0o755)
		w, err := os.Create(path.Join(thing_dir, name+".json"))
		if err != nil {
			return err
		}
		e := json.NewEncoder(w)
		e.SetIndent("", "\t")
		return e.Encode(thing)
	}

	if len(bp.blocks) > 0 { // blocks
		blocks_dir := path.Join(fpath, "blocks")
		os.Mkdir(blocks_dir, 0o755)
		for _, be := range bp.blocks {
			err := _add_thing(blocks_dir, be.MinecraftBlock.Description.Identifier, be)
			if err != nil {
				return err
			}
		}
	}
	if len(bp.items) > 0 { // items
		items_dir := path.Join(fpath, "items")
		os.Mkdir(items_dir, 0o755)
		for _, ib := range bp.items {
			err := _add_thing(items_dir, ib.MinecraftItem.Description.Identifier, ib)
			if err != nil {
				return err
			}
		}
	}
	if len(bp.entities) > 0 { // items
		items_dir := path.Join(fpath, "entities")
		os.Mkdir(items_dir, 0o755)
		for _, eb := range bp.entities {
			err := _add_thing(items_dir, eb.MinecraftEntity.Description.Identifier, eb)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
