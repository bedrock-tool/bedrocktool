package skinconverter

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
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
	Bones               json.RawMessage `json:"bones"`
	TextureWidth        int             `json:"texturewidth"`
	TextureHeight       int             `json:"textureheight"`
	VisibleBoundsWidth  float64         `json:"visible_bounds_width"`
	VisibleBoundsHeight float64         `json:"visible_bounds_height"`
	VisibleBoundsOffset []float64       `json:"visible_bounds_offset,omitempty"`
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

func (skin *Skin) writeMetadataJson(fs utils.WriterFS, filename string) error {
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

func parseGeometry180(raw []byte, identifier string) (*SkinGeometry, error) {
	var m map[string]geometry180
	err := json.Unmarshal(raw, &m)
	if err != nil {
		return nil, err
	}
	geom, ok := m[identifier]
	if !ok {
		return nil, fmt.Errorf("wrong identifier")
	}

	return &SkinGeometry{
		Description: SkinGeometryDescription{
			Identifier:          identifier,
			TextureWidth:        json.Number(strconv.Itoa(int(geom.TextureWidth))),
			TextureHeight:       json.Number(strconv.Itoa(int(geom.TextureHeight))),
			VisibleBoundsWidth:  geom.VisibleBoundsWidth,
			VisibleBoundsHeight: geom.VisibleBoundsHeight,
			VisibleBoundsOffset: geom.VisibleBoundsOffset,
		},
		Bones: geom.Bones,
	}, nil
}

func (skin *Skin) ParseGeometry() (identifier string, format_version string, geometry *SkinGeometry, err error) {
	var resourcePatch map[string]map[string]string
	if len(skin.SkinResourcePatch) > 0 {
		err := utils.ParseJson(skin.SkinResourcePatch, &resourcePatch)
		if err != nil {
			return "", "", nil, err
		}
	}
	if resourcePatch != nil {
		identifier = resourcePatch["geometry"]["default"]
	}

	if len(skin.SkinGeometry) == 0 {
		return identifier, "", nil, nil
	}

	var ver struct {
		FormatVersion string `json:"format_version"`
	}
	if err := utils.ParseJson(skin.SkinGeometry, &ver); err != nil {
		return "", "", nil, err
	}
	if ver.FormatVersion == "1.8.0" {
		geometry, err := parseGeometry180(skin.SkinGeometry, identifier)
		return identifier, ver.FormatVersion, geometry, err
	} else {
		var data struct {
			Geometry []SkinGeometry `json:"minecraft:geometry"`
		}
		if err := utils.ParseJson(skin.SkinGeometry, &data); err != nil {
			return "", "", nil, err
		}
		if len(data.Geometry) == 0 {
			return identifier, ver.FormatVersion, nil, nil
		}
		var geometry *SkinGeometry
		if identifier == "" {
			geometry = &data.Geometry[0]
			identifier = geometry.Description.Identifier
		} else {
			for _, geom := range data.Geometry {
				if geom.Description.Identifier == identifier {
					geometry = &geom
					break
				}
			}
		}
		return identifier, ver.FormatVersion, geometry, nil
	}
}
