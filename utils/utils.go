// Package utils ...
package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unsafe"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

var Options struct {
	Debug              bool
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

// SplitExt splits path to filename and extension
func SplitExt(filename string) (name, ext string) {
	name, ext = filepath.Base(filename), filepath.Ext(filename)
	name = strings.TrimSuffix(name, ext)
	return
}

func RandSeededUUID(str string) string {
	h := sha256.Sum256([]byte(str))
	id, _ := uuid.NewRandomFromReader(bytes.NewBuffer(h[:]))
	return id.String()
}

func WriteManifest(manifest *resource.Manifest, fpath string) error {
	w, err := os.Create(filepath.Join(fpath, "manifest.json"))
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
	cipher, _ := aes.NewCipher(key)

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

type cfb8 struct {
	r             io.Reader
	cipher        cipher.Block
	shiftRegister [16]byte
	iv            [16]byte
}

func NewCfb8(r io.Reader, key []byte) io.Reader {
	c := &cfb8{
		r: r,
	}
	c.cipher, _ = aes.NewCipher(key)
	copy(c.shiftRegister[:], key[:16])
	return c
}

//go:noescape
//go:linkname memmove runtime.memmove
func memmove(to, from unsafe.Pointer, n uintptr)

func (c *cfb8) Read(dst []byte) (n int, err error) {
	_ = c.shiftRegister[15]
	n, err = c.r.Read(dst)
	if n > 0 {
		_ = dst[n-1]
		for off := 0; off < n; off += 1 {
			c.cipher.Encrypt(c.iv[:], c.shiftRegister[:])

			// shift
			memmove(unsafe.Pointer(&c.shiftRegister[0]), unsafe.Pointer(&c.shiftRegister[1]), 15)
			c.shiftRegister[15] = dst[off]

			dst[off] ^= c.iv[0]
		}
	}
	return
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
		if err := cmd.Start(); err != nil {
			logrus.Error(err)
		}
		return
	}
	if runtime.GOOS == "linux" {
		println(path)
	}
}
