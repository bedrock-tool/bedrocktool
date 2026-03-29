package subcommands

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sirupsen/logrus"
)

type ResourcePacksSettings struct {
	ProxySettings proxy.ProxySettings `without:"listen"`

	SaveEncrypted bool `opt:"Save Encrypted" flag:"save-encrypted"`
	OnlyKeys      bool `opt:"Only save keys" flag:"only-keys"`
	Folders       bool `opt:"Write Folders" flag:"folders"`
	NhanAZ        bool `opt:"Decrypt & extract (NhanAZ)" flag:"nhanaz"`
}

type ResourcePackCMD struct{}

func (ResourcePackCMD) Name() string {
	return "packs"
}

func (ResourcePackCMD) Description() string {
	return locale.Loc("pack_synopsis", nil)
}

func (ResourcePackCMD) Settings() any {
	return new(ResourcePacksSettings)
}

type packContent struct {
	Content []struct {
		Path string  `json:"path"`
		Key  *string `json:"key"`
	} `json:"content"`
}

// decryptPackToDir replicates the decryption flow:
// 1) contents.json is AES-256-CFB8 encrypted after a 0x100-byte header using the pack content key.
// 2) Each entry may have its own key; if absent the file is copied verbatim (JSON prettified when possible).
// Non-encrypted files (or missing keys) are still copied, so the output folder is always complete.
func decryptPackToDir(pack resource.Pack, outputDir string) error {
	key := []byte(pack.ContentKey())
	if len(key) != 32 {
		return fmt.Errorf("content key must be 32 bytes, got %d", len(key))
	}

	f, err := pack.Open("contents.json")
	if err != nil {
		return fmt.Errorf("open contents.json: %w", err)
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read contents.json: %w", err)
	}
	if len(raw) <= 0x100 {
		return fmt.Errorf("contents.json too small (%d bytes)", len(raw))
	}

	manifestBytes := append([]byte(nil), raw[0x100:]...)
	utils.CfbDecrypt(manifestBytes, key)

	var manifest packContent
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("parse contents.json: %w", err)
	}

	// Write decrypted contents.json back to output folder (prettified).
	var prettyManifest []byte
	if pm, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		prettyManifest = pm
	} else {
		prettyManifest = manifestBytes
	}
	if err := os.MkdirAll(outputDir, 0o775); err == nil {
		_ = os.WriteFile(filepath.Join(outputDir, "contents.json"), prettyManifest, 0o664)
	}

	// sort and dedup paths
	sort.Slice(manifest.Content, func(i, j int) bool { return manifest.Content[i].Path < manifest.Content[j].Path })
	dedup := manifest.Content[:0]
	for _, entry := range manifest.Content {
		if len(dedup) > 0 && dedup[len(dedup)-1].Path == entry.Path {
			continue
		}
		dedup = append(dedup, entry)
	}
	manifest.Content = dedup

	for _, entry := range manifest.Content {
		inputFile, err := pack.Open(entry.Path)
		if err != nil {
			continue // missing file, skip
		}

		data, err := io.ReadAll(inputFile)
		inputFile.Close()
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Path, err)
		}

		if entry.Key != nil {
			entryKey := []byte(*entry.Key)
			if len(entryKey) != 32 {
				logrus.Warnf("Expected key length 32, got %d for %s, skipping decrypt", len(entryKey), entry.Path)
				continue
			}
			buf := append([]byte(nil), data...)
			utils.CfbDecrypt(buf, entryKey)
			data = buf
		}

		// pretty-print JSON when possible
		if strings.HasSuffix(strings.ToLower(entry.Path), ".json") {
			var v any
			if err := json.Unmarshal(data, &v); err == nil {
				data, _ = json.MarshalIndent(v, "", "  ")
			}
		}

		outPath := filepath.Join(outputDir, entry.Path)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o775); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
		}
		if err := os.WriteFile(outPath, data, 0o664); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		logrus.Infof("Decrypted %s", entry.Path)
	}

	logrus.Info("Decryption finished")
	return nil
}

func processPack(outputDir string, packNameCounts map[string]int, pack resource.Pack, packSettings *ResourcePacksSettings) error {
	idx := packNameCounts[pack.Name()]
	packNameCounts[pack.Name()] += 1

	packName := text.Clean(pack.Name())
	packName = strings.ReplaceAll(packName, "\n", " ")
	packUUID := pack.UUID()
	if idx > 0 {
		packName += "_" + strconv.Itoa(idx)
	}
	packFilename := utils.MakeValidFilename(packName)
	logrus.Infof("ResourcePack(Id: %s Key: %s Name: %s %s)", packUUID, pack.ContentKey(), packName, pack.Version())
	messages.SendEvent(&messages.EventProcessingPack{
		ID: pack.UUID().String(),
	})

	if packSettings.SaveEncrypted && pack.Encrypted() {
		f, err := os.Create(filepath.Join(outputDir, packFilename+".zip"))
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err = pack.WriteTo(f); err != nil {
			return err
		}
	}

	var packPath string

	// NhanAZ mode: always extract to folder, decrypt if possible, copy otherwise.
	if packSettings.NhanAZ {
		packPath = filepath.Join(outputDir, packFilename)
		// Always copy full pack first so files not listed in contents.json are preserved.
		if err := utils.CopyFS(pack, utils.OSWriter{Base: packPath}); err != nil {
			return err
		}
		// Then decrypt/overwrite entries that have keys.
		if pack.Encrypted() && pack.ContentKey() != "" {
			if err := decryptPackToDir(pack, packPath); err != nil {
				logrus.Warnf("Failed to decrypt pack %s: %v (kept raw copy)", packName, err)
			}
		}
	} else if packSettings.Folders {
		packPath = filepath.Join(outputDir, packFilename)
		if err := utils.CopyFS(pack, utils.OSWriter{Base: packPath}); err != nil {
			return err
		}
	} else {
		packPath = filepath.Join(outputDir, packFilename+".mcpack")
		f, err := os.Create(packPath)
		if err != nil {
			return err
		}
		defer f.Close()

		if !pack.Encrypted() {
			logrus.Trace("Saving verbatim")
			if _, err := pack.WriteTo(f); err != nil {
				return err
			}
		} else {
			zw := zip.NewWriter(f)
			defer zw.Close()
			utils.ZipCompressPool(zw)
			err = utils.CopyFS(pack, utils.ZipWriter{Writer: zw})
			if err != nil {
				return err
			}
		}
	}

	var icon *image.RGBA
	f, err := pack.Open("pack_icon.png")
	if err == nil {
		defer f.Close()
		iconDec, err := png.Decode(f)
		if err != nil {
			logrus.Warnf("Failed to Parse pack_icon.png %s", pack.Name())
		}
		var ok bool
		icon, ok = iconDec.(*image.RGBA)
		if !ok {
			icon = image.NewRGBA(iconDec.Bounds())
			draw.Draw(icon, iconDec.Bounds(), iconDec, iconDec.Bounds().Min, draw.Src)
		}
	}

	messages.SendEvent(&messages.EventFinishedPack{
		ID:       pack.UUID().String(),
		Name:     pack.Name(),
		Version:  pack.Version(),
		Filepath: packPath,
		Icon:     icon,
	})
	return nil
}

const keysFile = "keys.db"

func dumpKeys(keys map[string]string) error {
	f, err := os.OpenFile(utils.PathData(keysFile), os.O_APPEND|os.O_RDWR|os.O_CREATE, 0o775)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	existingKeys := map[string]string{}
	for _, v := range lines {
		if len(v) == 0 {
			continue
		}
		item := strings.Split(v, "=")
		t1 := strings.TrimSpace(item[0])
		if len(t1) == 0 || t1[0] == []byte("#")[0] {
			continue
		}
		existingKeys[item[0]] = item[1]
	}

	for uuid, key := range keys {
		if key == "" {
			continue
		}
		existing := existingKeys[uuid]
		if existing != "" {
			//logrus.Warn(locale.Loc("warn_key_exists", locale.Strmap{"Id": uuid}))
			if existing != key {
				logrus.Warn(locale.Loc("compare_key", locale.Strmap{"Id": uuid, "Prev": existing, "Now": key}))
			}
			continue
		}
		f.WriteString(uuid + "=" + key + "\n")
	}
	return nil
}

type resourcePackHandler struct {
	packSettings *ResourcePacksSettings
}

func (r *resourcePackHandler) onPacket(s *proxy.Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	switch pk := pk.(type) {
	case *packet.ResourcePacksInfo:
		keys := make(map[string]string)
		for _, p := range pk.TexturePacks {
			if len(p.ContentKey) > 0 {
				keys[p.UUID.String()] = p.ContentKey
			}
		}
		if len(keys) > 0 {
			logrus.WithField("Count", len(keys)).Info(locale.Loc("writing_keys", locale.Strmap{"Path": keysFile}))
			err := dumpKeys(keys)
			if err != nil {
				logrus.Errorf("Error Dumping Keys: %s", err)
			}
		}

		if r.packSettings.OnlyKeys {
			s.DisconnectServer()
		}
		messages.SendEvent(&messages.EventInitialPacksInfo{
			Packs:    pk.TexturePacks,
			KeysOnly: r.packSettings.OnlyKeys,
		})

	case *packet.ResourcePackChunkData:
		messages.SendEvent(&messages.EventPackDownloadProgress{
			ID:         pk.UUID,
			BytesAdded: len(pk.Data),
		})

	case *packet.ResourcePackClientResponse:
		if pk.Response == packet.PackResponseCompleted {
			go s.DisconnectServer()
			packs := s.Server.ResourcePacks()
			if len(packs) == 0 {
				logrus.Warn(locale.Loc("no_resourcepacks", nil))
			}
		}
	}
	return pk, nil
}

func (r *resourcePackHandler) Handler() *proxy.Handler {
	var wg sync.WaitGroup
	errs := make(chan error, 1)
	packChannel := make(chan resource.Pack, 10)

	return &proxy.Handler{
		Name: "Resourcepacks",
		SessionStart: func(s *proxy.Session, hostname string) error {
			// Default output dir is hostname; NhanAZ mode overrides with dated hostname.
			outputDir := utils.PathData("packs", hostname)
			if r.packSettings.NhanAZ {
				today := time.Now().Format("02-01-2006")
				outputDir = utils.PathData("packs", today+"-"+hostname)
			}
			os.MkdirAll(outputDir, 0o777)
			packNameCounts := make(map[string]int)

			wg.Add(1)
			go func() {
				defer close(errs)
				defer wg.Done()
				for pack := range packChannel {
					err := processPack(outputDir, packNameCounts, pack, r.packSettings)
					if err != nil {
						errs <- err
					}
				}
			}()

			return nil
		},

		FilterResourcePack: func(_ *proxy.Session, id string) bool {
			return r.packSettings.OnlyKeys
		},

		OnFinishedPack: func(_ *proxy.Session, pack resource.Pack) error {
			select {
			case err := <-errs:
				if err != nil {
					return err
				}
			default:
			}
			packChannel <- pack
			return nil
		},

		PacketCallback: r.onPacket,

		OnSessionEnd: func(s *proxy.Session, _wg *sync.WaitGroup) {
			close(packChannel)
			wg.Wait()
			logrus.Info("Done!")
		},
	}
}

func (ResourcePackCMD) Run(ctx context.Context, settings any) error {
	packSettings := settings.(*ResourcePacksSettings)

	// NhanAZ mode also enables extra diagnostics and preserves encrypted blobs.
	if packSettings.NhanAZ {
		packSettings.ProxySettings.ExtraDebug = true
	}

	var handler = resourcePackHandler{packSettings: packSettings}

	p, err := proxy.New(ctx, packSettings.ProxySettings)
	if err != nil {
		return err
	}

	p.AddHandler(handler.Handler)

	err = p.Run(ctx, false)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	commands.RegisterCommand(&ResourcePackCMD{})
}
