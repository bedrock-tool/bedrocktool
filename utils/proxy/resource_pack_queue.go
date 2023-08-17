package proxy

import (
	"bytes"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

// packChunkSize is the size of a single chunk of data from a resource pack: 512 kB or 0.5 MB
const packChunkSize = 1024 * 128

// resourcePackQueue is used to aid in the handling of resource pack queueing and downloading. Only one
// resource pack is downloaded at a time.
type resourcePackQueue struct {
	currentPack   *resource.Pack
	currentOffset uint64

	serverPackAmount int
	downloadingPacks map[string]downloadingPack
	awaitingPacks    map[string]*downloadingPack
	NextPack         chan *resource.Pack
}

// downloadingPack is a resource pack that is being downloaded by a client connection.
type downloadingPack struct {
	buf           *bytes.Buffer
	chunkSize     uint32
	size          uint64
	expectedIndex uint32
	newFrag       chan []byte
	contentKey    string
}
