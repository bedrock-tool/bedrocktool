package skinconverter

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path"
	"strconv"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type SkinMeta struct {
	SkinID        string
	PlayFabID     string
	PremiumSkin   bool
	PersonaSkin   bool
	CapeID        string
	SkinColour    string
	ArmSize       string
	Trusted       bool
	PersonaPieces []protocol.PersonaPiece
}

type SkinPack struct {
	skins []*Skin
	Name  string
}

type skinEntry struct {
	LocalizationName string `json:"localization_name"`
	Geometry         string `json:"geometry"`
	Texture          string `json:"texture"`
	Type             string `json:"type"`
}

func NewSkinPack(name, fpath string) *SkinPack {
	return &SkinPack{
		Name: name,
	}
}

func (s *SkinPack) AddSkin(skin *Skin) bool {
	sh := skin.Hash()
	for _, skin2 := range s.skins {
		if skin2.Hash() == sh {
			return false
		}
	}
	s.skins = append(s.skins, skin)
	return true
}

func (s *SkinPack) Latest() *Skin {
	if len(s.skins) == 0 {
		return nil
	}
	return s.skins[len(s.skins)-1]
}

func write112Geometry(fs utils.WriterFS, geometryName string, geometry *SkinGeometry) error {
	f, err := fs.Create(fmt.Sprintf("geometry-%s.json", geometryName))
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	e.SetIndent("", "\t")
	return e.Encode(map[string]any{
		"format_version":     "1.12.0",
		"minecraft:geometry": []*SkinGeometry{geometry},
	})
}

func writePng(fs utils.WriterFS, filename string, img image.Image) error {
	f, err := fs.Create(filename)
	if err != nil {
		return errors.New(locale.Loc("failed_write", locale.Strmap{"Part": "Meta", "Path": filename, "Err": err}))
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return errors.New(locale.Loc("failed_write", locale.Strmap{"Part": "Texture", "Path": filename, "Err": err}))
	}
	return nil
}

func (sp *SkinPack) Save(fpath string) error {
	os.MkdirAll(fpath, 0o755)
	fs := utils.OSWriter{Base: fpath}

	var skinsJson struct {
		Skins []skinEntry `json:"skins"`
	}
	geometryJson := map[string]SkinGeometry_Old{}

	for i, skin := range sp.skins { // write skin texture
		skinName := sp.Name
		if i > 0 {
			skinName += "-" + strconv.Itoa(i)
		}

		skinImage := image.NewNRGBA(image.Rect(0, 0, int(skin.SkinImageWidth), int(skin.SkinImageHeight)))
		copy(skinImage.Pix, skin.SkinData)
		if err := writePng(fs, skinName+".png", skinImage); err != nil {
			return err
		}

		if skin.HaveCape() {
			capeImage := image.NewNRGBA(image.Rect(0, 0, int(skin.CapeImageWidth), int(skin.CapeImageHeight)))
			copy(capeImage.Pix, skin.CapeData)
			if err := writePng(fs, skinName+"_cape.png", capeImage); err != nil {
				return err
			}
		}

		if err := skin.writeMetadataJson(fs, skinName+"_metadata.json"); err != nil {
			return err
		}

		entry := skinEntry{
			LocalizationName: skinName,
			Texture:          skinName + ".png",
			Type:             "free",
		}
		if skin.ArmSize == "wide" {
			entry.Geometry = "minecraft.geometry.steve"
		} else {
			entry.Geometry = "minecraft.geometry.alex"
		}

		if skin.HaveGeometry() {
			identifier, formatVersion, geometry, err := skin.ParseGeometry()
			if err != nil {
				logrus.Warnf("failed to decode geometry %s %v", skinName, err)
			}
			_ = formatVersion
			if geometry != nil {
				err := write112Geometry(fs, identifier, geometry)
				if err != nil {
					logrus.Warnf("failed to write geometry %s %v", skinName, err)
				}
				geometryJson[identifier] = SkinGeometry_Old{
					SkinGeometryDescription: geometry.Description,
					Bones:                   geometry.Bones,
				}
				entry.Geometry = identifier
			}
		}
		skinsJson.Skins = append(skinsJson.Skins, entry)
	}

	if len(geometryJson) > 0 {
		f, err := fs.Create("geometry.json")
		if err != nil {
			return err
		}
		defer f.Close()
		e := json.NewEncoder(f)
		e.SetIndent("", "  ")
		if err := e.Encode(geometryJson); err != nil {
			return err
		}
	}

	{ // skins.json
		f, err := os.Create(path.Join(fpath, "skins.json"))
		if err != nil {
			return err
		}
		defer f.Close()
		e := json.NewEncoder(f)
		e.SetIndent("", "  ")
		if err := e.Encode(skinsJson); err != nil {
			return err
		}
	}

	{ // manifest.json
		manifest := resource.Manifest{
			FormatVersion: 2,
			Header: resource.Header{
				Name:               sp.Name,
				Description:        sp.Name,
				UUID:               uuid.New(),
				Version:            [3]int{1, 0, 0},
				MinimumGameVersion: [3]int{1, 17, 0},
			},
			Modules: []resource.Module{
				{
					UUID:        uuid.NewString(),
					Description: sp.Name + " Skinpack",
					Type:        "skin_pack",
					Version:     [3]int{1, 0, 0},
				},
			},
		}

		if err := utils.WriteManifest(&manifest, fs, ""); err != nil {
			return err
		}
	}

	return nil
}
