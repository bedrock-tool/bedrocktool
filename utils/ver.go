package utils

var Version string
var CmdName = "invalid"

const DisplayName = "C7 Proxy Client"

func IsDebug() bool {
	return Version == ""
}
