package worlds

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
			p, err := utils.PackFromBase(p)
			if err != nil {
				return err
			}
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
			packName := pack.Name()
			packNames[packName] = append(packNames[packName], pack.UUID())
		}

		var rdeps []dep
		for _, pack := range w.serverState.packs {
			log := w.log.WithField("pack", pack.Name())
			if pack.Encrypted() && !pack.CanDecrypt() {
				log.Warn("Cant add is encrypted")
				continue
			}
			logrus.Infof(locale.Loc("adding_pack", locale.Strmap{"Name": pack.Name()}))

			packName := pack.Name()
			if packIds := packNames[packName]; len(packIds) > 1 {
				packName = fmt.Sprintf("%s_%d", packName, slices.Index(packIds, pack.UUID()))
			}
			packName, _ = filenamify.FilenamifyV2(packName)
			err := writePackToFs(pack, fs, path.Join("resource_packs", packName))
			if err != nil {
				log.Error(err)
				continue
			}

			rdeps = append(rdeps, dep{
				PackID:  pack.Manifest().Header.UUID,
				Version: pack.Manifest().Header.Version,
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

func writePackToFs(pack utils.Pack, wfs utils.WriterFS, dir string) error {
	return fs.WalkDir(pack, ".", func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		r, err := pack.Open(fpath)
		if err != nil {
			return err
		}
		defer r.Close()
		w, err := wfs.Create(path.Join(dir, fpath))
		if err != nil {
			return err
		}
		defer w.Close()
		_, err = io.Copy(w, r)
		if err != nil {
			return err
		}
		return nil
	})
}
