package worlds

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/flytam/filenamify"
	"github.com/sirupsen/logrus"
)

func (w *worldsHandler) AddPacks(fs utils.WriterFS) error {
	type dep struct {
		PackID  string `json:"pack_id"`
		Version [3]int `json:"version"`
	}
	addPacksJSON := func(name string, deps []dep) error {
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

	// save behaviourpack
	if w.bp.HasContent() {
		name, err := filenamify.FilenamifyV2(w.serverState.Name)
		if err != nil {
			return err
		}
		packFolder := path.Join("behavior_packs", name)

		for _, p := range w.proxy.Server.ResourcePacks() {
			p := utils.PackFromBase(p)
			w.bp.CheckAddLink(p)
		}

		err = w.bp.Save(fs, packFolder)
		if err != nil {
			return err
		}

		err = addPacksJSON("world_behavior_packs.json", []dep{{
			PackID:  w.bp.Manifest.Header.UUID,
			Version: w.bp.Manifest.Header.Version,
		}})
		if err != nil {
			return err
		}

		// force resource packs for worlds with custom blocks
		w.settings.WithPacks = true
	}

	// add resource packs
	if w.settings.WithPacks {
		packNames := make(map[string][]string)
		for _, pack := range w.serverState.packs {
			packName := pack.Base().Name()
			packNames[packName] = append(packNames[packName], pack.Base().UUID())
		}

		var rdeps []dep
		for _, pack := range w.serverState.packs {
			log := w.log.WithField("pack", pack.Base().Name())
			if pack.Base().Encrypted() && !pack.CanDecrypt() {
				log.Warn("Cant add is encrypted")
				continue
			}
			logrus.Infof(locale.Loc("adding_pack", locale.Strmap{"Name": pack.Base().Name()}))

			packName := pack.Base().Name()
			if packIds := packNames[packName]; len(packIds) > 1 {
				packName = fmt.Sprintf("%s_%d", packName, slices.Index(packIds, pack.Base().UUID()))
			}
			packName, _ = filenamify.FilenamifyV2(packName)
			err := writePackToFs(pack, fs, path.Join("resource_packs", packName))
			if err != nil {
				log.Error(err)
				continue
			}

			rdeps = append(rdeps, dep{
				PackID:  pack.Base().Manifest().Header.UUID,
				Version: pack.Base().Manifest().Header.Version,
			})
		}

		if len(w.rp.Files) > 0 && w.settings.Players {
			btPlayersFolder := path.Join("resource_packs", "bt_players")
			w.rp.WriteToDir(fs, btPlayersFolder)
			rdeps = append(rdeps, dep{
				PackID:  w.rp.Manifest.Header.UUID,
				Version: w.rp.Manifest.Header.Version,
			})
		}

		if len(rdeps) > 0 {
			err := addPacksJSON("world_resource_packs.json", rdeps)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writePackToFs(pack utils.Pack, fs utils.WriterFS, dir string) error {
	packFS, packFiles, err := pack.FS()
	if err != nil {
		return err
	}

	addFile := func(filename string) error {
		file, err := packFS.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		f, err := fs.Create(path.Join(dir, filename))
		if err != nil {
			return err
		}
		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}
		return nil
	}

	for _, filename := range packFiles {
		err := addFile(filename)
		if err != nil {
			return err
		}
	}
	return nil
}
