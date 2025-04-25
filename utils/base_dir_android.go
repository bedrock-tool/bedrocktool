package utils

import (
	"gioui.org/app"
)

func BaseDir() string {
	baseDir, err := app.DataDir()
	if err != nil {
		panic(err)
	}

	return baseDir
}
