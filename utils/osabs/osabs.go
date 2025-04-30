package osabs

var cacheDir string
var dataDir string

func GetCacheDir() string {
	return cacheDir
}

func GetDataDir() string {
	return dataDir
}

func Init() error {
	return implInit()
}
