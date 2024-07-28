package worldstate

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/flytam/filenamify"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sirupsen/logrus"
)

func addPacksJSON(fs utils.WriterFS, name string, deps []resourcePackDependency) error {
	f, err := fs.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(deps); err != nil {
		return err
	}
	return nil
}

var removeSpace = strings.NewReplacer(" ", "")

func formatPackName(packName string) string {
	packName = text.Clean(packName)
	packName, _ = filenamify.FilenamifyV2(packName)
	packName = removeSpace.Replace(packName)
	packName = packName[:min(10, len(packName))]
	return packName
}

func (w *World) addResourcePacks() error {
	packNames := make(map[string][]string)
	for _, pack := range w.ResourcePacks {
		packName := formatPackName(pack.Name())
		packNames[packName] = append(packNames[packName], pack.UUID())
	}

	for _, pack := range w.ResourcePacks {
		log := w.log.WithField("pack", pack.Name())
		if pack.Encrypted() && !pack.CanRead() {
			log.Warn("Cant add is encrypted")
			continue
		}
		logrus.Infof(locale.Loc("adding_pack", locale.Strmap{"Name": text.Clean(pack.Name())}))

		messages.Router.Handle(&messages.Message{
			Source: "subcommand",
			Target: "ui",
			Data: messages.ProcessingWorldUpdate{
				Name:  w.Name,
				State: "Adding Resourcepack " + text.Clean(pack.Name()),
			},
		})

		packName := formatPackName(pack.Name())
		if packIds := packNames[packName]; len(packIds) > 1 {
			packName = fmt.Sprintf("%s_%d", packName[:8], slices.Index(packIds, pack.UUID()))
		}

		err := utils.CopyFS(pack, utils.SubFS(utils.OSWriter{Base: w.Folder}, path.Join("resource_packs", packName)))
		if err != nil {
			log.Error(err)
			continue
		}

		w.resourcePackDependencies = append(w.resourcePackDependencies, resourcePackDependency{
			UUID:    pack.Manifest().Header.UUID,
			Version: pack.Manifest().Header.Version,
		})
	}

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.ProcessingWorldUpdate{
			Name:  w.Name,
			State: "",
		},
	})

	return nil
}

func (w *World) FinalizePacks(addBehaviorPack func(fs utils.WriterFS) (*resource.Header, error)) error {
	err := <-w.resourcePacksDone
	if err != nil {
		return err
	}

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.ProcessingWorldUpdate{
			Name:  w.Name,
			State: "Adding Behaviorpack",
		},
	})

	fs := utils.OSWriter{Base: w.Folder}
	header, err := addBehaviorPack(fs)
	if err != nil {
		return err
	}

	if header != nil {
		err = addPacksJSON(fs, "world_behavior_packs.json", []resourcePackDependency{{
			UUID:    header.UUID,
			Version: header.Version,
		}})
		if err != nil {
			return err
		}
	}

	if len(w.resourcePackDependencies) > 0 {
		err := addPacksJSON(fs, "world_resource_packs.json", w.resourcePackDependencies)
		if err != nil {
			return err
		}
	}

	return nil
}
