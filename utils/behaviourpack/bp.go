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
	Manifest *resource.Manifest
	blocks   map[string]*BlockBehaviour
	items    map[string]*itemBehaviour
	entities map[string]*entityBehaviour
	biomes   []biomeBehaviour

	// script is a Script API entry point (scripts/main.js) when set.
	script string
}

// SetScript puts a Script API script in the pack as scripts/main.js, run
// with @minecraft/server and @minecraft/server-ui.
func (bp *Pack) SetScript(source string) {
	bp.script = source
}

func (bp *Pack) HasScript() bool {
	return bp.script != ""
}

func New(name string) *Pack {
	return &Pack{
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
	return bp.HasBlocks() || bp.HasItems() || bp.HasScript()
}

// writeScriptManifest writes the manifest with a script module, which the
// resource.Manifest type has no fields for (entry, module_name).
func (bp *Pack) writeScriptManifest(fs utils.WriterFS) error {
	m := bp.Manifest
	modules := []map[string]any{}
	for _, mod := range m.Modules {
		modules = append(modules, map[string]any{
			"type":        mod.Type,
			"uuid":        mod.UUID,
			"description": mod.Description,
			"version":     mod.Version,
		})
	}
	modules = append(modules, map[string]any{
		"type":        "script",
		"language":    "javascript",
		"uuid":        utils.RandSeededUUID(m.Header.Name + "_script_module"),
		"description": "Menus and scoreboards recorded by bedrocktool",
		"version":     []int{1, 0, 0},
		"entry":       "scripts/main.js",
	})
	deps := []map[string]any{}
	for _, d := range m.Dependencies {
		deps = append(deps, map[string]any{"uuid": d.UUID, "version": d.Version})
	}
	deps = append(deps,
		map[string]any{"module_name": "@minecraft/server", "version": "1.16.0"},
		map[string]any{"module_name": "@minecraft/server-ui", "version": "1.3.0"},
	)
	header := m.Header
	if v := header.MinimumGameVersion; v[0] < 1 || (v[0] == 1 && (v[1] < 21 || (v[1] == 21 && v[2] < 50))) {
		header.MinimumGameVersion = resource.Version{1, 21, 50}
	}
	out := map[string]any{
		"format_version": 2,
		"header":         header,
		"modules":        modules,
		"dependencies":   deps,
	}
	w, err := fs.Create("manifest.json")
	if err != nil {
		return err
	}
	defer w.Close()
	e := json.NewEncoder(w)
	e.SetIndent("", "	")
	return e.Encode(out)
}

func splitNamespace(identifier string) (ns, name string) {
	split := strings.SplitN(identifier, ":", 2)
	return split[0], split[len(split)-1]
}

func (bp *Pack) Save(fs utils.WriterFS) error {
	if bp.HasScript() {
		if err := bp.writeScriptManifest(fs); err != nil {
			return err
		}
		w, err := fs.Create("scripts/main.js")
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(bp.script))
		w.Close()
		if err != nil {
			return err
		}
	} else if err := utils.WriteManifest(bp.Manifest, fs, ""); err != nil {
		return err
	}

	_add_thing := func(base, identifier string, thing any) error {
		ns, name := splitNamespace(identifier)
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
		for _, be := range bp.blocks {
			err := _add_thing("blocks", be.MinecraftBlock.Description.Identifier, be)
			if err != nil {
				return err
			}
		}
	}
	if bp.HasItems() { // items
		for _, ib := range bp.items {
			err := _add_thing("items", ib.MinecraftItem.Description.Identifier, ib)
			if err != nil {
				return err
			}
		}
	}
	if bp.HasEntities() { // entities
		for _, eb := range bp.entities {
			err := _add_thing("entities", eb.MinecraftEntity.Description.Identifier, eb)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
