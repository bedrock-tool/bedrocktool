package utils

import "path"

var CacheFolder string
var DataFolder string

func PathCache(p ...string) string {
	pj := path.Join(p...)
	if path.IsAbs(pj) {
		return pj
	}
	return path.Join(CacheFolder, pj)
}

func PathData(p ...string) string {
	pj := path.Join(p...)
	if path.IsAbs(pj) {
		return pj
	}
	return path.Join(DataFolder, pj)
}
