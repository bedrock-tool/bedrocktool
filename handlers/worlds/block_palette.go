package worlds

import (
	"bytes"
	_ "embed"
	"fmt"
	"reflect"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// Vanilla palette from df-mc/dragonfly 4c7b5074be94fa83a1cd98e9c752083ad04a6e21
// (Minecraft 1.26.50). Retain older implementation states only in hashed mode:
// their local indices never need to match wire indices, and their semantics are
// useful to the renderer. All current states, including unimplemented blocks,
// retain their exact names and properties for disk export.
//
//go:embed block_states_1_26_50.nbt
var networkBlockStates []byte

func newNetworkBlockRegistry(hashed bool) (world.BlockRegistry, error) {
	if !hashed {
		return nil, fmt.Errorf("non-hashed block network IDs are unsupported for Minecraft 1.26.50: exact wire palette order is not implemented")
	}
	br := world.NewBlockRegistry()
	base := world.NewBlockRegistry()
	base.Finalize()
	existing := make(map[string][]map[string]any)
	for _, b := range base.Blocks() {
		name, props := b.EncodeBlock()
		normalized := make(map[string]any, len(props))
		for k, v := range props {
			if flag, ok := v.(bool); ok {
				if flag {
					v = uint8(1)
				} else {
					v = uint8(0)
				}
			}
			normalized[k] = v
		}
		props = normalized
		existing[name] = append(existing[name], props)
	}
	buf := bytes.NewBuffer(networkBlockStates)
	dec := nbt.NewDecoder(buf)
	for buf.Len() > 0 {
		var s world.BlockState
		if err := dec.Decode(&s); err != nil {
			return nil, fmt.Errorf("decode 1.26.50 vanilla palette: %w", err)
		}
		found := false
		for _, props := range existing[s.Name] {
			if (len(props) == 0 && len(s.Properties) == 0) || reflect.DeepEqual(props, s.Properties) {
				found = true
				break
			}
		}
		if !found {
			br.RegisterBlockState(s)
			existing[s.Name] = append(existing[s.Name], s.Properties)
		}
	}
	return br, nil
}
