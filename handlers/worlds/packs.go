package worlds

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/flytam/filenamify"
	"github.com/sirupsen/logrus"
)

func (w *worldsHandler) AddPacks(folder string) {
	type dep struct {
		PackID  string `json:"pack_id"`
		Version [3]int `json:"version"`
	}
	addPacksJSON := func(name string, deps []dep) {
		f, err := os.Create(path.Join(folder, name))
		if err != nil {
			logrus.Error(err)
			return
		}
		defer f.Close()
		if err := json.NewEncoder(f).Encode(deps); err != nil {
			logrus.Error(err)
			return
		}
	}

	// save behaviourpack
	if w.bp.HasContent() {
		name := strings.ReplaceAll(w.serverState.Name, "./", "")
		name = strings.ReplaceAll(name, "/", "-")
		name = strings.ReplaceAll(name, ":", "_")
		packFolder := path.Join(folder, "behavior_packs", name)
		_ = os.MkdirAll(packFolder, 0o755)

		for _, p := range w.proxy.Server.ResourcePacks() {
			p := utils.PackFromBase(p)
			w.bp.CheckAddLink(p)
		}

		w.bp.Save(packFolder, w.blockStates)
		addPacksJSON("world_behavior_packs.json", []dep{{
			PackID:  w.bp.Manifest.Header.UUID,
			Version: w.bp.Manifest.Header.Version,
		}})

		// force resource packs for worlds with custom blocks
		w.settings.WithPacks = true
	}

	// add resource packs
	if w.settings.WithPacks {
		packNames := make(map[string]int)
		for _, pack := range w.serverState.packs {
			packNames[pack.Base().Name()] += 1
		}

		var rdeps []dep
		for _, pack := range w.serverState.packs {
			if pack.Base().Encrypted() && !pack.CanDecrypt() {
				logrus.Warnf("Cant add %s, it is encrypted", pack.Base().Name())
				continue
			}
			logrus.Infof(locale.Loc("adding_pack", locale.Strmap{"Name": pack.Base().Name()}))

			packName := pack.Base().Name()
			if packNames[packName] > 1 {
				packName += "_" + pack.Base().UUID()
			}
			packName, _ = filenamify.FilenamifyV2(packName)
			packFolder := path.Join(folder, "resource_packs", packName)
			os.MkdirAll(packFolder, 0o755)
			err := extractPack(pack, packFolder)
			if err != nil {
				logrus.Error(err)
			}

			rdeps = append(rdeps, dep{
				PackID:  pack.Base().Manifest().Header.UUID,
				Version: pack.Base().Manifest().Header.Version,
			})
		}
		if len(rdeps) > 0 {
			addPacksJSON("world_resource_packs.json", rdeps)
		}
	}
}

func extractPack(p utils.Pack, folder string) error {
	fs, names, err := p.FS()
	if err != nil {
		return err
	}
	for _, name := range names {
		f, err := fs.Open(name)
		if err != nil {
			logrus.Error(err)
			continue
		}
		outPath := path.Join(folder, name)
		os.MkdirAll(path.Dir(outPath), 0777)
		w, err := os.Create(outPath)
		if err != nil {
			f.Close()
			logrus.Error(err)
			continue
		}
		io.Copy(w, f)
		f.Close()
		w.Close()
	}
	return nil
}
