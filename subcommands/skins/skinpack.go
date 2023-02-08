package skins

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type _skinWithIndex struct {
	i    int
	skin *Skin
}

func (s _skinWithIndex) Name(name string) string {
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

func NewSkinPack(name string) *SkinPack {
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

	var skinsJson []skinEntry
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
				logrus.Warnf("failed to decode geometry %s", skinName)
			} else {
				geometryJson[geometryName] = *geometry
				entry.Geometry = geometryName
			}
		}
		skinsJson = append(skinsJson, entry)
	}

	if len(geometryJson) > 0 { // geometry.json
		f, err := os.Create(path.Join(fpath, "geometry.json"))
		if err != nil {
			return err
		}
		if err := json.NewEncoder(f).Encode(geometryJson); err != nil {
			return err
		}
	}

	{ // skins.json
		f, err := os.Create(path.Join(fpath, "skins.json"))
		if err != nil {
			return err
		}
		if err := json.NewEncoder(f).Encode(skinsJson); err != nil {
			return err
		}
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

		if err := utils.WriteManifest(&manifest, fpath); err != nil {
			return err
		}
	}

	return nil
}
