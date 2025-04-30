package utils

import (
	"path"

	"github.com/bedrock-tool/bedrocktool/utils/osabs"
)

func PathCache(p ...string) string {
	pj := path.Join(p...)
	if path.IsAbs(pj) {
		return pj
	}
	return path.Join(osabs.GetCacheDir(), pj)
}

func PathData(p ...string) string {
	pj := path.Join(p...)
	if path.IsAbs(pj) {
		return pj
	}
	return path.Join(osabs.GetDataDir(), pj)
}
