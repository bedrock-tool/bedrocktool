package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

const KEYS_FILE = "keys.db"

func init() {
	register_command("packs", "downloads resourcepacks from a server", pack_main)
}

// decrypt using cfb with segmentsize = 1
func cfb_decrypt(data []byte, key []byte) ([]byte, error) {
	cipher, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	shift_register := append(key[:16], data...)
	iv := make([]byte, 16)
	off := 0
	for ; off < len(data); off += 1 {
		cipher.Encrypt(iv, shift_register)
		data[off] ^= iv[0]
		shift_register = shift_register[1:]
	}
	return data, nil
}

type content_item struct {
	Path string `json:"path"`
	Key  string `json:"key"`
}

type Content struct {
	Content []content_item `json:"content"`
}

type Pack struct {
	resource.Pack
}

func (p *Pack) ReadAll() ([]byte, error) {
	buf := make([]byte, p.Len())
	off := 0
	for {
		n, err := p.ReadAt(buf[off:], int64(off))
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		off += n
	}
	return buf, nil
}

func (p *Pack) Decrypt() ([]byte, error) {
	data, err := p.ReadAll()
	if err != nil {
		return nil, err
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	zip_out_buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(zip_out_buf)

	written := make(map[string]interface{})

	// read key contents file
	var content Content = Content{}
	{
		ff, err := z.Open("contents.json")
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		} else {
			fw, _ := zw.Create("contents.json")
			buf, _ := io.ReadAll(ff)
			if err := json.Unmarshal(buf, &content); err != nil {
				dec, _ := cfb_decrypt(buf[0x100:], []byte(p.ContentKey()))
				dec = bytes.Split(dec, []byte("\x00"))[0]
				fw.Write(dec)
				if err := json.Unmarshal(dec, &content); err != nil {
					return nil, err
				}
			} else {
				fw.Write(buf)
			}
			written["contents.json"] = true
		}
	}

	// decrypt each file in the contents file
	for _, entry := range content.Content {
		ff, _ := z.Open(entry.Path)
		stat, _ := ff.Stat()
		if ff == nil || stat.IsDir() {
			continue
		}
		buf, _ := io.ReadAll(ff)
		if entry.Key != "" {
			buf, _ = cfb_decrypt(buf, []byte(entry.Key))
		}
		fw, _ := zw.Create(entry.Path)
		fw.Write(buf)
		written[entry.Path] = true
	}

	// copy files not in the contents file
	for _, src_file := range z.File {
		if written[src_file.Name] == nil {
			zw.Copy(src_file)
		}
	}

	zw.Close()
	return zip_out_buf.Bytes(), nil
}

func dump_keys(keys map[string]string) {
	f, err := os.OpenFile(KEYS_FILE, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for uuid, key := range keys {
		if key == "" {
			continue
		}
		f.WriteString(uuid + "=" + key + "\n")
	}
}

func pack_main(ctx context.Context, args []string) error {
	var server string
	var save_encrypted bool
	if len(args) >= 1 {
		server = args[0]
		args = args[1:]
	}

	flag.BoolVar(&save_encrypted, "save-encrypted", false, "save encrypted resourcepacks")
	flag.CommandLine.Parse(args)
	if G_help {
		flag.Usage()
		return nil
	}

	hostname, serverConn, err := connect_server(ctx, server)
	if err != nil {
		return err
	}
	serverConn.Close()
	println("Received")

	if len(serverConn.ResourcePacks()) > 0 {
		println("Decrypting Resource Packs")
		os.Mkdir(hostname, 0777)

		// dump keys, download and decrypt the packs
		keys := make(map[string]string)
		for _, pack := range serverConn.ResourcePacks() {
			pack := &Pack{*pack}
			keys[pack.UUID()] = pack.ContentKey()
			fmt.Printf("ResourcePack(Id: %s Key: %s | Name: %s %s)\n", pack.UUID(), keys[pack.UUID()], pack.Name(), pack.Version())

			if save_encrypted {
				data, err := pack.ReadAll()
				if err != nil {
					return err
				}
				os.WriteFile(path.Join(hostname, pack.Name()+".zip"), data, 0644)
			}
			fmt.Printf("Decrypting...\n")

			data, err := pack.Decrypt()
			if err != nil {
				return err
			}
			os.WriteFile(hostname+"/"+pack.Name()+".mcpack", data, 0644)
		}
		fmt.Printf("Writing keys to %s\n", KEYS_FILE)
		dump_keys(keys)
	} else {
		fmt.Printf("No Resourcepack sent\n")
	}
	fmt.Printf("Done!\n")
	return nil
}
