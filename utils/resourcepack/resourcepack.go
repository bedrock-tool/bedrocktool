package resourcepack

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type Pack struct {
	Manifest resource.Manifest
	Files    map[string][]byte
}

// parseVersion parses the version passed in the format of a.b.c as a [3]int.
func parseVersion(ver string) [3]int {
	frag := strings.Split(ver, ".")
	if len(frag) != 3 {
		panic("invalid version number " + ver)
	}
	a, _ := strconv.ParseInt(frag[0], 10, 64)
	b, _ := strconv.ParseInt(frag[1], 10, 64)
	c, _ := strconv.ParseInt(frag[2], 10, 64)
	return [3]int{int(a), int(b), int(c)}
}

func buildManifest(headerUUID, moduleUUID uuid.UUID) resource.Manifest {
	return resource.Manifest{
		FormatVersion: 2,
		Header: resource.Header{
			Name:               "auto-generated resource pack",
			Description:        "This resource pack contains auto-generated content from dragonfly",
			UUID:               headerUUID,
			Version:            [3]int{0, 0, 1},
			MinimumGameVersion: parseVersion(protocol.CurrentVersion),
		},
		Modules: []resource.Module{
			{
				UUID:        moduleUUID.String(),
				Description: "This resource pack contains auto-generated content from dragonfly",
				Type:        "resources",
				Version:     [3]int{0, 0, 1},
			},
		},
	}
}

type Geometry struct {
	Description utils.SkinGeometryDescription `json:"description"`
	Bones       []any                         `json:"bones"`
}

type ClientEntityFile struct {
	FormatVersion string        `json:"format_version"`
	ClientEntity  *ClientEntity `json:"minecraft:client_entity"`
}

type ClientEntity struct {
	Description *ClientEntityDescription `json:"description"`
}

type ClientEntityDescription struct {
	Identifier        string            `json:"identifier"`
	Materials         map[string]string `json:"materials"`
	Geometry          map[string]string `json:"geometry"`
	Textures          map[string]string `json:"textures"`
	Scripts           map[string]any    `json:"scripts,omitempty"`
	Animations        map[string]string `json:"animations,omitempty"`
	RenderControllers []any             `json:"render_controllers"`
}

func New() *Pack {
	p := Pack{
		Manifest: buildManifest(uuid.New(), uuid.New()),
		Files:    make(map[string][]byte),
	}
	return &p
}

func (p *Pack) AddEntity(dir, name string, texture *image.RGBA, geometry *utils.SkinGeometryFile, isDefault bool) {
	textureData := bytes.NewBuffer(nil)
	png.Encode(textureData, texture)
	textureName := path.Join("textures", dir, name)
	p.Files[textureName+".png"] = textureData.Bytes()

	if !isDefault {
		p.Files[path.Join("models", dir, name+".json")], _ = json.Marshal(geometry)
	}

	p.Files[path.Join("entity", dir, name+".json")], _ = json.Marshal(&ClientEntityFile{
		FormatVersion: "1.10.0",
		ClientEntity: &ClientEntity{
			Description: &ClientEntityDescription{
				Identifier: dir + ":" + name,
				Materials: map[string]string{
					"default": "entity_alphatest",
				},
				Textures: map[string]string{
					"default": textureName,
				},
				Geometry: map[string]string{
					"default": geometry.Geometry[0].Description.Identifier,
				},
			},
		},
	})
}

func (p *Pack) AddPlayer(id string, skinTexture *image.NRGBA, capeTexture *image.NRGBA, capeID string, geometry *utils.SkinGeometryFile, isDefault bool) {
	var skinName = path.Join("textures", "player", id)
	var capeName = "textures/entity/cape_invisible"

	{
		textureData := bytes.NewBuffer(nil)
		png.Encode(textureData, skinTexture)
		p.Files[skinName+".png"] = textureData.Bytes()
	}

	if capeTexture != nil {
		capeName = path.Join("textures", "player", "cape_"+capeID)

		textureData := bytes.NewBuffer(nil)
		png.Encode(textureData, skinTexture)
		p.Files[capeName+".png"] = textureData.Bytes()
	}

	if !isDefault {
		p.Files[path.Join("models", "player", id+".json")], _ = json.Marshal(geometry)
	}

	var clientEntity = ClientEntityFile{
		FormatVersion: "1.10.0",
		ClientEntity: &ClientEntity{
			Description: &ClientEntityDescription{
				Identifier: "player:" + id,
				Materials: map[string]string{
					"default":   "entity_alphatest",
					"cape":      "entity_alphatest",
					"animated":  "player_animated",
					"spectator": "player_spectator",
				},
				Textures: map[string]string{
					"default": skinName,
					"cape":    capeName,
				},
				Geometry: map[string]string{
					"default": geometry.Geometry[0].Description.Identifier,
					"cape":    "geometry.cape",
				},
				//Scripts: map[string]any{
				//	"scale": "0.9375",
				//	"initialize": []string{
				//		"variable.is_holding_right = 0.0;",
				//		"variable.is_blinking = 0.0;",
				//		"variable.last_blink_time = 0.0;",
				//		"variable.hand_bob = 0.0;",
				//		"variable.is_using_vr = false;",
				//		"variable.player_arm_height = 0.0;",
				//		"variable.bob_animation = 0.0;",
				//		"variable.bob_animation = 0.0;",
				//		"variable.is_first_person = false;",
				//	},
				//	"pre_animation": []string{
				//		"variable.helmet_layer_visible = 1.0;",
				//		"variable.leg_layer_visible = 1.0;",
				//		"variable.boot_layer_visible = 1.0;",
				//		"variable.chest_layer_visible = 1.0;",
				//		"variable.attack_body_rot_y = Math.sin(360*Math.sqrt(variable.attack_time)) * 5.0;",
				//		"variable.tcos0 = (math.cos(query.modified_distance_moved * 38.17) * query.modified_move_speed / variable.gliding_speed_value) * 57.3;",
				//		"variable.first_person_rotation_factor = math.sin((1 - variable.attack_time) * 180.0);",
				//		"variable.hand_bob = query.life_time < 0.01 ? 0.0 : variable.hand_bob + ((query.is_on_ground && query.is_alive ? math.clamp(math.sqrt(math.pow(query.position_delta(0), 2.0) + math.pow(query.position_delta(2), 2.0)), 0.0, 0.1) : 0.0) - variable.hand_bob) * 0.02;",
				//
				//		"variable.map_angle = math.clamp(1 - variable.player_x_rotation / 45.1, 0.0, 1.0);",
				//		"variable.item_use_normalized = query.main_hand_item_use_duration / query.main_hand_item_max_duration;",
				//	},
				//	"animate": []string{
				//		"root",
				//	},
				//},
				//Animations: map[string]string{
				//	"root":                    "controller.animation.player.root",
				//	"base_controller":         "controller.animation.player.base",
				//	"hudplayer":               "controller.animation.player.hudplayer",
				//	"humanoid_base_pose":      "animation.humanoid.base_pose",
				//	"look_at_target":          "controller.animation.humanoid.look_at_target",
				//	"look_at_target_ui":       "animation.player.look_at_target.ui",
				//	"look_at_target_default":  "animation.humanoid.look_at_target.default",
				//	"look_at_target_gliding":  "animation.humanoid.look_at_target.gliding",
				//	"look_at_target_swimming": "animation.humanoid.look_at_target.swimming",
				//	"look_at_target_inverted": "animation.player.look_at_target.inverted",
				//	"cape":                    "animation.player.cape",
				//	"move.arms":               "animation.player.move.arms",
				//	"move.legs":               "animation.player.move.legs",
				//	"swimming":                "animation.player.swim",
				//	"swimming.legs":           "animation.player.swim.legs",
				//	"crawling":                "animation.player.crawl",
				//	"crawling.legs":           "animation.player.crawl.legs",
				//	"riding.arms":             "animation.player.riding.arms",
				//	"riding.legs":             "animation.player.riding.legs",
				//	"holding":                 "animation.player.holding",
				//	"brandish_spear":          "animation.humanoid.brandish_spear",
				//	"holding_spyglass":        "animation.humanoid.holding_spyglass",
				//	"charging":                "animation.humanoid.charging",
				//	"attack.positions":        "animation.player.attack.positions",
				//	"attack.rotations":        "animation.player.attack.rotations",
				//	"sneaking":                "animation.player.sneaking",
				//	"bob":                     "animation.player.bob",
				//	"damage_nearby_mobs":      "animation.humanoid.damage_nearby_mobs",
				//	"bow_and_arrow":           "animation.humanoid.bow_and_arrow",
				//	"use_item_progress":       "animation.humanoid.use_item_progress",
				//	"skeleton_attack":         "animation.skeleton.attack",
				//	"sleeping":                "animation.player.sleeping",
				//	//"first_person_base_pose":          "animation.player.first_person.base_pose",
				//	//"first_person_empty_hand":         "animation.player.first_person.empty_hand",
				//	//"first_person_swap_item":          "animation.player.first_person.swap_item",
				//	//"first_person_shield_block":       "animation.player.first_person.shield_block",
				//	//"first_person_attack_controller":  "controller.animation.player.first_person_attack",
				//	//"first_person_attack_rotation":    "animation.player.first_person.attack_rotation",
				//	//"first_person_vr_attack_rotation": "animation.player.first_person.vr_attack_rotation",
				//	//"first_person_walk":               "animation.player.first_person.walk",
				//	//"first_person_map_controller":     "controller.animation.player.first_person_map",
				//	//"first_person_map_hold":           "animation.player.first_person.map_hold",
				//	//"first_person_map_hold_attack":    "animation.player.first_person.map_hold_attack",
				//	//"first_person_map_hold_off_hand":  "animation.player.first_person.map_hold_off_hand",
				//	//"first_person_map_hold_main_hand": "animation.player.first_person.map_hold_main_hand",
				//	//"first_person_crossbow_equipped":  "animation.player.first_person.crossbow_equipped",
				//	"third_person_crossbow_equipped": "animation.player.crossbow_equipped",
				//	"third_person_bow_equipped":      "animation.player.bow_equipped",
				//	"crossbow_hold":                  "animation.player.crossbow_hold",
				//	"crossbow_controller":            "controller.animation.player.crossbow",
				//	"shield_block_main_hand":         "animation.player.shield_block_main_hand",
				//	"shield_block_off_hand":          "animation.player.shield_block_off_hand",
				//	"blink":                          "controller.animation.persona.blink",
				//	"tooting_goat_horn":              "animation.humanoid.tooting_goat_horn",
				//	"holding_brush":                  "animation.humanoid.holding_brush",
				//	"brushing":                       "animation.humanoid.brushing",
				//},
				RenderControllers: []any{"controller.render.player.third_person"},
			},
		},
	}

	p.Files[path.Join("entity", "player", id+".json")], _ = json.Marshal(clientEntity)
}

func (p *Pack) WriteTo(w io.Writer) {
	b := bufio.NewWriter(w)
	zw := zip.NewWriter(b)

	f, err := zw.Create("manifest.json")
	if err != nil {
		panic(err)
	}
	_ = json.NewEncoder(f).Encode(&p.Manifest)

	for name, content := range p.Files {
		f, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		f.Write(content)
	}
	err = zw.Close()
	if err != nil {
		panic(err)
	}
	b.Flush()
}

func (p *Pack) WriteToDir(fs utils.WriterFS, dir string) {
	f, err := fs.Create(path.Join(dir, "manifest.json"))
	if err != nil {
		panic(err)
	}
	_ = json.NewEncoder(f).Encode(&p.Manifest)
	f.Close()

	for name, content := range p.Files {
		fullName := path.Join(dir, name)
		f, err := fs.Create(fullName)
		if err != nil {
			panic(err)
		}
		f.Write(content)
		f.Close()
	}
}
