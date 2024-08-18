package utils

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
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
			mats, ok := components["minecraft:material_instances"].(map[string]any)
			if ok {
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
		textures, ok := vm["textures"]
		if !ok {
			continue
		}
		if texture, ok := textures.(string); ok {
			out[name] = texture
			continue
		}
		if textures, ok := textures.(map[string]any); ok {
			texture, ok := textures["up"].(string)
			if !ok {
				continue
			}
			out[name] = texture
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

	o := make(map[string]string)
	for k, v := range m.Data {
		if tex, ok := v.Textures.(string); ok {
			o[k] = tex
		}
	}

	return o, nil
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

func ResolveColors(entries []protocol.BlockEntry, packs []resource.Pack) map[string]color.RGBA {
	log := logrus.WithField("func", "ResolveColors")
	colors := make(map[string]color.RGBA)

	processPack := func(pack resource.Pack, textureNames map[string]string) error {
		blocksJson, err := readBlocksJson(pack)
		if err != nil {
			return err
		}
		if blocksJson == nil {
			return nil
		}

		for block, name := range blocksJson {
			textureNames[block] = name
		}

		flipbooks, err := loadFlipbooks(pack)
		if err != nil {
			return err
		}

		terrainTextures, err := loadTerrainTexture(pack)
		if err != nil {
			return err
		}

		if flipbooks == nil && terrainTextures == nil {
			return nil
		}

		for block, texture_name := range textureNames {
			var texturePath string
			if flipbook_texture, ok := flipbooks[texture_name]; ok {
				texturePath = flipbook_texture
			} else {
				terrain_texture, ok := terrainTextures[texture_name]
				if ok {
					texturePath = terrain_texture
				}
			}

			if texturePath == "" {
				continue
			}

			matches, err := fs.Glob(pack, texturePath+".*")
			if err != nil {
				log.Warn(err)
				continue
			}
			if len(matches) == 0 {
				continue
			}

			texturePath = matches[0]

			delete(textureNames, block)
			r, err := pack.Open(texturePath)
			if err != nil {
				log.Error(err)
				continue
			}
			var img image.Image
			switch filepath.Ext(texturePath) {
			case ".png":
				img, err = png.Decode(r)
			case ".tga":
				img, err = tga.Decode(r)
			default:
				err = errors.New("invalid ext " + texturePath)
			}
			r.Close()
			if err != nil {
				return err
			}
			if img == nil {
				continue
			}

			colors[block] = calculateMeanAverageColour(img)
		}
		return nil
	}

	textureNames := getTextureNames(entries)
	for _, pack := range packs {
		err := processPack(pack, textureNames)
		if err != nil {
			log.Warn(err)
		}
	}

	if len(textureNames) > 0 {
		println("")
	}

	return colors
}
