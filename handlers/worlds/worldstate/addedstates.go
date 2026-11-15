package worldstate

import (
	"maps"

	"github.com/df-mc/dragonfly/server/world"
)

// addedStatesRegistry is the block registry a saved world is written with.
// Blocks the behavior pack gave states the server never sent (cardinal
// rotation) are saved with them, at their default values, so they match the
// pack's definition. It reads them back without those states, as the server's
// registry knows the blocks.
type addedStatesRegistry struct {
	world.BlockRegistry
	added map[string]map[string]any
}

// WithAddedStates wraps reg so blocks in added are saved with their extra
// states. It returns reg as is when there are none. Chunks are written to
// disk with the registry they were decoded with, so chunks from the server
// must be decoded with this one for the states to be saved.
func WithAddedStates(reg world.BlockRegistry, added map[string]map[string]any) world.BlockRegistry {
	if reg == nil || len(added) == 0 {
		return reg
	}
	return addedStatesRegistry{BlockRegistry: reg, added: maps.Clone(added)}
}

func (r addedStatesRegistry) RuntimeIDToState(rid uint32) (string, map[string]any, bool) {
	name, props, ok := r.BlockRegistry.RuntimeIDToState(rid)
	extra, has := r.added[name]
	if !ok || !has {
		return name, props, ok
	}
	out := make(map[string]any, len(props)+len(extra))
	maps.Copy(out, props)
	for k, v := range extra {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	return name, out, true
}

func (r addedStatesRegistry) StateToRuntimeID(name string, props map[string]any) (uint32, bool) {
	if extra, has := r.added[name]; has {
		props = maps.Clone(props)
		for k := range extra {
			delete(props, k)
		}
	}
	return r.BlockRegistry.StateToRuntimeID(name, props)
}
