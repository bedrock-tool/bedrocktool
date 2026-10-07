package worldstate

import (
	"bytes"
	"fmt"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/df-mc/goleveldb/leveldb/util"
	"net"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Go issue 77975: WSARecvFrom kept an overlapped output pointer on a
// movable goroutine stack. The corruption can surface in unrelated DB code.
func TestWindowsStorageToolchain(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows overlapped I/O regression")
	}
	v := strings.TrimPrefix(runtime.Version(), "go")
	var major, minor, patch int
	if _, err := fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch); err != nil {
		t.Fatalf("unverified Go toolchain %q", runtime.Version())
	}
	if major < 1 || (major == 1 && (minor < 26 || (minor == 26 && patch < 2))) {
		t.Fatalf("unsafe Windows UDP toolchain %s: require Go >=1.26.2 (golang/go#77975)", runtime.Version())
	}
}

// Uses only temporary synthetic test DBs and loopback UDP; never capture/world
// paths. Exercise background compaction, GC and pending overlapped reads together.
func TestStorageCompactionWithPendingUDP(t *testing.T) {
	old := debug.SetGCPercent(20)
	defer debug.SetGCPercent(old)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 2048)
		for {
			if _, _, err := conn.ReadFromUDP(buf); err != nil {
				return
			}
		}
	}()
	defer func() { conn.Close(); <-done }()
	sender, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	stop := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				sender.Write([]byte("synthetic stress"))
			}
		}
	}()
	defer func() { close(stop); <-sent }()
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		dir := t.TempDir()
		wg.Add(1)
		go func() {
			defer wg.Done()
			options := &opt.Options{WriteBuffer: 64 * 1024, CompactionTableSize: 64 * 1024, Compression: opt.FlateCompression}
			db, err := leveldb.OpenFile(dir, options)
			if err != nil {
				t.Error(err)
				return
			}
			for i := 0; i < 4096; i++ {
				value := bytes.Repeat([]byte(strconv.Itoa(i)+"synthetic-only"), 100)
				if err := db.Put([]byte(fmt.Sprintf("key-%05d", i)), value, nil); err != nil {
					t.Error(err)
					db.Close()
					return
				}
				if i%256 == 0 {
					runtime.GC()
				}
			}
			if err := db.CompactRange(util.Range{}); err != nil {
				t.Error(err)
			}
			if err := db.Close(); err != nil {
				t.Error(err)
			}
			db, err = leveldb.OpenFile(dir, options)
			if err != nil {
				t.Error(err)
				return
			}
			defer db.Close()
			for i := 0; i < 4096; i++ {
				got, err := db.Get([]byte(fmt.Sprintf("key-%05d", i)), nil)
				want := bytes.Repeat([]byte(strconv.Itoa(i)+"synthetic-only"), 100)
				if err != nil || !bytes.Equal(got, want) {
					t.Errorf("round-trip %d: error=%v equal=%v", i, err, bytes.Equal(got, want))
					return
				}
			}
		}()
	}
	wg.Wait()
}
