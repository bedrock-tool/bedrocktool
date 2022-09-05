package utils

import "github.com/sanbornm/go-selfupdate/selfupdate"

var Version string

const updateServer = "https://updates.yuv.pink/"

var Updater = &selfupdate.Updater{
	CurrentVersion: Version,
	ApiURL:         updateServer,
	BinURL:         updateServer,
	Dir:            "update/",
	CmdName:        "bedrocktool", // app name
}
