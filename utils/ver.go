package utils

var Version string
var CmdName = "invalid"

func IsDebug() bool {
	return Version == ""
}
