package utils

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path"
	"strings"

	"github.com/dblezek/tga"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

func getTextureNames(entries []protocol.BlockEntry) map[string]string {
	var res = map[string]string{}
	for _, be := range entries {
		if components, ok := be.Properties["components"].(map[string]any); ok {
			if mats, ok := components["minecraft:material_instances"].(map[string]any); ok {
				if mm, ok := mats["materials"].(map[string]any); ok {
					mats = mm
				}
				instance, ok := mats["*"].(map[string]any)
				if !ok {
					instance, _ = mats["up"].(map[string]any)
				}
				if instance != nil {
					texture, ok := instance["texture"].(string)
					if ok {
						res[be.Name] = texture
					}
				}
			}
			continue
		}
		res[be.Name] = be.Name
	}
	return res
}

func readBlocksJson(f fs.FS) (map[string]string, error) {
	blocksJsonContent, err := fs.ReadFile(f, "blocks.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var m map[string]any
	err = ParseJson(blocksJsonContent, &m)
	if err != nil {
		return nil, err
	}

	var out = make(map[string]string)
	for name, v := range m {
		if !strings.Contains(name, ":") {
			name = "minecraft:" + name
		}
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		texs, ok := vm["textures"]
		if !ok {
			out[name] = name
		}
	reParse:
		switch textures := texs.(type) {
		case string:
			out[name] = textures
		case map[string]any:
			texs = textures["up"]
			goto reParse
		}
	}
	return out, nil
}

func loadFlipbooks(f fs.FS) (map[string]string, error) {
	flipbookContent, err := fs.ReadFile(f, "textures/flipbook_textures.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var m []struct {
		Texture string `json:"flipbook_texture"`
		Atlas   string `json:"atlas_tile"`
	}
	err = ParseJson(flipbookContent, &m)
	if err != nil {
		return nil, err
	}

	o := make(map[string]string)
	for _, v := range m {
		o[v.Atlas] = v.Texture
	}
	return o, nil
}

func loadTexturesList(f fs.FS) (map[string]string, error) {
	texturesContent, err := fs.ReadFile(f, "textures/textures_list.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var list []string
	err = ParseJson(texturesContent, &list)
	if err != nil {
		return nil, err
	}

	o := make(map[string]string)
	for _, name := range list {
		o[path.Base(name)] = name
	}

	return o, nil
}

func loadTerrainTexture(f fs.FS) (map[string]string, error) {
	terrainContent, err := fs.ReadFile(f, "textures/terrain_texture.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var m struct {
		Data map[string]struct {
			Textures any `json:"textures"`
		} `json:"texture_data"`
	}
	err = ParseJson(terrainContent, &m)
	if err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for textureName, v := range m.Data {
		textures := v.Textures
	reParse:
		switch texture := textures.(type) {
		case string:
			out[textureName] = texture
		case []any:
			textures = texture[0]
			goto reParse
		case map[string]any:
			variations, ok := texture["variations"].([]any)
			if ok {
				variation := variations[0].(map[string]any)
				textures = variation
				goto reParse
			}
			out[textureName] = texture["path"].(string)
		default:
			continue
		}
	}

	return out, nil
}

func calculateMeanAverageColour(img image.Image) (c color.RGBA) {
	imgSize := img.Bounds().Size()

	var redSum float64
	var greenSum float64
	var blueSum float64

	for x := 0; x < imgSize.X; x++ {
		for y := 0; y < imgSize.Y; y++ {
			pixel := img.At(x, y)
			col := color.RGBAModel.Convert(pixel).(color.RGBA)
			if col.A < 128 {
				continue
			}

			redSum += float64(col.R) * float64(col.R)
			greenSum += float64(col.G) * float64(col.G)
			blueSum += float64(col.B) * float64(col.B)
		}
	}

	imgArea := float64(imgSize.X * imgSize.Y)

	return color.RGBA{
		uint8(math.Round(math.Sqrt(redSum / imgArea))),
		uint8(math.Round(math.Sqrt(greenSum / imgArea))),
		uint8(math.Round(math.Sqrt(blueSum / imgArea))),
		0xff,
	}
}

type mergedFS struct {
	fss []fs.FS
}

func (m *mergedFS) Open(name string) (f fs.File, err error) {
	for _, fsys := range m.fss {
		f, err = fsys.Open(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		break
	}
	return
}

func ResolveColors(entries []protocol.BlockEntry, packs []resource.Pack) map[string]color.RGBA {
	log := logrus.WithField("func", "ResolveColors")
	colors := make(map[string]color.RGBA)

	processPack := func(pack resource.Pack, merged fs.FS, textureNames map[string]string) error {
		flipbooks, err := loadFlipbooks(pack)
		if err != nil {
			return err
		}

		terrainTextures, err := loadTerrainTexture(pack)
		if err != nil {
			return err
		}

		texturesList, err := loadTexturesList(pack)
		if err != nil {
			return err
		}

		if len(flipbooks)+len(terrainTextures)+len(texturesList) == 0 {
			return nil
		}

		for block, texture_name := range textureNames {
			if _, ok := colors[block]; ok {
				continue
			}

			var texturePath string
			if terrain_texture, ok := terrainTextures[texture_name]; ok {
				texturePath = terrain_texture
			} else if flipbook_texture, ok := flipbooks[texture_name]; ok {
				texturePath = flipbook_texture
			} else if tex, ok := texturesList[texture_name]; ok {
				texturePath = tex
			} else {
				continue
			}

			var img image.Image
			for _, format := range []string{".png", ".tga"} {
				r, err := merged.Open(texturePath + format)
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil {
					log.Error(err)
					continue
				}

				switch format {
				case ".png":
					img, err = png.Decode(r)
				case ".tga":
					img, err = tga.Decode(r)
				default:
					panic("invalid extension")
				}
				r.Close()
				if err != nil {
					return err
				}
				break
			}
			if img == nil {
				continue
			}

			colors[block] = calculateMeanAverageColour(img)
			delete(textureNames, block)
		}

		return nil
	}

	textureNames := getTextureNames(entries)
	var blockPacks []resource.Pack
	var merged mergedFS
	for _, pack := range packs {
		blocksJson, err := readBlocksJson(pack)
		if err != nil {
			logrus.Error(err)
		}

		for block, name := range blocksJson {
			if _, ok := textureNames[block]; !ok {
				textureNames[block] = name
			}
		}

		blockPacks = append(blockPacks, pack)
		merged.fss = append(merged.fss, pack)
	}

	for _, pack := range blockPacks {
		err := processPack(pack, &merged, textureNames)
		if err != nil {
			log.Warn(err)
		}
	}

	return colors
}
