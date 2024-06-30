package utils

import (
	"os"
	"path"
)

func walkDirRemove(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return err
	}

	for _, name1 := range names {
		name2 := path.Join(name, name1)
		err := RemoveFile(name2)
		if err != nil {
			if err := walkDirRemove(name2); err != nil {
				return err
			}
			err = RemoveDir(name2)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func RemoveTree(dir string) error {
	return walkDirRemove(dir)
}
