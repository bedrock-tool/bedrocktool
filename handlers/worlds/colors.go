package worlds

import (
	"encoding/json"
	"errors"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
	"github.com/thomaso-mirodin/intmath/u32"
	"golang.org/x/exp/slices"
)

func getTexturePaths(entries []protocol.BlockEntry) map[string]string {
	var paths = map[string]string{}
	for _, be := range entries {
		if components, ok := be.Properties["components"].(map[string]any); ok {
			mats, ok := components["minecraft:material_instances"].(map[string]any)
			if !ok {
				paths[be.Name] = ""
				continue
			}
			instance, ok := mats["*"].(map[string]any)
			if !ok {
				if instance, ok = mats["up"].(map[string]any); !ok {
					continue
				}
			}

			texture, ok := instance["texture"].(string)
			if !ok {
				continue
			}
			paths[be.Name] = texture
		}
	}
	return paths
}

func readBlocksJson(fs fs.FS) (map[string]string, error) {
	f, err := fs.Open("blocks.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	var m map[string]any
	err = d.Decode(&m)
	if err != nil {
		return nil, err
	}

	var out = make(map[string]string)
	for name, v := range m {
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

func getBlockColor(fs fs.FS, path string) (c color.RGBA, err error) {
	r, err := fs.Open(path)
	if err != nil {
		return c, err
	}
	defer r.Close()

	img, err := png.Decode(r)
	if err != nil {
		return c, err
	}

	var rt, gt, bt uint32
	dx := img.Bounds().Dx()
	dy := img.Bounds().Dy()
	for x := 0; x < dx; x++ {
		for y := 0; y < dy; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rt += r / 0xff * r / 0xff
			gt += g / 0xff * g / 0xff
			bt += b / 0xff * b / 0xff
		}
	}

	return color.RGBA{
		R: uint8(u32.Sqrt(rt)),
		G: uint8(u32.Sqrt(gt)),
		B: uint8(u32.Sqrt(bt)),
		A: 0xff,
	}, nil
}

func (m *MapUI) resolveColors(entries []protocol.BlockEntry) {
	paths := getTexturePaths(entries)

	for _, p := range m.w.serverState.packs {
		fs, names, err := p.FS()
		if err != nil {
			logrus.Error(err)
			continue
		}

		blocksJson, err := readBlocksJson(fs)
		if err != nil {
			logrus.Error(err)
			continue
		}

		for name, path := range paths {
			if path == "" {
				paths[name] = blocksJson[name]
			}
		}

		for name, path := range paths {
			texturePath := "textures/blocks/" + strings.Replace(path, ":", "/", 1) + ".png"
			_, hasFile := slices.BinarySearch(names, texturePath)
			if !hasFile {
				continue
			}
			delete(paths, name)
			col, err := getBlockColor(fs, texturePath)
			if err != nil {
				logrus.Error(err)
				continue
			}
			utils.CustomBlockColors[name] = col
		}
	}
	m.wg.Done()
}
