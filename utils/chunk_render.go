package utils

// chunk_render.go: the top-down map of chunks (the in-game map, the GUI's
// map, saved world images and the render command).
//
// Each column is its highest block's colour: the colour averaged from its
// top texture (the server's own for custom blocks), grass, leaves and water
// in their biome's colours. Water shows the ground under it, darker the
// deeper it is. Like a map item, the land is lit from the north: a block
// higher than the one north of it is lighter, lower darker, so hills and
// buildings stand out.

import (
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

// waterDepthMax is how deep water is looked through for the ground.
const waterDepthMax = 24

// noHeight is a column with nothing in it.
const noHeight = math.MinInt16

// The game's default tints, which grey textures are coloured with: the
// plains grass and foliage, and the default water.
var (
	plainsGrass   = color.RGBA{0x91, 0xbd, 0x59, 0xff}
	plainsFoliage = color.RGBA{0x77, 0xab, 0x2f, 0xff}
	birchFoliage  = color.RGBA{0x80, 0xa7, 0x55, 0xff}
	spruceFoliage = color.RGBA{0x61, 0x99, 0x61, 0xff}
	mangroveLeaf  = color.RGBA{0x8d, 0xb1, 0x27, 0xff}
	lilyPadTint   = color.RGBA{0x20, 0x80, 0x30, 0xff}
	defaultWater  = color.RGBA{0x44, 0xaf, 0xf5, 0xff}
	unknownColor  = color.RGBA{0x80, 0x80, 0x80, 0xff}
)

// The colour maps' corners: hot and wet, hot and dry, cold.
var (
	grassWet, grassDry, grassCold       = [3]float64{0x47, 0xcd, 0x33}, [3]float64{0xbf, 0xb7, 0x55}, [3]float64{0x80, 0xb4, 0x97}
	foliageWet, foliageDry, foliageCold = [3]float64{0x1a, 0xbf, 0x00}, [3]float64{0xae, 0xa4, 0x2a}, [3]float64{0x60, 0xa1, 0x7b}
)

type tintKind uint8

const (
	tintNone tintKind = iota
	tintGrass
	tintFoliage
	tintWater
)

// blockInfo is what painting needs of a block, worked out once.
type blockInfo struct {
	name  string
	color color.RGBA
	tint  tintKind
	skip  bool // drawn as nothing: air, structure voids, light blocks
	water bool
}

type biomeTints struct {
	grass, foliage, water color.RGBA
}

type ChunkRenderer struct {
	customBlockColors map[string]color.RGBA
	br                world.BlockRegistry

	// Biomes, when set, colours grass, leaves and water by biome.
	Biomes *world.BiomeRegistry

	mu     sync.Mutex
	blocks map[uint32]*blockInfo
	tints  map[uint32]biomeTints
	// edges are each rendered chunk's southmost row of heights, for
	// lighting the row north of it in the chunk south of it.
	edges map[world.ChunkPos][16]int16
}

func NewChunkRenderer(br world.BlockRegistry) *ChunkRenderer {
	return &ChunkRenderer{
		br:     br,
		blocks: make(map[uint32]*blockInfo),
		tints:  make(map[uint32]biomeTints),
		edges:  make(map[world.ChunkPos][16]int16),
	}
}

func (cr *ChunkRenderer) ResolveColors(entries []protocol.BlockEntry, packs []resource.Pack) {
	colors := ResolveColors(entries, packs)
	cr.mu.Lock()
	cr.customBlockColors = colors
	cr.blocks = make(map[uint32]*blockInfo)
	cr.mu.Unlock()
}

// Reset forgets the chunk edges kept for lighting.
func (cr *ChunkRenderer) Reset() {
	cr.mu.Lock()
	cr.edges = make(map[world.ChunkPos][16]int16)
	cr.mu.Unlock()
}

func shortName(name string) string {
	return name[strings.IndexByte(name, ':')+1:]
}

func tintOf(name string) tintKind {
	n := shortName(name)
	switch {
	case n == "water" || n == "flowing_water":
		return tintWater
	case n == "grass_block" || n == "grass" || n == "short_grass" || n == "tall_grass" || n == "tallgrass" ||
		n == "fern" || n == "large_fern" || n == "bush" || n == "sugar_cane" || n == "reeds":
		return tintGrass
	case strings.Contains(n, "birch_leaves"), strings.Contains(n, "spruce_leaves"), strings.Contains(n, "cherry_leaves"),
		strings.Contains(n, "azalea_leaves"), strings.Contains(n, "pale_oak_leaves"), strings.Contains(n, "mangrove_leaves"):
		return tintNone // their colour doesn't change with the biome
	case strings.Contains(n, "leaves"), n == "vine":
		return tintFoliage
	}
	return tintNone
}

// greyMax is how far apart a colour's channels may be for it to count as
// grey: a texture already coloured isn't tinted again.
const greyMax = 24

// foliageTint colours a grey-textured plant the way the game does by
// default, so grass and leaves come out green.
func foliageTint(name string, c color.RGBA) color.RGBA {
	if max(c.R, c.G, c.B)-min(c.R, c.G, c.B) > greyMax {
		return c
	}
	n := shortName(name)
	var tint color.RGBA
	switch {
	case strings.Contains(n, "birch_leaves"):
		tint = birchFoliage
	case strings.Contains(n, "spruce_leaves"):
		tint = spruceFoliage
	case strings.Contains(n, "mangrove_leaves"):
		tint = mangroveLeaf
	case strings.Contains(n, "leaves"), n == "vine":
		tint = plainsFoliage
	case n == "waterlily", n == "lily_pad":
		tint = lilyPadTint
	case n == "grass" || n == "grass_block" || n == "short_grass" || n == "tall_grass" || n == "tallgrass" ||
		n == "fern" || n == "large_fern" || n == "bush":
		tint = plainsGrass
	default:
		return c
	}
	mul := func(a, b uint8) uint8 { return uint8(uint32(a) * uint32(b) / 255) }
	return color.RGBA{mul(c.R, tint.R), mul(c.G, tint.G), mul(c.B, tint.B), c.A}
}

// nameColor is a block's colour by name: the server's texture for custom
// blocks, the vanilla table, the block's own map colour, or one made up
// from the name.
func (cr *ChunkRenderer) nameColor(name string, b world.Block, known bool) color.RGBA {
	if name == "minecraft:water" || name == "minecraft:flowing_water" {
		return defaultWater
	}
	if c, ok := cr.customBlockColors[name]; ok && c.A != 0 {
		return c
	}
	if c := LookupColor(name); c.A != 0 {
		return c
	}
	if known {
		if _, unknown := b.(world.UnknownBlock); !unknown {
			if c := b.Color(); c.A != 0 && c != (color.RGBA{0xff, 0, 0xff, 0xff}) {
				return c
			}
		}
	}
	if name == "" {
		return unknownColor
	}
	h := fnv.New32a()
	h.Write([]byte(name))
	v := h.Sum32()
	return color.RGBA{0x50 + uint8(v)%0x60, 0x50 + uint8(v>>8)%0x60, 0x50 + uint8(v>>16)%0x60, 0xff}
}

func (cr *ChunkRenderer) block(rid uint32) *blockInfo {
	if info, ok := cr.blocks[rid]; ok {
		return info
	}
	info := &blockInfo{}
	b, known := cr.br.BlockByRuntimeID(rid)
	if known {
		info.name, _ = b.EncodeBlock()
	} else if name, _, ok := cr.br.RuntimeIDToState(rid); ok {
		info.name = name
	}
	switch info.name {
	case "minecraft:air", "minecraft:structure_void", "minecraft:barrier":
		info.skip = true
	default:
		if strings.HasPrefix(info.name, "minecraft:light_block") {
			info.skip = true
		}
	}
	info.water = info.name == "minecraft:water" || info.name == "minecraft:flowing_water"
	if !info.skip {
		info.color = foliageTint(info.name, cr.nameColor(info.name, b, known))
		info.tint = tintOf(info.name)
	}
	cr.blocks[rid] = info
	return info
}

// colormap is a colour map's colour for a temperature and rainfall.
func colormap(temp, rain float64, wet, dry, cold [3]float64) color.RGBA {
	t := math.Max(0, math.Min(1, temp))
	r := math.Max(0, math.Min(1, rain)) * t
	wc, ww, wd := 1-t, r, t-r
	ch := func(i int) uint8 { return uint8(math.Round(wc*cold[i] + ww*wet[i] + wd*dry[i])) }
	return color.RGBA{ch(0), ch(1), ch(2), 0xff}
}

func (cr *ChunkRenderer) biomeTints(id uint32) (biomeTints, bool) {
	if t, ok := cr.tints[id]; ok {
		return t, true
	}
	if cr.Biomes == nil {
		return biomeTints{}, false
	}
	b, ok := cr.Biomes.BiomeByID(int(id))
	if !ok {
		return biomeTints{}, false
	}
	n := shortName(b.String())
	t := biomeTints{
		grass:   colormap(b.Temperature(), b.Rainfall(), grassWet, grassDry, grassCold),
		foliage: colormap(b.Temperature(), b.Rainfall(), foliageWet, foliageDry, foliageCold),
		water:   b.WaterColour(),
	}
	switch {
	case n == "mangrove_swamp":
		t.grass, t.foliage = color.RGBA{0x6a, 0x70, 0x39, 0xff}, color.RGBA{0x8d, 0xb1, 0x27, 0xff}
	case strings.HasPrefix(n, "swamp"):
		t.grass, t.foliage = color.RGBA{0x6a, 0x70, 0x39, 0xff}, color.RGBA{0x6a, 0x70, 0x39, 0xff}
	case strings.HasPrefix(n, "roofed_forest"), strings.HasPrefix(n, "dark_forest"):
		g := t.grass
		t.grass = color.RGBA{uint8((int(g.R&0xfe) + 0x28) >> 1), uint8((int(g.G&0xfe) + 0x34) >> 1), uint8((int(g.B&0xfe) + 0x0a) >> 1), 0xff}
	case strings.HasPrefix(n, "mesa"), strings.Contains(n, "badlands"):
		t.grass, t.foliage = color.RGBA{0x90, 0x81, 0x4d, 0xff}, color.RGBA{0x9e, 0x81, 0x4d, 0xff}
	case n == "cherry_grove":
		t.grass, t.foliage = color.RGBA{0xb6, 0xdb, 0x61, 0xff}, color.RGBA{0xb6, 0xdb, 0x61, 0xff}
	case n == "pale_garden":
		t.grass, t.foliage = color.RGBA{0x77, 0x82, 0x72, 0xff}, color.RGBA{0x87, 0x8d, 0x76, 0xff}
	}
	if t.water.A == 0 {
		t.water = defaultWater
	}
	t.water.A = 0xff
	cr.tints[id] = t
	return t, true
}

// tintAt is a biome tint at a block, blended between the 4x4 biome cells
// around it (within the chunk) so borders don't step.
func (cr *ChunkRenderer) tintAt(c *chunk.Chunk, kind tintKind, x uint8, y int16, z uint8) (color.RGBA, bool) {
	pick := func(t biomeTints) color.RGBA {
		switch kind {
		case tintGrass:
			return t.grass
		case tintFoliage:
			return t.foliage
		}
		return t.water
	}
	fx, fz := float64(x)-2, float64(z)-2
	cx, cz := math.Floor(fx/4), math.Floor(fz/4)
	tx, tz := (fx-cx*4)/4, (fz-cz*4)/4
	var sum [3]float64
	wsum := 0.0
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			w := (1 - math.Abs(float64(i)-tx)) * (1 - math.Abs(float64(j)-tz))
			if w <= 0 {
				continue
			}
			bx := min(max(int(cx)+i, 0), 3)*4 + 2
			bz := min(max(int(cz)+j, 0), 3)*4 + 2
			t, ok := cr.biomeTints(c.Biome(uint8(bx), y, uint8(bz)))
			if !ok {
				continue
			}
			col := pick(t)
			sum[0] += w * float64(col.R)
			sum[1] += w * float64(col.G)
			sum[2] += w * float64(col.B)
			wsum += w
		}
	}
	if wsum == 0 {
		return color.RGBA{}, false
	}
	return color.RGBA{uint8(sum[0] / wsum), uint8(sum[1] / wsum), uint8(sum[2] / wsum), 0xff}, true
}

func scaleColor(c color.RGBA, f float64) color.RGBA {
	s := func(v uint8) uint8 { return uint8(math.Max(0, math.Min(255, float64(v)*f))) }
	return color.RGBA{s(c.R), s(c.G), s(c.B), c.A}
}

func mixColor(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}

// tinted is a block's colour in its biome's colours.
func (cr *ChunkRenderer) tinted(c *chunk.Chunk, info *blockInfo, x uint8, y int16, z uint8) color.RGBA {
	col := info.color
	if info.tint == tintNone || cr.Biomes == nil {
		return col
	}
	t, ok := cr.tintAt(c, info.tint, x, y, z)
	if !ok {
		return col
	}
	if info.tint == tintWater {
		return t
	}
	def := plainsGrass
	if info.tint == tintFoliage {
		def = plainsFoliage
	}
	ch := func(v, to, from uint8) uint8 {
		return uint8(math.Min(255, float64(v)*float64(to)/float64(max(from, 1))))
	}
	return color.RGBA{ch(col.R, t.R, def.R), ch(col.G, t.G, def.G), ch(col.B, t.B, def.B), col.A}
}

// surface is a column's colour and the height of its top block (noHeight
// if it has none).
func (cr *ChunkRenderer) surface(c *chunk.Chunk, x, z uint8, top int16) (color.RGBA, int16) {
	bottom := int16(c.Range().Min())
	for y := top; y > bottom; y-- {
		info := cr.block(c.Block(x, y, z, 0))
		if info.skip {
			continue
		}
		col := cr.tinted(c, info, x, y, z)
		if info.water {
			floor, depth := col, int16(0)
			for d := y - 1; d > max(bottom, y-waterDepthMax); d-- {
				fi := cr.block(c.Block(x, d, z, 0))
				if fi.skip || fi.water {
					continue
				}
				floor, depth = cr.tinted(c, fi, x, d, z), y-d
				break
			}
			if depth == 0 {
				depth = waterDepthMax
			}
			col = mixColor(floor, col, math.Min(0.92, 0.55+0.025*float64(depth)))
		} else if col.A != 0xff {
			// glass and the like: what's under shows through
			under, _ := cr.surface(c, x, z, y-1)
			a := float64(col.A) / 255
			col = mixColor(under, color.RGBA{col.R, col.G, col.B, 0xff}, a)
		}
		col.A = 0xff
		return col, y
	}
	return color.RGBA{}, noHeight
}

// slopeShade lights a block at height h whose north neighbour is at north
// from the north, like a map item.
func slopeShade(h, north int16) float64 {
	switch {
	case north == noHeight:
	case h > north:
		return 1.0
	case h < north:
		return 0.76
	}
	return 0.88
}

// Chunk2Img draws a chunk without knowing where it is: its north row is
// lit flat.
func (cr *ChunkRenderer) Chunk2Img(c *chunk.Chunk) *image.RGBA {
	return cr.render(c, nil, nil)
}

// ChunkImage draws the chunk at pos, lighting its north row from the chunk
// north of it if that one was drawn already.
func (cr *ChunkRenderer) ChunkImage(pos world.ChunkPos, c *chunk.Chunk) *image.RGBA {
	cr.mu.Lock()
	north, ok := cr.edges[world.ChunkPos{pos[0], pos[1] - 1}]
	cr.mu.Unlock()
	var northp *[16]int16
	if ok {
		northp = &north
	}
	var south [16]int16
	img := cr.render(c, northp, &south)
	cr.mu.Lock()
	cr.edges[pos] = south
	cr.mu.Unlock()
	return img
}

// NoteHeights keeps a chunk's southmost row of heights without drawing it,
// so the chunk south of it is lit right whichever is drawn first.
func (cr *ChunkRenderer) NoteHeights(pos world.ChunkPos, c *chunk.Chunk) {
	if cr.br == nil || c == nil {
		return
	}
	hm := c.HeightMapWithWater()
	var south [16]int16
	cr.mu.Lock()
	for x := uint8(0); x < 16; x++ {
		_, south[x] = cr.surface(c, x, 15, hm.At(x, 15))
	}
	cr.edges[pos] = south
	cr.mu.Unlock()
}

func (cr *ChunkRenderer) render(c *chunk.Chunk, north, south *[16]int16) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	if cr.br == nil || c == nil {
		return img // no block registry yet: nothing to draw with
	}
	hm := c.HeightMapWithWater()

	cr.mu.Lock()
	defer cr.mu.Unlock()

	var cols [16][16]color.RGBA
	var hs [16][16]int16
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			cols[x][z], hs[x][z] = cr.surface(c, x, z, hm.At(x, z))
		}
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			h := hs[x][z]
			if h == noHeight {
				continue
			}
			n := int16(noHeight)
			if z > 0 {
				n = hs[x][z-1]
			} else if north != nil {
				n = north[x]
			}
			img.SetRGBA(x, z, scaleColor(cols[x][z], slopeShade(h, n)))
		}
	}
	if south != nil {
		for x := 0; x < 16; x++ {
			south[x] = hs[x][15]
		}
	}
	return img
}
