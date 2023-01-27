package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func UnpackZip(r io.ReaderAt, size int64, unpack_folder string) {
	zr, _ := zip.NewReader(r, size)
	for _, src_file := range zr.File {
		src_name := strings.ReplaceAll(src_file.Name, "\\", "/")
		out_path := path.Join(unpack_folder, src_name)
		if src_file.Mode().IsDir() {
			os.Mkdir(out_path, 0o755)
		} else {
			os.MkdirAll(path.Dir(out_path), 0o755)
			fr, _ := src_file.Open()
			f, _ := os.Create(path.Join(unpack_folder, src_name))
			io.Copy(f, fr)
		}
	}
}

func ZipFolder(filename, folder string) error {
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if !d.Type().IsDir() {
			rel := path[len(folder)+1:]
			zwf, _ := zw.Create(rel)
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Println(err)
			}
			zwf.Write(data)
		}
		return nil
	})
	zw.Close()
	f.Close()
	return err
}
