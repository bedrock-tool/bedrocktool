package encryptor

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math/rand"
	"path/filepath"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
)

type contentItem struct {
	Path string `json:"path"`
	Key  string `json:"key"`
}

type Content struct {
	Content []contentItem `json:"content"`
}

func GenerateKey() (out []byte) {
	out = make([]byte, 32)
	var vocab = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	for i := 0; i < 32; i++ {
		out[i] = vocab[rand.Intn(len(vocab))]
	}
	return
}

func encryptCfb(data []byte, key []byte) {
	b, _ := aes.NewCipher(key)
	s := cipher.NewCFBEncrypter(b, key[0:16])
	s.XORKeyStream(data, data)
}

func canEncrypt(path string) bool {
	if path == "manifest.json" {
		return false
	}
	s := filepath.SplitList(path)
	if s[0] == "texts" {
		return false
	}
	if s[len(s)-1] == "contents.json" {
		return false
	}
	return true
}

func enc(fsys fs.FS, fsyso fstest.MapFS, contentsJson *Content, dir string) error {
	// get all files in this folder
	matches, err := fs.Glob(fsys, dir+"**")
	if err != nil {
		return err
	}

	for _, path := range matches {
		// create output file
		ifo, err := fs.Stat(fsys, path)
		if err != nil {
			return err
		}
		fo := &fstest.MapFile{
			ModTime: ifo.ModTime(),
			Mode:    ifo.Mode(),
		}
		fsyso[path] = fo

		// recurse
		if ifo.IsDir() {
			return enc(fsys, fsyso, contentsJson, path+"/")
		}

		// read data
		var data []byte
		data, err = fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		// encrypt if needed
		if canEncrypt(path) {
			key := GenerateKey()
			it := contentItem{
				Path: path,
				Key:  hex.EncodeToString(key),
			}
			contentsJson.Content = append(contentsJson.Content, it)
			encryptCfb(data, key)
		}

		// write to output
		fo.Data = data
	}
	return nil
}

func Enc(fsys fs.FS, id *uuid.UUID, ContentKey []byte) (fs.FS, error) {
	var manifest map[string]any

	// read the manifest
	f, err := fsys.Open("manifest.json")
	if err == nil {
		dec := json.NewDecoder(f)
		err = dec.Decode(&manifest)
		if err != nil {
			return nil, err
		}
		header, ok := manifest["header"].(map[string]any)
		if !ok {
			return nil, errors.New("no header")
		}

		// get id from manifest if not specified, else change it in the manifet
		if id == nil {
			idstr, ok := header["uuid"].(string)
			if !ok {
				return nil, errors.New("no id")
			}
			_id, err := uuid.Parse(idstr)
			if err != nil {
				return nil, err
			}
			id = &_id
		} else {
			header["uuid"] = id.String()
		}
	} else {
		if id != nil {
			// create a manifest
		} else {
			return nil, err
		}
	}

	fsyso := fstest.MapFS{}
	// encrypt
	var contentsJson Content
	err = enc(fsys, fsyso, &contentsJson, "")
	if err != nil {
		return nil, err
	}

	// write new manifest
	manifestData, _ := json.MarshalIndent(manifest, "", "\t")
	fsyso["manifest.json"] = &fstest.MapFile{
		Data: manifestData,
	}

	// write the contents.json encrypted
	contentsBuf := bytes.NewBuffer(nil)
	binary.Write(contentsBuf, binary.LittleEndian, uint32(0))
	binary.Write(contentsBuf, binary.LittleEndian, uint32(0x9bcfb9fc))
	binary.Write(contentsBuf, binary.LittleEndian, uint64(0))
	contentsBuf.WriteByte(byte(len(id.String())))
	contentsBuf.Write([]byte(id.String()))
	contentsBuf.Write(make([]byte, 0xff-contentsBuf.Len()))
	contentsData, _ := json.Marshal(&contentsJson)
	encryptCfb(contentsData, ContentKey)
	contentsBuf.Write(contentsData)
	fsyso["contents.json"] = &fstest.MapFile{
		Data:    contentsBuf.Bytes(),
		Mode:    0775,
		ModTime: time.Now(),
	}

	return fsyso, nil
}

func fstozip(fsys fs.FS, zw *zip.Writer, dir string) error {
	// get files in this folder
	matches, err := fs.Glob(fsys, dir+"**")
	if err != nil {
		return err
	}
	for _, path := range matches {
		// if this path is a folder, recurse
		ifo, _ := fs.Stat(fsys, path)
		if ifo.IsDir() {
			return fstozip(fsys, zw, path+"/")
		}
		// copy the file to the zip
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     ifo.Name(),
			Modified: ifo.ModTime(),
		})
		if err != nil {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		w.Write(data)
	}
	return nil
}

func FSToZip(fsys fs.FS, w io.Writer) error {
	zw := zip.NewWriter(w)
	err := fstozip(fsys, zw, "")
	if err != nil {
		return err
	}
	zw.Close()
	return nil
}
