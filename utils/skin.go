package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type Skin struct {
	*protocol.Skin
}

type SkinGeometry struct {
	SkinGeometryDescription
	Bones []any `json:"bones"`
}

type SkinGeometryDescription struct {
	Identifier          string    `json:"identifier,omitempty"`
	Texturewidth        int       `json:"texturewidth"`
	Textureheight       int       `json:"textureheight"`
	VisibleBoundsWidth  float64   `json:"visible_bounds_width"`
	VisibleBoundsHeight float64   `json:"visible_bounds_height"`
	VisibleBoundsOffset []float64 `json:"visible_bounds_offset,omitempty"`
}

type SkinGeometry_1_12 struct {
	Description SkinGeometryDescription `json:"description"`
	Bones       []any                   `json:"bones"`
}

func (skin *Skin) Hash() uuid.UUID {
	h := append(skin.CapeData, append(skin.SkinData, skin.SkinGeometry...)...)
	return uuid.NewSHA1(uuid.NameSpaceURL, h)
}

func (skin *Skin) getGeometry() (*SkinGeometry_1_12, string, error) {
	if !skin.HaveGeometry() {
		return nil, "", errors.New("no geometry")
	}

	var data map[string]any
	if err := json.Unmarshal(skin.SkinGeometry, &data); err != nil {
		return nil, "", err
	}

	if len(data) == 0 {
		return nil, "", nil
	}

	arr, ok := data["minecraft:geometry"].([]any)
	if !ok {
		return nil, "", errors.New("invalid geometry")
	}
	geom, ok := arr[0].(map[string]any)
	if !ok {
		return nil, "", errors.New("invalid geometry")
	}

	desc, ok := geom["description"].(map[string]any)
	if !ok {
		return nil, "", errors.New("invalid geometry")
	}

	texture_width, _ := desc["texture_width"].(float64)
	texture_height, _ := desc["texture_height"].(float64)
	visible_bounds_width, _ := desc["visible_bounds_width"].(float64)
	visible_bounds_height, _ := desc["visible_bounds_height"].(float64)
	visibleOffset, _ := desc["visible_bounds_offset"].([]float64)

	return &SkinGeometry_1_12{
		Description: SkinGeometryDescription{
			Identifier:          desc["identifier"].(string),
			Texturewidth:        int(texture_width),
			Textureheight:       int(texture_height),
			VisibleBoundsWidth:  visible_bounds_width,
			VisibleBoundsHeight: visible_bounds_height,
			VisibleBoundsOffset: visibleOffset,
		},
		Bones: geom["bones"].([]any),
	}, desc["identifier"].(string), nil
}

// WriteCape writes the cape as a png at output_path
func (skin *Skin) WriteCapePng(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return errors.New(locale.Loc("failed_write", locale.Strmap{"Part": "Cape", "Path": output_path, "Err": err}))
	}
	defer f.Close()
	cape_tex := image.NewRGBA(image.Rect(0, 0, int(skin.CapeImageWidth), int(skin.CapeImageHeight)))
	cape_tex.Pix = skin.CapeData

	if err := png.Encode(f, cape_tex); err != nil {
		return fmt.Errorf(locale.Loc("failed_write", locale.Strmap{"Part": "Cape", "Err": err}))
	}
	return nil
}

// WriteTexture writes the main texture for this skin to a file
func (skin *Skin) writeSkinTexturePng(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return errors.New(locale.Loc("failed_write", locale.Strmap{"Part": "Meta", "Path": output_path, "Err": err}))
	}
	defer f.Close()
	skin_tex := image.NewRGBA(image.Rect(0, 0, int(skin.SkinImageWidth), int(skin.SkinImageHeight)))
	skin_tex.Pix = skin.SkinData

	if err := png.Encode(f, skin_tex); err != nil {
		return errors.New(locale.Loc("failed_write", locale.Strmap{"Part": "Texture", "Path": output_path, "Err": err}))
	}
	return nil
}

func (skin *Skin) writeMetadataJson(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return errors.New(locale.Loc("failed_write", locale.Strmap{"Part": "Meta", "Path": output_path, "Err": err}))
	}
	defer f.Close()
	d, err := json.MarshalIndent(SkinMeta{
		skin.SkinID,
		skin.PlayFabID,
		skin.PremiumSkin,
		skin.PersonaSkin,
		skin.CapeID,
		skin.SkinColour,
		skin.ArmSize,
		skin.Trusted,
		skin.PersonaPieces,
	}, "", "    ")
	if err != nil {
		return err
	}
	f.Write(d)
	return nil
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
