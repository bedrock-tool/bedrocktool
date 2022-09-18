package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path"

	"github.com/flytam/filenamify"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

type Skin struct {
	protocol.Skin
}

// WriteGeometry writes the geometry json for the skin to output_path
func (skin *Skin) WriteGeometry(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("failed to write Geometry %s: %s", output_path, err)
	}
	defer f.Close()
	io.Copy(f, bytes.NewReader(skin.SkinGeometry))
	return nil
}

// WriteCape writes the cape as a png at output_path
func (skin *Skin) WriteCape(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("failed to write Cape %s: %s", output_path, err)
	}
	defer f.Close()
	cape_tex := image.NewRGBA(image.Rect(0, 0, int(skin.CapeImageWidth), int(skin.CapeImageHeight)))
	cape_tex.Pix = skin.CapeData

	if err := png.Encode(f, cape_tex); err != nil {
		return fmt.Errorf("error writing skin: %s", err)
	}
	return nil
}

// WriteAnimations writes skin animations to the folder
func (skin *Skin) WriteAnimations(output_path string) error {
	logrus.Warnf("%s has animations (unimplemented)", output_path)
	return nil
}

// WriteTexture writes the main texture for this skin to a file
func (skin *Skin) WriteTexture(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("error writing Texture: %s", err)
	}
	defer f.Close()
	skin_tex := image.NewRGBA(image.Rect(0, 0, int(skin.SkinImageWidth), int(skin.SkinImageHeight)))
	skin_tex.Pix = skin.SkinData

	if err := png.Encode(f, skin_tex); err != nil {
		return fmt.Errorf("error writing Texture: %s", err)
	}
	return nil
}

func (skin *Skin) WriteTint(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("failed to write Tint %s: %s", output_path, err)
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(skin.PieceTintColours)
	if err != nil {
		return fmt.Errorf("failed to write Tint %s: %s", output_path, err)
	}
	return nil
}

func (skin *Skin) WriteMeta(output_path string) error {
	f, err := os.Create(output_path)
	if err != nil {
		return fmt.Errorf("failed to write Tint %s: %s", output_path, err)
	}
	defer f.Close()
	d, err := json.MarshalIndent(struct {
		SkinID        string
		PlayFabID     string
		PremiumSkin   bool
		PersonaSkin   bool
		CapeID        string
		SkinColour    string
		ArmSize       string
		Trusted       bool
		PersonaPieces []protocol.PersonaPiece
	}{
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

// Write writes all data for this skin to a folder
func (skin *Skin) Write(output_path, name string) error {
	name, _ = filenamify.FilenamifyV2(name)
	skin_dir := path.Join(output_path, name)

	have_geometry, have_cape, have_animations, have_tint := len(skin.SkinGeometry) > 0, len(skin.CapeData) > 0, len(skin.Animations) > 0, len(skin.PieceTintColours) > 0

	os.MkdirAll(skin_dir, 0o755)
	if have_geometry {
		if err := skin.WriteGeometry(path.Join(skin_dir, "geometry.json")); err != nil {
			return err
		}
	}
	if have_cape {
		if err := skin.WriteCape(path.Join(skin_dir, "cape.png")); err != nil {
			return err
		}
	}
	if have_animations {
		if err := skin.WriteAnimations(skin_dir); err != nil {
			return err
		}
	}
	if have_tint {
		if err := skin.WriteTint(path.Join(skin_dir, "tint.json")); err != nil {
			return err
		}
	}

	if err := skin.WriteMeta(path.Join(skin_dir, "metadata.json")); err != nil {
		return err
	}

	return skin.WriteTexture(path.Join(skin_dir, "skin.png"))
}

type skin_anim struct {
	ImageWidth, ImageHeight uint32
	ImageData               string
	AnimationType           uint32
	FrameCount              float32
	ExpressionType          uint32
}

type jsonSkinData struct {
	SkinID                          string
	PlayFabID                       string
	SkinResourcePatch               string
	SkinImageWidth, SkinImageHeight uint32
	SkinData                        string
	Animations                      []skin_anim
	CapeImageWidth, CapeImageHeight uint32
	CapeData                        string
	SkinGeometry                    string
	AnimationData                   string
	GeometryDataEngineVersion       string
	PremiumSkin                     bool
	PersonaSkin                     bool
	PersonaCapeOnClassicSkin        bool
	PrimaryUser                     bool
	CapeID                          string
	FullID                          string
	SkinColour                      string
	ArmSize                         string
	PersonaPieces                   []protocol.PersonaPiece
	PieceTintColours                []protocol.PersonaPieceTintColour
	Trusted                         bool
}

func (s *Skin) Json() *jsonSkinData {
	var skin_animations []skin_anim
	for _, sa := range s.Animations {
		skin_animations = append(skin_animations, skin_anim{
			ImageWidth:     sa.ImageWidth,
			ImageHeight:    sa.ImageHeight,
			ImageData:      base64.RawStdEncoding.EncodeToString(sa.ImageData),
			AnimationType:  sa.AnimationType,
			FrameCount:     sa.FrameCount,
			ExpressionType: sa.ExpressionType,
		})
	}
	return &jsonSkinData{
		SkinID:                    s.SkinID,
		PlayFabID:                 s.PlayFabID,
		SkinResourcePatch:         base64.RawStdEncoding.EncodeToString(s.SkinResourcePatch),
		SkinImageWidth:            s.SkinImageWidth,
		SkinImageHeight:           s.SkinImageHeight,
		SkinData:                  base64.RawStdEncoding.EncodeToString(s.SkinData),
		Animations:                skin_animations,
		CapeImageWidth:            s.CapeImageWidth,
		CapeImageHeight:           s.CapeImageHeight,
		CapeData:                  base64.RawStdEncoding.EncodeToString(s.CapeData),
		SkinGeometry:              base64.RawStdEncoding.EncodeToString(s.SkinGeometry),
		AnimationData:             base64.RawStdEncoding.EncodeToString(s.AnimationData),
		GeometryDataEngineVersion: base64.RawStdEncoding.EncodeToString(s.GeometryDataEngineVersion),
		PremiumSkin:               s.PremiumSkin,
		PersonaSkin:               s.PersonaSkin,
		PersonaCapeOnClassicSkin:  s.PersonaCapeOnClassicSkin,
		PrimaryUser:               s.PrimaryUser,
		CapeID:                    s.CapeID,
		FullID:                    s.FullID,
		SkinColour:                s.SkinColour,
		ArmSize:                   s.ArmSize,
		PersonaPieces:             s.PersonaPieces,
		PieceTintColours:          s.PieceTintColours,
		Trusted:                   s.Trusted,
	}
}
