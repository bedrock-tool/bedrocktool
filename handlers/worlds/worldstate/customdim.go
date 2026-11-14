package worldstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// customDimension is a custom dimension of a world being read, so dragonfly
// can load its chunks.
type customDimension struct {
	name string
	r    cube.Range
}

func (d customDimension) Range() cube.Range               { return d.r }
func (customDimension) WaterEvaporates() bool             { return false }
func (customDimension) LavaSpreadDuration() time.Duration { return time.Second * 3 / 2 }
func (customDimension) WeatherCycle() bool                { return false }
func (customDimension) TimeCycle() bool                   { return false }
func (d customDimension) String() string                  { return d.name }

var (
	registeredDimsMu sync.Mutex
	// registeredDims are the custom dimensions registered with dragonfly, by id.
	registeredDims = map[int]world.Dimension{}
)

// WorldDimension is a dimension a saved world has chunks in.
type WorldDimension struct {
	ID         int
	Identifier string
	Dimension  world.Dimension
}

// RegisterWorldDimensions registers the custom dimensions of the world in
// folder with dragonfly (its registry is for the whole program), so its
// chunks can be read, and returns the world's dimensions: the overworld,
// nether and end, and the custom ones by their id.
//
// A custom dimension's height is read from a dimensions/*.json in the
// world's behavior packs; -64 to 319 otherwise, as the game does.
func RegisterWorldDimensions(folder string) ([]WorldDimension, error) {
	dims := []WorldDimension{
		{ID: 0, Identifier: "minecraft:overworld", Dimension: world.Overworld},
		{ID: 1, Identifier: "minecraft:nether", Dimension: world.Nether},
		{ID: 2, Identifier: "minecraft:the_end", Dimension: world.End},
	}
	table, err := ReadDimensionTable(folder)
	if err != nil {
		return nil, err
	}
	if len(table) == 0 {
		return dims, nil
	}
	heights := readDimensionHeights(folder)

	idents := make([]string, 0, len(table))
	for ident := range table {
		idents = append(idents, ident)
	}
	sort.Slice(idents, func(i, j int) bool { return table[idents[i]] < table[idents[j]] })

	registeredDimsMu.Lock()
	defer registeredDimsMu.Unlock()
	for _, ident := range idents {
		id := table[ident]
		r, ok := heights[ident]
		if !ok {
			r = cube.Range{-64, 319}
		}
		dim, ok := registeredDims[id]
		if !ok {
			d := customDimension{name: ident, r: r}
			if err := world.RegisterDimension(id, fmt.Sprintf("%s#%d", ident, id), d); err != nil {
				return nil, fmt.Errorf("dimension %s (%d): %w", ident, id, err)
			}
			registeredDims[id] = d
			dim = d
		} else if d, _ := dim.(customDimension); d.name != ident || d.r != r {
			// an earlier world had another dimension with this id
			return nil, fmt.Errorf("dimension id %d is %s in an earlier world, %s here", id, d.name, ident)
		}
		dims = append(dims, WorldDimension{ID: id, Identifier: ident, Dimension: dim})
	}
	return dims, nil
}

// readDimensionHeights reads minecraft:dimension_height from the
// dimensions/*.json of a world's behavior packs.
func readDimensionHeights(folder string) map[string]cube.Range {
	out := map[string]cube.Range{}
	files, _ := filepath.Glob(filepath.Join(folder, "behavior_packs", "*", "dimensions", "*.json"))
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var def struct {
			Dimension struct {
				Description struct {
					Identifier string `json:"identifier"`
				} `json:"description"`
				Components struct {
					Height *struct {
						MinY        int `json:"min_y"`
						HeightRange int `json:"height_range"`
					} `json:"minecraft:dimension_height"`
				} `json:"components"`
			} `json:"minecraft:dimension"`
		}
		if json.Unmarshal(b, &def) != nil {
			continue
		}
		h := def.Dimension.Components.Height
		if id := def.Dimension.Description.Identifier; id != "" && h != nil && h.HeightRange >= 16 {
			out[id] = cube.Range{h.MinY, h.MinY + h.HeightRange - 1}
		}
	}
	return out
}

// FindDimension finds a dimension by its name: overworld, nether, end,
// a custom dimension's identifier (bedrocktool:world_2) or the name
// without its namespace (world_2), or its id.
func FindDimension(dims []WorldDimension, name string) (WorldDimension, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "end", "the_end":
		name = "minecraft:the_end"
	case "overworld", "nether":
		name = "minecraft:" + name
	}
	for _, d := range dims {
		_, short, _ := strings.Cut(d.Identifier, ":")
		if d.Identifier == name || short == name || fmt.Sprint(d.ID) == name {
			return d, true
		}
	}
	return WorldDimension{}, false
}
