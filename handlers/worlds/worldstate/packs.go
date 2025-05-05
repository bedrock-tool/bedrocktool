package worldstate

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
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

func (w *World) addResourcePacks() error {
	packNames := make(map[string][]string)
	for _, pack := range w.ResourcePacks {
		packName := utils.FormatPackName(pack.Name())
		packNames[packName] = append(packNames[packName], pack.UUID().String())
	}

	for _, pack := range w.ResourcePacks {
		log := w.log.WithField("pack", pack.Name())
		if pack.Encrypted() && !pack.CanRead() {
			log.Warn("Cant add is encrypted")
			continue
		}
		logrus.Info(locale.Loc("adding_pack", locale.Strmap{"Name": text.Clean(pack.Name())}))

		messages.SendEvent(&messages.EventProcessingWorldUpdate{
			WorldName: w.Name,
			State:     "Adding Resourcepack " + text.Clean(pack.Name()),
		})

		packName := utils.FormatPackName(pack.Name())
		if packIds := packNames[packName]; len(packIds) > 1 {
			packName = fmt.Sprintf("%s_%d", packName[:8], slices.Index(packIds, pack.UUID().String()))
		}

		err := utils.CopyFS(pack, utils.SubFS(utils.OSWriter{Base: w.Folder}, path.Join("resource_packs", packName)))
		if err != nil {
			log.Error(err)
			continue
		}
	}

	messages.SendEvent(&messages.EventProcessingWorldUpdate{
		WorldName: w.Name,
		State:     "",
	})

	return nil
}

type addedPack struct {
	BehaviorPack bool
	Header       *resource.Header
}

func (w *World) finalizePacks(addAdditionalPacks func(fs utils.WriterFS) ([]addedPack, error)) error {
	err := <-w.resourcePacksDone
	if err != nil {
		return err
	}

	messages.SendEvent(&messages.EventProcessingWorldUpdate{
		WorldName: w.Name,
		State:     "Adding Behaviorpack",
	})

	fs := utils.OSWriter{Base: w.Folder}
	additionalPacks, err := addAdditionalPacks(fs)
	if err != nil {
		return err
	}

	var resourcePackDependencies []resourcePackDependency
	for _, pack := range w.ResourcePacks {
		resourcePackDependencies = append(resourcePackDependencies, resourcePackDependency{
			UUID:    pack.Manifest().Header.UUID.String(),
			Version: pack.Manifest().Header.Version,
		})
	}

	var behaviorPackDependencies []resourcePackDependency
	for _, p := range additionalPacks {
		dep := resourcePackDependency{
			UUID:    p.Header.UUID.String(),
			Version: p.Header.Version,
		}
		if p.BehaviorPack {
			behaviorPackDependencies = append(behaviorPackDependencies, dep)
		} else {
			resourcePackDependencies = append(resourcePackDependencies, dep)
		}
	}

	if len(behaviorPackDependencies) > 0 {
		err := addPacksJSON(fs, "world_behavior_packs.json", behaviorPackDependencies)
		if err != nil {
			return err
		}
	}

	if len(resourcePackDependencies) > 0 {
		err := addPacksJSON(fs, "world_resource_packs.json", resourcePackDependencies)
		if err != nil {
			return err
		}
	}

	return nil
}
