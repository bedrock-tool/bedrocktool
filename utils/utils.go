// Package utils ...
package utils

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/crypt"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"

	//"github.com/sandertv/gophertunnel/minecraft/gatherings"

	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

var Options struct {
	Debug              bool
	Preload            bool
	IsInteractive      bool
	ExtraDebug         bool
	EnableDNS          bool
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

func connectServer(ctx context.Context, address string, ClientData *login.ClientData, wantPacks bool, packetFunc PacketFunc) (serverConn *minecraft.Conn, err error) {
	cd := login.ClientData{}
	if ClientData != nil {
		cd = *ClientData
	}

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": address}))
	serverConn, err = minecraft.Dialer{
		TokenSource: GetTokenSource(),
		ClientData:  cd,
		PacketFunc: func(header packet.Header, payload []byte, src, dst net.Addr) {
			if Options.Debug {
				PacketLogger(header, payload, src, dst)
			}
			if packetFunc != nil {
				packetFunc(header, payload, src, dst)
			}
		},
		DownloadResourcePack: func(id uuid.UUID, version string, current int, total int) bool {
			return wantPacks
		},
	}.DialContext(ctx, "raknet", address)
	if err != nil {
		return serverConn, err
	}

	logrus.Debug(locale.Loc("connected", nil))
	ClientAddr = serverConn.LocalAddr()
	return serverConn, nil
}

func spawnConn(ctx context.Context, clientConn *minecraft.Conn, serverConn *minecraft.Conn, gd minecraft.GameData) error {
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

func InitExtraDebug(ctx context.Context) {
	if !Options.ExtraDebug {
		return
	}
	Options.Debug = true

	var logPlain, logCryptEnc io.WriteCloser = nil, nil

	// open plain text log
	logPlain, err := os.Create("packets.log")
	if err != nil {
		logrus.Error(err)
	}

	// open gpg log
	logCrypt, err := os.Create("packets.log.gpg")
	if err != nil {
		logrus.Error(err)
	} else {
		// encrypter for the log
		logCryptEnc, err = crypt.Encer("packets.log", logCrypt)
		if err != nil {
			logrus.Error(err)
		}
	}

	b := bufio.NewWriter(io.MultiWriter(logPlain, logCryptEnc))
	FLog = b
	if err != nil {
		logrus.Error(err)
	}
	go func() {
		<-ctx.Done()
		b.Flush()

		if logPlain != nil {
			logPlain.Close()
		}
		if logCrypt != nil {
			logCrypt.Close()
		}
		if logCryptEnc != nil {
			logCryptEnc.Close()
		}
	}()
}
