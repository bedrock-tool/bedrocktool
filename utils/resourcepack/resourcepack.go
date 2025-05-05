package resourcepack

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"path"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/skinconverter"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
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
	RenderControllers []string          `json:"render_controllers"`
}

func New() *Pack {
	p := Pack{
		Manifest: buildManifest(uuid.New(), uuid.New()),
		Files:    make(map[string][]byte),
	}
	return &p
}

type EntityPlayer struct {
	Identifier string
	UUID       uuid.UUID
}

func (p *Pack) MakePlayers(players []EntityPlayer, playerSkins map[uuid.UUID]*protocol.Skin) error {
	for _, player := range players {
		skin := skinconverter.Skin{Skin: playerSkins[player.UUID]}

		skinTexture := image.NewNRGBA(image.Rect(0, 0, int(skin.SkinImageWidth), int(skin.SkinImageHeight)))
		copy(skinTexture.Pix, skin.SkinData)

		var capeTexture *image.NRGBA
		if skin.CapeID != "" {
			capeTexture = image.NewNRGBA(image.Rect(0, 0, int(skin.CapeImageWidth), int(skin.CapeImageHeight)))
			copy(capeTexture.Pix, skin.CapeData)
		}

		var skinTextureName = path.Join("textures", "player", player.UUID.String())
		var capeTextureName = "textures/entity/cape_invisible"

		textureData := bytes.NewBuffer(nil)
		png.Encode(textureData, skinTexture)
		p.Files[skinTextureName+".png"] = textureData.Bytes()

		if capeTexture != nil {
			capeTextureName = path.Join("textures", "player", "cape_"+player.UUID.String())
			textureData := bytes.NewBuffer(nil)
			png.Encode(textureData, skinTexture)
			p.Files[capeTextureName+".png"] = textureData.Bytes()
		}

		geometryIdentifier, formatVersion, geometry, err := skin.ParseGeometry()
		if err != nil {
			logrus.Error(err)
			continue
		}

		repl := strings.NewReplacer("-", "_", "+", "_")
		geometryIdentifier = repl.Replace(geometryIdentifier)

		if geometry != nil {
			geometry.Description.Identifier = geometryIdentifier
			p.Files[path.Join("models", "player", geometryIdentifier+".json")], _ = json.Marshal(skinconverter.SkinGeometryFile{
				FormatVersion: formatVersion,
				Geometry:      []skinconverter.SkinGeometry{*geometry},
			})
		}

		var clientEntity = ClientEntityFile{
			FormatVersion: "1.10.0",
			ClientEntity: &ClientEntity{
				Description: &ClientEntityDescription{
					Identifier: player.Identifier,
					Materials: map[string]string{
						"default": "entity_alphatest",
						"cape":    "entity_alphatest",
					},
					Textures: map[string]string{
						"default": skinTextureName,
						"cape":    capeTextureName,
					},
					Geometry: map[string]string{
						"default": geometryIdentifier,
						"cape":    "geometry.cape",
					},
					RenderControllers: []string{"controller.render.player.third_person"},
					Scripts: map[string]any{
						"initialize": []string{
							"variable.is_holding_right = 0.0;",
							"variable.is_blinking = 0.0;",
							"variable.last_blink_time = 0.0;",
							"variable.hand_bob = 0.0;",
						},
						"pre_animation": []string{
							"variable.helmet_layer_visible = !query.has_head_gear;",
							"variable.leg_layer_visible = 1.0;",
							"variable.boot_layer_visible = 1.0;",
							"variable.chest_layer_visible = 1.0;",
							"variable.attack_body_rot_y = Math.sin(360*Math.sqrt(variable.attack_time)) * 5.0;",
							"variable.tcos0 = (math.cos(query.modified_distance_moved * 38.17) * query.modified_move_speed / variable.gliding_speed_value) * 57.3;",
							"variable.first_person_rotation_factor = math.sin((1 - variable.attack_time) * 180.0);",
							"variable.hand_bob = query.life_time < 0.01 ? 0.0 : variable.hand_bob + ((query.is_on_ground && query.is_alive ? math.clamp(math.sqrt(math.pow(query.position_delta(0), 2.0) + math.pow(query.position_delta(2), 2.0)), 0.0, 0.1) : 0.0) - variable.hand_bob) * 0.02;",
							"variable.map_angle = math.clamp(1 - variable.player_x_rotation / 45.1, 0.0, 1.0);",
							"variable.item_use_normalized = query.main_hand_item_use_duration / query.main_hand_item_max_duration;",
						},
						"animate": []string{
							"root",
						},
					},
					Animations: map[string]string{
						"root":                              "controller.animation.player.root",
						"base_controller":                   "controller.animation.player.base",
						"hudplayer":                         "controller.animation.player.hudplayer",
						"humanoid_base_pose":                "animation.humanoid.base_pose",
						"look_at_target":                    "controller.animation.humanoid.look_at_target",
						"look_at_target_ui":                 "animation.player.look_at_target.ui",
						"look_at_target_default":            "animation.humanoid.look_at_target.default",
						"look_at_target_gliding":            "animation.humanoid.look_at_target.gliding",
						"look_at_target_swimming":           "animation.humanoid.look_at_target.swimming",
						"look_at_target_inverted":           "animation.player.look_at_target.inverted",
						"cape":                              "animation.player.cape",
						"move.arms":                         "animation.player.move.arms",
						"move.legs":                         "animation.player.move.legs",
						"swimming":                          "animation.player.swim",
						"swimming.legs":                     "animation.player.swim.legs",
						"riding.arms":                       "animation.player.riding.arms",
						"riding.legs":                       "animation.player.riding.legs",
						"holding":                           "animation.player.holding",
						"brandish_spear":                    "animation.humanoid.brandish_spear",
						"charging":                          "animation.humanoid.charging",
						"attack.positions":                  "animation.player.attack.positions",
						"attack.rotations":                  "animation.player.attack.rotations",
						"sneaking":                          "animation.player.sneaking",
						"bob":                               "animation.player.bob",
						"damage_nearby_mobs":                "animation.humanoid.damage_nearby_mobs",
						"bow_and_arrow":                     "animation.humanoid.bow_and_arrow",
						"use_item_progress":                 "animation.humanoid.use_item_progress",
						"skeleton_attack":                   "animation.skeleton.attack",
						"sleeping":                          "animation.player.sleeping",
						"first_person_base_pose":            "animation.player.first_person.base_pose",
						"first_person_empty_hand":           "animation.player.first_person.empty_hand",
						"first_person_swap_item":            "animation.player.first_person.swap_item",
						"first_person_attack_controller":    "controller.animation.player.first_person_attack",
						"first_person_attack_rotation":      "animation.player.first_person.attack_rotation",
						"first_person_attack_rotation_item": "animation.player.first_person.attack_rotation_item",
						"first_person_vr_attack_rotation":   "animation.player.first_person.vr_attack_rotation",
						"first_person_walk":                 "animation.player.first_person.walk",
						"first_person_map_controller":       "controller.animation.player.first_person_map",
						"first_person_map_hold":             "animation.player.first_person.map_hold",
						"first_person_map_hold_attack":      "animation.player.first_person.map_hold_attack",
						"first_person_map_hold_off_hand":    "animation.player.first_person.map_hold_off_hand",
						"first_person_map_hold_main_hand":   "animation.player.first_person.map_hold_main_hand",
						"first_person_crossbow_equipped":    "animation.player.first_person.crossbow_equipped",
						"first_person_crossbow_hold":        "animation.player.first_person.crossbow_hold",
						"first_person_breathing_bob":        "animation.player.first_person.breathing_bob",
						"third_person_crossbow_equipped":    "animation.player.crossbow_equipped",
						"third_person_bow_equipped":         "animation.player.bow_equipped",
						"crossbow_hold":                     "animation.player.crossbow_hold",
						"crossbow_controller":               "controller.animation.player.crossbow",
						"shield_block_main_hand":            "animation.player.shield_block_main_hand",
						"shield_block_off_hand":             "animation.player.shield_block_off_hand",
						"blink":                             "controller.animation.persona.blink",
						"fishing_rod":                       "animation.humanoid.fishing_rod",
						"holding_spyglass":                  "animation.humanoid.holding_spyglass",
						"first_person_shield_block":         "animation.player.first_person.shield_block",
						"tooting_goat_horn":                 "animation.humanoid.tooting_goat_horn",
						"holding_brush":                     "animation.humanoid.holding_brush",
						"brushing":                          "animation.humanoid.brushing",
						"crawling":                          "animation.player.crawl",
						"crawling.legs":                     "animation.player.crawl.legs",
						"holding_heavy_core":                "animation.player.holding_heavy_core",
					},
				},
			},
		}

		clientEntityData, err := json.Marshal(&clientEntity)
		if err != nil {
			return err
		}
		p.Files[path.Join("entity", "players", player.UUID.String()+".json")] = clientEntityData
	}

	return nil
}

func (p *Pack) WriteToFS(fs utils.WriterFS) error {
	f, err := fs.Create("manifest.json")
	if err != nil {
		return err
	}
	_ = json.NewEncoder(f).Encode(&p.Manifest)
	f.Close()

	for name, content := range p.Files {
		f, err := fs.Create(name)
		if err != nil {
			return err
		}
		f.Write(content)
		f.Close()
	}
	return nil
}
