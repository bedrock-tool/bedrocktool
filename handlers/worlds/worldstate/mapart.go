package worldstate

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Maps in item frames (Hive draws avatars on them) and in containers.
//
// CAPTURE: whenever a chunk is stored, its block entities are scanned for
// item frames and containers holding a filled map; a map not seen before is
// reported through OnMapSeen, so the handler can ask the server for its
// pixels (MapInfoRequest), which arrive as ClientBoundMapItemData.
//
// SAVE: every map's pixels are written under map_<id> (locked, so the game
// never redraws them from the saved world).
//
// EXPORT (ExportMapArt): each map as tiles/<id>.png, and every wall of two
// or more touching frames facing the same way stitched into one picture,
// laid out from the frames' facing_direction as you'd see it standing in
// front of the wall.

const mapSize = 128

// MapFrame is an item frame holding a map.
type MapFrame struct {
	Pos    cube.Pos
	MapID  int64
	Facing int32 // facing_direction: 0 down, 1 up, 2 north, 3 south, 4 west, 5 east; -1 unknown
}

type chunkMaps struct {
	frames []MapFrame
	loose  []int64 // maps in chests, barrels...
}

// NewMap is an empty map with the given ID.
func NewMap(id int64) *Map {
	return &Map{
		MapID:         id,
		ParentMapId:   -1,
		Width:         mapSize,
		Height:        mapSize,
		Decorations:   []any{},
		FullyExplored: true,
		MapLocked:     true,
	}
}

// ApplyMapPacket returns m (or a new map when m is nil) with pk's pixels
// drawn on it. m itself is left as it was, so a map being saved elsewhere
// isn't written to.
func ApplyMapPacket(m *Map, pk *packet.ClientBoundMapItemData) *Map {
	var out *Map
	if m == nil {
		out = NewMap(pk.MapID)
	} else {
		c := *m
		out = &c
	}
	out.Dimension = pk.Dimension
	out.XCenter = pk.Origin.X()
	out.ZCenter = pk.Origin.Z()
	if scale, ok := pk.Scale.Value(); ok {
		out.Scale = scale
	}
	pixels, ok := pk.Pixels.Value()
	if !ok {
		return out
	}
	width, _ := pk.Width.Value()
	height, _ := pk.Height.Value()
	xOff, _ := pk.XOffset.Value()
	yOff, _ := pk.YOffset.Value()
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			i := int(y*width + x)
			px, py := x+xOff, y+yOff
			if i >= len(pixels) || px < 0 || py < 0 || px >= mapSize || py >= mapSize {
				continue
			}
			c := pixels[i]
			o := int(py*mapSize+px) * 4
			out.Colors[o], out.Colors[o+1], out.Colors[o+2], out.Colors[o+3] = c.R, c.G, c.B, c.A
		}
	}
	return out
}

// mapUUID is the map an item NBT holds, if it is a filled map.
func mapUUID(v any) (int64, bool) {
	item, ok := v.(map[string]any)
	if !ok {
		return 0, false
	}
	if name, ok := item["Name"].(string); ok && name != "minecraft:filled_map" && name != "minecraft:map" {
		return 0, false
	}
	tag, ok := item["tag"].(map[string]any)
	if !ok {
		return 0, false
	}
	switch id := tag["map_uuid"].(type) {
	case int64:
		return id, true
	case int32:
		return int64(id), true
	}
	return 0, false
}

func nbtList(v any) []any {
	switch l := v.(type) {
	case []any:
		return l
	case []map[string]any:
		out := make([]any, len(l))
		for i, m := range l {
			out[i] = m
		}
		return out
	}
	return nil
}

// scanMapsLocked notes the maps a chunk's block entities hold. Called with
// stateLock held.
func (w *World) scanMapsLocked(pos world.ChunkPos, ch *Chunk) {
	cm := &chunkMaps{}
	for p, be := range ch.BlockEntities {
		switch id, _ := be["id"].(string); id {
		case "ItemFrame", "GlowItemFrame":
			if mid, ok := mapUUID(be["Item"]); ok {
				cm.frames = append(cm.frames, MapFrame{Pos: p, MapID: mid, Facing: w.frameFacing(ch, p)})
			}
		default:
			for _, it := range nbtList(be["Items"]) {
				if mid, ok := mapUUID(it); ok {
					cm.loose = append(cm.loose, mid)
				}
			}
		}
	}
	if len(cm.frames) == 0 && len(cm.loose) == 0 {
		delete(w.mapRefs, pos)
		return
	}
	w.mapRefs[pos] = cm
	note := func(id int64) {
		if _, seen := w.seenMaps[id]; seen {
			return
		}
		w.seenMaps[id] = struct{}{}
		if w.OnMapSeen != nil {
			w.OnMapSeen(id)
		}
	}
	for _, f := range cm.frames {
		note(f.MapID)
	}
	for _, id := range cm.loose {
		note(id)
	}
}

func (w *World) frameFacing(ch *Chunk, p cube.Pos) int32 {
	if w.BlockRegistry == nil || p.Y() < w.dimRange.Min() || p.Y() > w.dimRange.Max() {
		return -1
	}
	rid := ch.Block(uint8(p.X()&15), int16(p.Y()), uint8(p.Z()&15), 0)
	_, props, ok := w.BlockRegistry.RuntimeIDToState(rid)
	if !ok {
		return -1
	}
	switch f := props["facing_direction"].(type) {
	case int32:
		return f
	case uint8:
		return int32(f)
	}
	return -1
}

// referencedMapsLocked is every map the world's frames and containers hold.
func (w *World) referencedMapsLocked() map[int64]struct{} {
	out := make(map[int64]struct{})
	for _, cm := range w.mapRefs {
		for _, f := range cm.frames {
			out[f.MapID] = struct{}{}
		}
		for _, id := range cm.loose {
			out[id] = struct{}{}
		}
	}
	return out
}

// importMapsLocked adds the maps this world holds that arrived while an
// earlier world was being captured.
func (w *World) importMapsLocked(known map[int64]*Map) {
	for id := range w.referencedMapsLocked() {
		if _, ok := w.memState.maps[id]; ok {
			continue
		}
		if m, ok := known[id]; ok {
			w.memState.maps[id] = m
		}
	}
}

func mapImage(m *Map) *image.NRGBA {
	if m == nil {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, mapSize, mapSize))
	copy(img.Pix, m.Colors[:])
	// A blank map (one colour, or never received) is left out.
	first := [4]byte(img.Pix[:4])
	for i := 4; i < len(img.Pix); i += 4 {
		if [4]byte(img.Pix[i:i+4]) != first {
			return img
		}
	}
	return nil
}

// layout places a frame on its wall as seen from in front of it: column
// and row, each counting right and down.
func layout(f MapFrame) (col, row int) {
	x, y, z := f.Pos.X(), f.Pos.Y(), f.Pos.Z()
	switch f.Facing {
	case 2: // faces north: you look south, your right is west
		return -x, -y
	case 3: // faces south: you look north, your right is east
		return x, -y
	case 4: // faces west: you look east, your right is south
		return z, -y
	case 5: // faces east: you look west, your right is north
		return -z, -y
	case 1: // on the floor, north at the top
		return x, z
	case 0: // on the ceiling, seen from below, north at the top
		return x, -z
	}
	return 0, 0
}

// ExportMapArt writes the world's maps as PNGs into dir: tiles/<id>.png for
// each map, and grid_x_y_z.png for each wall of two or more maps. Returns
// how many of each it wrote.
func (w *World) ExportMapArt(dir string) (grids, tiles int, err error) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()

	images := make(map[int64]*image.NRGBA)
	for id := range w.referencedMapsLocked() {
		if img := mapImage(w.memState.maps[id]); img != nil {
			images[id] = img
		}
	}
	if len(images) == 0 {
		return 0, 0, nil
	}

	tileDir := filepath.Join(dir, "tiles")
	if err := os.MkdirAll(tileDir, 0o777); err != nil {
		return 0, 0, err
	}
	for id, img := range images {
		if err := writePNG(filepath.Join(tileDir, fmt.Sprintf("%d.png", id)), img); err != nil {
			return grids, tiles, err
		}
		tiles++
	}

	// Walls: frames facing the same way that touch, side by side.
	byPos := make(map[cube.Pos]MapFrame)
	for _, cm := range w.mapRefs {
		for _, f := range cm.frames {
			if images[f.MapID] != nil {
				byPos[f.Pos] = f
			}
		}
	}
	seen := make(map[cube.Pos]bool)
	positions := make([]cube.Pos, 0, len(byPos))
	for p := range byPos {
		positions = append(positions, p)
	}
	slices.SortFunc(positions, func(a, b cube.Pos) int {
		for i := range 3 {
			if a[i] != b[i] {
				return a[i] - b[i]
			}
		}
		return 0
	})
	for _, start := range positions {
		if seen[start] {
			continue
		}
		facing := byPos[start].Facing
		var wall []MapFrame
		queue := []cube.Pos{start}
		seen[start] = true
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			wall = append(wall, byPos[p])
			for _, d := range []cube.Pos{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
				n := p.Add(d)
				if f, ok := byPos[n]; ok && !seen[n] && f.Facing == facing {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
		if len(wall) < 2 {
			continue
		}
		minC, minR, maxC, maxR := 1<<31-1, 1<<31-1, -(1 << 31), -(1 << 31)
		for _, f := range wall {
			c, r := wallLayout(f, wall)
			minC, maxC = min(minC, c), max(maxC, c)
			minR, maxR = min(minR, r), max(maxR, r)
		}
		canvas := image.NewNRGBA(image.Rect(0, 0, (maxC-minC+1)*mapSize, (maxR-minR+1)*mapSize))
		for _, f := range wall {
			c, r := wallLayout(f, wall)
			at := image.Pt((c-minC)*mapSize, (r-minR)*mapSize)
			draw.Draw(canvas, image.Rectangle{Min: at, Max: at.Add(image.Pt(mapSize, mapSize))}, images[f.MapID], image.Point{}, draw.Src)
		}
		name := fmt.Sprintf("grid_x%d_y%d_z%d.png", start.X(), start.Y(), start.Z())
		if err := writePNG(filepath.Join(dir, name), canvas); err != nil {
			return grids, tiles, err
		}
		grids++
	}
	return grids, tiles, nil
}

// wallLayout is layout, falling back to the wall's own shape when the
// frames' facing isn't known: rows by height, columns along the axis the
// frames spread over.
func wallLayout(f MapFrame, wall []MapFrame) (col, row int) {
	if f.Facing >= 0 && f.Facing <= 5 {
		return layout(f)
	}
	xs, zs := map[int]bool{}, map[int]bool{}
	for _, o := range wall {
		xs[o.Pos.X()] = true
		zs[o.Pos.Z()] = true
	}
	if len(xs) >= len(zs) {
		return f.Pos.X(), -f.Pos.Y()
	}
	return f.Pos.Z(), -f.Pos.Y()
}

func writePNG(name string, img image.Image) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
