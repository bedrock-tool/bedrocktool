package android

import (
	"embed"
	"io/fs"

	"github.com/bedrock-tool/bedrocktool/locale"
)

// The Android strings live in locale/ here rather than in the shared
// locale files, so they never get in the way of the PC GUI's.
//
//go:embed locale/*.yaml
var localeFiles embed.FS

func init() {
	sub, err := fs.Sub(localeFiles, "locale")
	if err == nil {
		err = locale.AddMessages(sub)
	}
	if err != nil {
		panic(err) // embedded files: only fails while developing
	}
}

// T is locale.Loc for the strings in this package's locale/en.yaml.
func T(id string, tmpl locale.Strmap) string {
	return locale.Loc(id, tmpl)
}
