package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

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

type _skinWithIndex struct {
	i    int
	skin *Skin
}

func (s _skinWithIndex) Name(name string) string {
	if s.i == 1 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, s.i)
}

type SkinPack struct {
	skins map[uuid.UUID]_skinWithIndex
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
		skins: make(map[uuid.UUID]_skinWithIndex),
		Name:  name,
	}
}

func (s *SkinPack) AddSkin(skin *Skin) bool {
	sh := skin.Hash()
	if _, ok := s.skins[sh]; !ok {
		s.skins[sh] = _skinWithIndex{len(s.skins) + 1, skin}
		return true
	}
	return false
}

func (s *SkinPack) Save(fpath, serverName string) error {
	os.MkdirAll(fpath, 0o755)

	var skinsJson struct {
		Skins []skinEntry `json:"skins"`
	}
	geometryJson := map[string]SkinGeometry{}

	for _, s2 := range s.skins { // write skin texture
		skinName := s2.Name(s.Name)

		if err := s2.skin.writeSkinTexturePng(path.Join(fpath, skinName+".png")); err != nil {
			return err
		}

		if err := s2.skin.writeMetadataJson(path.Join(fpath, skinName+"_metadata.json")); err != nil {
			return err
		}

		if s2.skin.HaveCape() {
			if err := s2.skin.WriteCapePng(path.Join(fpath, skinName+"_cape.png")); err != nil {
				return err
			}
		}

		entry := skinEntry{
			LocalizationName: skinName,
			Texture:          skinName + ".png",
			Type:             "free",
		}
		if s2.skin.ArmSize == "wide" {
			entry.Geometry = "minecraft.geometry.steve"
		} else {
			entry.Geometry = "minecraft.geometry.alex"
		}

		if s2.skin.HaveGeometry() {
			geometry, geometryName, err := s2.skin.getGeometry()
			if err != nil {
				logrus.Warnf("failed to decode geometry %s %v", skinName, err)
			} else if geometry != nil {
				f, err := os.Create(path.Join(fpath, fmt.Sprintf("geometry-%s.json", geometryName)))
				if err != nil {
					return err
				}
				e := json.NewEncoder(f)
				e.SetIndent("", "\t")
				if err := e.Encode(map[string]any{
					"format_version":     "1.12.0",
					"minecraft:geometry": []*SkinGeometry_1_12{geometry},
				}); err != nil {
					f.Close()
					return err
				}
				f.Close()
				geometryJson[geometryName] = SkinGeometry{
					SkinGeometryDescription: geometry.Description,
					Bones:                   geometry.Bones,
				}
				entry.Geometry = geometryName
			}
		}
		skinsJson.Skins = append(skinsJson.Skins, entry)
	}

	if len(geometryJson) > 0 { // geometry.json
		f, err := os.Create(path.Join(fpath, "geometry.json"))
		if err != nil {
			return err
		}
		e := json.NewEncoder(f)
		e.SetIndent("", "\t")
		if err := e.Encode(geometryJson); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	{ // skins.json
		f, err := os.Create(path.Join(fpath, "skins.json"))
		if err != nil {
			return err
		}
		e := json.NewEncoder(f)
		e.SetIndent("", "\t")
		if err := e.Encode(skinsJson); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	{ // manifest.json
		manifest := resource.Manifest{
			FormatVersion: 2,
			Header: resource.Header{
				Name:               s.Name,
				Description:        serverName + " " + s.Name,
				UUID:               uuid.NewString(),
				Version:            [3]int{1, 0, 0},
				MinimumGameVersion: [3]int{1, 17, 0},
			},
			Modules: []resource.Module{
				{
					UUID:        uuid.NewString(),
					Description: s.Name + " Skinpack",
					Type:        "skin_pack",
					Version:     [3]int{1, 0, 0},
				},
			},
		}

		if err := WriteManifest(&manifest, fpath); err != nil {
			return err
		}
	}

	return nil
}
