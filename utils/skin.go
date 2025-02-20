package utils

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type Skin struct {
	*protocol.Skin
}

type SkinGeometry_Old struct {
	SkinGeometryDescription
	Bones json.RawMessage `json:"bones"`
}

type SkinGeometryDescription struct {
	Identifier          string      `json:"identifier"`
	TextureWidth        json.Number `json:"texture_width"`
	TextureHeight       json.Number `json:"texture_height"`
	VisibleBoundsWidth  float64     `json:"visible_bounds_width"`
	VisibleBoundsHeight float64     `json:"visible_bounds_height"`
	VisibleBoundsOffset []float64   `json:"visible_bounds_offset,omitempty"`
}

type SkinGeometry struct {
	Description SkinGeometryDescription `json:"description"`
	Bones       json.RawMessage         `json:"bones"`
}

type SkinGeometryFile struct {
	FormatVersion string         `json:"format_version"`
	Geometry      []SkinGeometry `json:"minecraft:geometry"`
}

type geometry180 struct {
	Bones         json.RawMessage `json:"bones"`
	TextureWidth  int             `json:"texturewidth"`
	TextureHeight int             `json:"textureheight"`
}

type geom180 struct {
	m  map[string]geometry180
	id string
}

func (n *geom180) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"format_version": "1.8.0",
	}
	for k, v := range n.m {
		m[k] = v
	}
	return json.Marshal(m)
}

func (n *geom180) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	if n.m == nil {
		n.m = make(map[string]geometry180)
	}
	var geom geometry180
	err = json.Unmarshal(m[n.id], &geom)
	if err != nil {
		return err
	}
	n.m[n.id] = geom
	return nil
}

func (skin *Skin) Hash() uuid.UUID {
	h := append(skin.CapeData, append(skin.SkinData, skin.SkinGeometry...)...)
	return uuid.NewSHA1(uuid.NameSpaceURL, h)
}

func (skin *Skin) getGeometry() (*SkinGeometry, string, error) {
	if !skin.HaveGeometry() {
		return nil, "", errors.New("no geometry")
	}
	geom, identifier, err := ParseServerSkinGeometry(skin.Skin)
	if err != nil {
		return nil, "", err
	}
	if geom == nil {
		return nil, identifier, nil
	}
	if len(geom.Geometry) == 0 {
		return nil, identifier, nil
	}
	return &geom.Geometry[0], identifier, nil
}

func (skin *Skin) writeMetadataJson(fs WriterFS, filename string) error {
	f, err := fs.Create(filename)
	if err != nil {
		return errors.New(locale.Loc("failed_write", locale.Strmap{"Part": "Meta", "Path": filename, "Err": err}))
	}
	defer f.Close()
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(SkinMeta{
		skin.SkinID,
		skin.PlayFabID,
		skin.PremiumSkin,
		skin.PersonaSkin,
		skin.CapeID,
		skin.SkinColour,
		skin.ArmSize,
		skin.Trusted,
		skin.PersonaPieces,
	})
}

func (skin *Skin) HaveGeometry() bool {
	return len(skin.SkinGeometry) > 0
}

func (skin *Skin) HaveCape() bool {
	return len(skin.CapeData) > 0
}

func (skin *Skin) HaveAnimations() bool {
	return len(skin.Animations) > 0
}

func (skin *Skin) HaveTint() bool {
	return len(skin.PieceTintColours) > 0
}

func (skin *Skin) Complex() bool {
	return skin.HaveGeometry() || skin.HaveCape() || skin.HaveAnimations() || skin.HaveTint()
}

func ParseServerSkinGeometry(skin *protocol.Skin) (*SkinGeometryFile, string, error) {
	var resourcePatch map[string]map[string]string
	if len(skin.SkinResourcePatch) > 0 {
		err := ParseJson(skin.SkinResourcePatch, &resourcePatch)
		if err != nil {
			return nil, "", err
		}
	}
	var identifier string
	if resourcePatch != nil {
		identifier = resourcePatch["geometry"]["default"]
	}

	var data *struct {
		FormatVersion string         `json:"format_version"`
		Geometry      []SkinGeometry `json:"minecraft:geometry"`
	}
	err := ParseJson(skin.SkinGeometry, &data)
	if err != nil {
		return nil, identifier, err
	}
	if data == nil {
		return nil, identifier, nil
	}

	if data.FormatVersion == "1.8.0" {
		var m geom180 = geom180{
			id: identifier,
		}
		err := ParseJson(skin.SkinGeometry, &m)
		if err != nil {
			return nil, "", err
		}
		geom := m.m[identifier]
		return &SkinGeometryFile{
			FormatVersion: data.FormatVersion,
			Geometry: []SkinGeometry{
				{
					Description: SkinGeometryDescription{
						Identifier:    identifier,
						TextureWidth:  json.Number(strconv.Itoa(int(geom.TextureWidth))),
						TextureHeight: json.Number(strconv.Itoa(int(geom.TextureHeight))),
					},
					Bones: geom.Bones,
				},
			},
		}, identifier, nil
	}

	return &SkinGeometryFile{
		FormatVersion: string(skin.GeometryDataEngineVersion),
		Geometry:      data.Geometry,
	}, identifier, nil
}
