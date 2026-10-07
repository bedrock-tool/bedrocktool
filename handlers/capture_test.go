package handlers

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/proxy/pcap2"
	"github.com/klauspost/compress/s2"
	"github.com/sirupsen/logrus"
)

func TestRawCaptureExistsBeforeServerConnect(t *testing.T) {
	old, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	// An existing captures directory must also be tightened before recording.
	if err := os.Mkdir("captures", 0o755); err != nil {
		t.Fatal(err)
	}
	p := &packetCapturer{log: logrus.NewEntry(logrus.New())}
	if err := p.onServerName("test-realm"); err != nil {
		t.Fatal(err)
	}
	payload := []byte{58, 9, 1, 0, 1, 0xfe, 0xff}
	defer p.rawFile.Close()
	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{"captures": 0o700, p.rawFile.Name(): 0o600} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != want {
				t.Errorf("%s permissions = %o, want %o", path, info.Mode().Perm(), want)
			}
		}
	}
	p.dumpPacket(false, payload, time.Unix(1, 0))
	files, err := filepath.Glob("captures/*.raw.pcap2")
	if err != nil || len(files) != 1 {
		t.Fatalf("raw capture missing before join: %v %v", files, err)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := pcap2.NewPcap2Reader(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = r.ReadPacket(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	start := 16 + int(binary.LittleEndian.Uint64(data[8:16]))
	n := int(binary.LittleEndian.Uint32(data[start+4 : start+8]))
	decoded, err := s2.Decode(nil, data[start+17:start+17+n])
	if err != nil || !bytes.Equal(decoded, payload) {
		t.Fatalf("raw payload changed: %x %v", decoded, err)
	}
}
