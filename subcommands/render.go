package subcommands

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bedrock-tool/bedrocktool/locale"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/merge"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type RenderSettings struct {
	WorldPath string `opt:"locale.opt_render_world" flag:"world" desc:"locale.desc_render_world"`
	Out       string `opt:"locale.opt_render_out" flag:"out" default:"world.png" desc:"locale.desc_render_out"`
	NoImage   bool   `opt:"locale.opt_render_no_image" flag:"no-image" desc:"locale.desc_render_no_image"`
	MapArt    bool   `opt:"locale.opt_render_map_art" flag:"map-art" desc:"locale.desc_render_map_art"`
	MapArtOut string `opt:"locale.opt_render_map_art_out" flag:"map-art-out" desc:"locale.desc_render_map_art_out"`
	Dimension string `opt:"locale.opt_render_dimension" flag:"dimension" desc:"locale.desc_render_dimension"`
}

type RenderCMD struct{}

func (RenderCMD) Name() string {
	return "render"
}

func (RenderCMD) Description() string {
	return locale.Loc("render_synopsis", nil)
}

func (RenderCMD) Settings() any {
	return new(RenderSettings)
}

func (RenderCMD) Run(ctx context.Context, settings any) error {
	renderSettings := settings.(*RenderSettings)

	blockReg := &merge.BlockRegistry{
		BlockRegistry: world.DefaultBlockRegistry,
		Rids:          make(map[uint32]merge.Block),
	}

	if renderSettings.WorldPath == "" {
		var ok bool
		renderSettings.WorldPath, ok = utils.UserInput(ctx, locale.Loc("render_world_prompt", nil), func(s string) bool {
			_, err := os.Stat(s)
			return err == nil
		})
		if !ok {
			return nil
		}
	}

	renderSettings.WorldPath = path.Clean(strings.ReplaceAll(renderSettings.WorldPath, "\\", "/"))
	renderSettings.Out = path.Clean(strings.ReplaceAll(renderSettings.Out, "\\", "/"))

	if renderSettings.WorldPath == "" {
		return fmt.Errorf("missing -world")
	}
	fmt.Printf("%s\n", renderSettings.WorldPath)

	// a .mcworld is a zipped world folder
	if st, err := os.Stat(renderSettings.WorldPath); err == nil && !st.IsDir() {
		tmp, err := os.MkdirTemp("", "bedrocktool-render-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := unzipWorld(renderSettings.WorldPath, tmp); err != nil {
			return fmt.Errorf("opening %s: %w", renderSettings.WorldPath, err)
		}
		renderSettings.WorldPath = tmp
	}

	// custom dimensions (-merge-dimensions) must be known before reading
	dims, err := worldstate.RegisterWorldDimensions(renderSettings.WorldPath)
	if err != nil {
		return err
	}

	db, err := mcdb.Config{
		Log:    slog.Default(),
		Blocks: blockReg,
		LDBOptions: &opt.Options{
			ReadOnly: true,
		},
	}.Open(renderSettings.WorldPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var resourcePacks []resource.Pack
	resourcePacksFolder := path.Join(renderSettings.WorldPath, "resource_packs")
	resourcePackEntries, err := os.ReadDir(resourcePacksFolder)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range resourcePackEntries {
		pack, err := resource.ReadPath(path.Join(resourcePacksFolder, entry.Name()))
		if err != nil {
			return err
		}
		resourcePacks = append(resourcePacks, pack)
	}

	var behaviorPacks []resource.Pack
	behaviorPacksFolder := path.Join(renderSettings.WorldPath, "behavior_packs")
	behaviorPackEntries, err := os.ReadDir(behaviorPacksFolder)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range behaviorPackEntries {
		pack, err := resource.ReadPath(path.Join(behaviorPacksFolder, entry.Name()))
		if err != nil {
			return err
		}
		behaviorPacks = append(behaviorPacks, pack)
	}

	var entries []protocol.BlockEntry
	for _, pack := range behaviorPacks {
		blockEntries, err := fs.Glob(pack, "blocks/**/*.json")
		if err != nil {
			return err
		}
		for _, bff := range blockEntries {
			f, err := pack.Open(bff)
			if err != nil {
				return err
			}
			var BlockBehaviour behaviourpack.BlockBehaviour
			err = json.NewDecoder(f).Decode(&BlockBehaviour)
			f.Close()
			if err != nil {
				return err
			}
			block := BlockBehaviour.MinecraftBlock

			ent := protocol.BlockEntry{
				Name: block.Description.Identifier,
				Properties: map[string]any{
					"components": block.Components,
				},
			}
			entries = append(entries, ent)
		}
	}

	dim, err := pickDimension(db, dims, renderSettings.Dimension)
	if err != nil {
		return err
	}
	if len(dims) > 3 || dim.ID != 0 {
		logrus.Info(locale.Loc("render_dimension", locale.Strmap{"Name": dim.Identifier}))
	}
	dimRange := &mcdb.IteratorRange{Dimension: dim.Dimension}

	if renderSettings.MapArt {
		if err := exportWorldMapArt(db, blockReg, renderSettings, dimRange); err != nil {
			return err
		}
	}
	if renderSettings.NoImage {
		return nil
	}

	renderer := utils.NewChunkRenderer(blockReg)
	renderer.Biomes = world.DefaultBiomes
	renderer.ResolveColors(entries, resourcePacks)

	boundsMin := world.ChunkPos{math.MaxInt32, math.MaxInt32}
	boundsMax := world.ChunkPos{math.MinInt32, math.MinInt32}
	// the first pass also notes each chunk's south edge, so every chunk's
	// north row is lit from its neighbour's heights
	it := db.NewColumnIterator(dimRange)
	for it.Next() {
		pos := it.Position()
		renderer.NoteHeights(pos, it.Column().Chunk)
		boundsMin[0] = min(boundsMin[0], pos[0])
		boundsMin[1] = min(boundsMin[1], pos[1])
		boundsMax[0] = max(boundsMax[0], pos[0])
		boundsMax[1] = max(boundsMax[1], pos[1])
	}
	it.Release()
	if err := it.Error(); err != nil {
		return err
	}

	chunksX := int(boundsMax[0] - boundsMin[0] + 1)
	chunksY := int(boundsMax[1] - boundsMin[1] + 1)
	r := image.Rect(0, 0, chunksX*16, chunksY*16)
	fmt.Printf("%dx%d pixels\n", r.Dx(), r.Dy())
	img := image.NewRGBA(r)

	it = db.NewColumnIterator(dimRange)
	for it.Next() {
		col := it.Column()
		pos := it.Position()
		if it.Error() != nil {
			break
		}

		tile := renderer.ChunkImage(pos, col.Chunk)
		px := image.Pt(
			int((pos.X()-boundsMin.X())*16),
			int((pos.Z()-boundsMin.Z())*16),
		)
		draw.Draw(img, image.Rect(
			px.X, px.Y,
			px.X+16, px.Y+16,
		), tile, image.Point{}, draw.Src)
	}
	it.Release()
	if err := it.Error(); err != nil {
		return err
	}

	outPath := utils.PathData(renderSettings.Out)

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	err = png.Encode(f, img)
	if err != nil {
		return err
	}

	logrus.Info(locale.Loc("render_wrote", locale.Strmap{"Path": outPath}))

	return nil
}

func init() {
	commands.RegisterCommand(&RenderCMD{})
}

// pickDimension is the dimension -dimension names, or with none, the
// overworld, or else the first dimension the world has chunks in.
func pickDimension(db *mcdb.DB, dims []worldstate.WorldDimension, name string) (worldstate.WorldDimension, error) {
	if name != "" {
		if d, ok := worldstate.FindDimension(dims, name); ok {
			return d, nil
		}
		var names []string
		for _, d := range dims {
			names = append(names, d.Identifier)
		}
		return worldstate.WorldDimension{}, errors.New(locale.Loc("render_dimension_unknown", locale.Strmap{"Name": name, "List": strings.Join(names, ", ")}))
	}
	for _, d := range dims {
		it := db.NewColumnIterator(&mcdb.IteratorRange{Dimension: d.Dimension})
		has := it.Next()
		it.Release()
		if has {
			return d, nil
		}
	}
	return dims[0], nil
}

// exportWorldMapArt writes the maps a saved world's item frames and chests
// hold as PNGs, the same way the worlds downloader's -map-art does: each map
// as tiles/<id>.png, and each wall of maps stitched into one picture.
func exportWorldMapArt(db *mcdb.DB, br world.BlockRegistry, settings *RenderSettings, dimRange *mcdb.IteratorRange) error {
	var frames []worldstate.MapFrame
	ids := make(map[int64]struct{})
	it := db.NewColumnIterator(dimRange)
	for it.Next() {
		col := it.Column()
		facing := func(p cube.Pos) int32 {
			r := col.Chunk.Range()
			if p.Y() < r.Min() || p.Y() > r.Max() {
				return -1
			}
			rid := col.Chunk.Block(uint8(p.X()&15), int16(p.Y()), uint8(p.Z()&15), 0)
			_, props, ok := br.RuntimeIDToState(rid)
			if !ok {
				return -1
			}
			return worldstate.FacingDirection(props)
		}
		for _, be := range col.BlockEntities {
			f, loose := worldstate.MapsIn(be.Pos, be.Data, facing)
			frames = append(frames, f...)
			for _, x := range f {
				ids[x.MapID] = struct{}{}
			}
			for _, id := range loose {
				ids[id] = struct{}{}
			}
		}
	}
	it.Release()
	if err := it.Error(); err != nil {
		return err
	}

	images := make(map[int64]*image.NRGBA)
	for id := range ids {
		data, err := db.LDB().Get([]byte(fmt.Sprintf("map_%d", id)), nil)
		if err != nil {
			continue // the map's pixels were never saved
		}
		var m worldstate.Map
		if err := nbt.UnmarshalEncoding(data, &m, nbt.LittleEndian); err != nil {
			logrus.Warnf("map %d: %s", id, err)
			continue
		}
		if img := worldstate.MapImage(&m); img != nil {
			images[id] = img
		}
	}

	dir := settings.MapArtOut
	if dir == "" {
		dir = strings.TrimSuffix(settings.Out, path.Ext(settings.Out)) + "_mapart"
	}
	dir = utils.PathData(dir)
	grids, tiles, err := worldstate.WriteMapArt(dir, images, frames)
	if err != nil {
		return err
	}
	switch {
	case len(ids) == 0:
		logrus.Info(locale.Loc("render_map_art_none", nil))
	case tiles == 0:
		logrus.Info(locale.Loc("render_map_art_no_pixels", locale.Strmap{"Count": len(ids)}))
	default:
		logrus.Info(locale.Loc("render_map_art_written", locale.Strmap{"Maps": tiles, "Walls": grids, "Dir": dir}))
	}
	return nil
}

// unzipWorld extracts a .mcworld into dir.
func unzipWorld(name, dir string) error {
	zr, err := zip.OpenReader(name)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		target := filepath.Join(dir, filepath.FromSlash(f.Name))
		// a name like ../x would land outside dir
		if rel, err := filepath.Rel(dir, target); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("bad path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
