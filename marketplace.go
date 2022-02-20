package main

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"encoding/json"
	"io"
	"os"
)

// decrypt using cfb with segmentsize = 1
func cfb_decrypt(data []byte, key []byte) ([]byte, error) {
	b, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	shift_register := append(key[:16], data...) // prefill with iv + cipherdata
	_tmp := make([]byte, 16)
	off := 0
	for off < len(data) {
		b.Encrypt(_tmp, shift_register)
		data[off] ^= _tmp[0]
		shift_register = shift_register[1:]
		off++
	}
	return data, nil
}

type ContentEntry struct {
	Path string `json:"path"`
	Key  string `json:"key"`
}

type ContentJson struct {
	Content []ContentEntry `json:"content"`
}

func decrypt_pack(pack_zip []byte, filename, key string) error {
	// open reader and writers
	r := bytes.NewReader(pack_zip)
	z, err := zip.NewReader(r, r.Size())
	if err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	defer f.Close()
	defer zw.Close()

	written := make(map[string]interface{})

	// read content json file
	var content ContentJson
	{
		ff, err := z.Open("contents.json")
		if err != nil {
			if os.IsNotExist(err) {
				content = ContentJson{}
			} else {
				return err
			}
		} else {
			buf, _ := io.ReadAll(ff)
			dec, _ := cfb_decrypt(buf[0x100:], []byte(key))
			dec = bytes.Split(dec, []byte("\x00"))[0] // remove trailing \x00 (example: play.galaxite.net)
			fw, _ := zw.Create("contents.json")
			fw.Write(dec)
			if err := json.Unmarshal(dec, &content); err != nil {
				return err
			}
			written["contents.json"] = true
		}
	}

	// copy and decrypt all content
	for _, entry := range content.Content {
		ff, _ := z.Open(entry.Path)
		buf, _ := io.ReadAll(ff)
		if entry.Key != "" {
			buf, _ = cfb_decrypt(buf, []byte(entry.Key))
		}
		fw, _ := zw.Create(entry.Path)
		fw.Write(buf)
		written[entry.Path] = true
	}

	// copy everything not in the contents file
	for _, src_file := range z.File {
		if written[src_file.Name] == nil {
			zw.Copy(src_file)
		}
	}

	return nil
}
