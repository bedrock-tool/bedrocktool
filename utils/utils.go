// Package utils ...
package utils

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	//"github.com/sandertv/gophertunnel/minecraft/gatherings"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

var Options struct {
	Debug              bool
	Preload            bool
	IsInteractive      bool
	ExtraDebug         bool
	Capture            bool
	PathCustomUserData string
}

var nameRegexp = regexp.MustCompile(`\||(?:§.?)`)

// CleanupName cleans name so it can be used as a filename
func CleanupName(name string) string {
	name = strings.Split(name, "\n")[0]
	var _tmp struct {
		K string `json:"k"`
	}
	err := json.Unmarshal([]byte(name), &_tmp)
	if err == nil {
		name = _tmp.K
	}
	name = string(nameRegexp.ReplaceAll([]byte(name), []byte("")))
	name = strings.TrimSpace(name)
	return name
}

// connections

func connectServer(ctx context.Context, address string, ClientData *login.ClientData, wantPacks bool, packetFunc PacketFunc, tokenSource oauth2.TokenSource) (serverConn *minecraft.Conn, err error) {
	cd := login.ClientData{}
	if ClientData != nil {
		cd = *ClientData
	}

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": address}))
	serverConn, err = minecraft.Dialer{
		TokenSource: tokenSource,
		ClientData:  cd,
		PacketFunc:  packetFunc,
		DownloadResourcePack: func(id uuid.UUID, version string, current int, total int) bool {
			return wantPacks
		},
	}.DialContext(ctx, "raknet", address)
	if err != nil {
		return serverConn, err
	}

	logrus.Debug(locale.Loc("connected", nil))
	return serverConn, nil
}

func spawnConn(ctx context.Context, clientConn minecraft.IConn, serverConn minecraft.IConn, gd minecraft.GameData) error {
	wg := sync.WaitGroup{}
	errs := make(chan error, 2)
	if clientConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- clientConn.StartGame(gd)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- serverConn.DoSpawn()
	}()

	wg.Wait()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				return errors.New(locale.Loc("failed_start_game", locale.Strmap{"Err": err}))
			}
		case <-ctx.Done():
			return errors.New(locale.Loc("connection_cancelled", nil))
		default:
		}
	}
	return nil
}

// SplitExt splits path to filename and extension
func SplitExt(filename string) (name, ext string) {
	name, ext = path.Base(filename), path.Ext(filename)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return
}

func Clamp(a, b int) int {
	if a > b {
		return b
	}
	if a < 0 {
		return 0
	}
	return a
}

func RandSeededUUID(str string) string {
	h := sha256.Sum256([]byte(str))
	id, _ := uuid.NewRandomFromReader(bytes.NewBuffer(h[:]))
	return id.String()
}

func WriteManifest(manifest *resource.Manifest, fpath string) error {
	w, err := os.Create(path.Join(fpath, "manifest.json"))
	if err != nil {
		return err
	}
	defer w.Close()
	e := json.NewEncoder(w)
	e.SetIndent("", "\t")
	if err = e.Encode(manifest); err != nil {
		return err
	}
	return nil
}

func CfbDecrypt(data []byte, key []byte) []byte {
	cipher, _ := aes.NewCipher([]byte(key))

	shiftRegister := append(key[:16], data...)
	iv := make([]byte, 16)
	off := 0
	for ; off < len(data); off += 1 {
		cipher.Encrypt(iv, shiftRegister)
		data[off] ^= iv[0]
		shiftRegister = shiftRegister[1:]
	}
	return data
}

func abs(n float32) float32 {
	if n < 0 {
		n = -n
	}
	return n
}

func SizeofFmt(num float32) string {
	for _, unit := range []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi"} {
		if abs(num) < 1024.0 {
			return fmt.Sprintf("%3.1f%sB", num, unit)
		}
		num /= 1024.0
	}
	return fmt.Sprintf("%.1fYiB", num)
}

func ShowFile(path string) {
	path, _ = filepath.Abs(path)
	if runtime.GOOS == "windows" {
		cmd := exec.Command(`explorer`, "/select,", path)
		cmd.Start()
		return
	}
	if runtime.GOOS == "linux" {
		println(path)
	}
}
