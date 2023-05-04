package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/sanbornm/go-selfupdate/selfupdate"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var Version string
var CmdName = "bedrocktool"

var UpdateAvailable string

const updateServer = "https://updates.yuv.pink/"

type trequester struct {
	selfupdate.Requester
}

func (httpRequester *trequester) Fetch(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// set user agent to know what versions are run
	h, _ := os.Hostname()
	v, _ := mem.VirtualMemory()
	c, _ := cpu.Info()
	var ct string
	if len(c) > 0 {
		ct = c[0].ModelName
	}
	req.Header.Add("User-Agent", fmt.Sprintf("%s '%s' '%s' %d %d '%s'", CmdName, Version, h, runtime.NumCPU(), v.Total, ct))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad http status from %s: %v", url, resp.Status)
	}

	return resp.Body, nil
}

var Updater = &selfupdate.Updater{
	CurrentVersion: Version,
	ApiURL:         updateServer,
	BinURL:         updateServer,
	Dir:            "update/",
	CmdName:        CmdName,
	Requester:      &trequester{},
}
