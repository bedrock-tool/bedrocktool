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
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unsafe"

	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tailscale/hujson"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

var Options struct {
	Debug         bool
	IsInteractive bool
	ExtraDebug    bool
	Capture       bool
	Env           string
}

var LogOff bool

var nameRegexp = regexp.MustCompile(`\||(?:§.?)`)

var PackFromBase = func(pack resource.Pack) (resource.Pack, error) {
	return pack, nil
}

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

func WriteManifest(manifest *resource.Manifest, fs WriterFS, fpath string) error {
	w, err := fs.Create(path.Join(fpath, "manifest.json"))
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
		logrus.Println(path)
	}
}

func ParseJson(s []byte, out any) error {
	v, err := hujson.Parse(s)
	if err != nil {
		if !strings.Contains(err.Error(), "invalid character") {
			return err
		}
	}
	v.Standardize()
	s = v.Pack()

	d := json.NewDecoder(bytes.NewBuffer(s))
	err = d.Decode(out)
	if err != nil {
		return err
	}
	return nil
}

// stackToItem converts a network ItemStack representation back to an item.Stack.
func StackToItem(reg world.BlockRegistry, it protocol.ItemStack) item.Stack {
	t, ok := world.ItemByRuntimeID(it.NetworkID, int16(it.MetadataValue))
	if !ok {
		t = block.Air{}
	}
	if it.BlockRuntimeID > 0 {
		// It shouldn't matter if it (for whatever reason) wasn't able to get the block runtime ID,
		// since on the next line, we assert that the block is an item. If it didn't succeed, it'll
		// return air anyway.
		b, _ := reg.BlockByRuntimeID(uint32(it.BlockRuntimeID))
		if t, ok = b.(world.Item); !ok {
			t = block.Air{}
		}
	}
	//noinspection SpellCheckingInspection
	if nbter, ok := t.(world.NBTer); ok && len(it.NBTData) != 0 {
		t = nbter.DecodeNBT(it.NBTData).(world.Item)
	}
	s := item.NewStack(t, int(it.Count))
	return nbtconv.Item(it.NBTData, &s)
}
