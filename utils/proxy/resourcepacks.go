package proxy

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type exemptedResourcePack struct {
	uuid    string
	version string
}

type rpHandler struct {
	Server               minecraft.IConn
	Client               minecraft.IConn
	cache                packCache
	queue                *resourcePackQueue
	nextPack             chan *resource.Pack
	ignoredResourcePacks []exemptedResourcePack
	remotePacks          *packet.ResourcePacksInfo
	receivedRemotePacks  chan struct{}
	receivedRemoteStack  chan struct{}
	packMu               sync.Mutex
	resourcePacks        []*resource.Pack
	stack                *packet.ResourcePackStack
}

func NewRpHandler(server, client minecraft.IConn) *rpHandler {
	r := &rpHandler{
		Server: server,
		Client: client,
		queue: &resourcePackQueue{
			packsToDownload:  make(map[string]*resource.Pack),
			downloadingPacks: make(map[string]downloadingPack),
			awaitingPacks:    make(map[string]*downloadingPack),
		},
		receivedRemotePacks: make(chan struct{}),
		receivedRemoteStack: make(chan struct{}),
	}
	if r.Client != nil {
		r.nextPack = make(chan *resource.Pack)
	}
	return r
}

// from server
func (r *rpHandler) OnResourcePacksInfo(pk *packet.ResourcePacksInfo) error {
	// First create a new resource pack queue with the information in the packet so we can download them
	// properly later.
	totalPacks := len(pk.TexturePacks) + len(pk.BehaviourPacks)
	r.queue.serverPackAmount = totalPacks
	packsToDownload := make([]string, 0, totalPacks)

	for _, pack := range pk.TexturePacks {
		if _, ok := r.queue.downloadingPacks[pack.UUID]; ok {
			logrus.Warnf("duplicate texture pack entry %v in resource pack info\n", pack.UUID)
			r.queue.serverPackAmount--
			continue
		}
		if r.cache.Has(pack.UUID + "_" + pack.Version) {
			r.ignoredResourcePacks = append(r.ignoredResourcePacks, exemptedResourcePack{
				uuid:    pack.UUID,
				version: pack.Version,
			})
			r.resourcePacks = append(r.resourcePacks, r.cache.Get(pack.UUID+"_"+pack.Version).WithContentKey(pack.ContentKey))
			r.queue.serverPackAmount--
			continue
		}
		// This UUID_Version is a hack Mojang put in place.
		packsToDownload = append(packsToDownload, pack.UUID+"_"+pack.Version)
		r.queue.downloadingPacks[pack.UUID] = downloadingPack{
			size:       pack.Size,
			buf:        bytes.NewBuffer(make([]byte, 0, pack.Size)),
			newFrag:    make(chan []byte),
			contentKey: pack.ContentKey,
		}
	}
	for _, pack := range pk.BehaviourPacks {
		if _, ok := r.queue.downloadingPacks[pack.UUID]; ok {
			logrus.Warnf("duplicate behaviour pack entry %v in resource pack info\n", pack.UUID)
			r.queue.serverPackAmount--
			continue
		}
		if r.cache.Has(pack.UUID + "_" + pack.Version) {
			r.ignoredResourcePacks = append(r.ignoredResourcePacks, exemptedResourcePack{
				uuid:    pack.UUID,
				version: pack.Version,
			})
			r.resourcePacks = append(r.resourcePacks, r.cache.Get(pack.UUID+"_"+pack.Version).WithContentKey(pack.ContentKey))
			r.queue.serverPackAmount--
			continue
		}
		// This UUID_Version is a hack Mojang put in place.
		packsToDownload = append(packsToDownload, pack.UUID+"_"+pack.Version)
		r.queue.downloadingPacks[pack.UUID] = downloadingPack{
			size:       pack.Size,
			buf:        bytes.NewBuffer(make([]byte, 0, pack.Size)),
			newFrag:    make(chan []byte),
			contentKey: pack.ContentKey,
		}
	}

	r.remotePacks = pk
	close(r.receivedRemotePacks)

	if len(packsToDownload) != 0 {
		r.Server.Expect(packet.IDResourcePackDataInfo, packet.IDResourcePackChunkData)
		_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{
			Response:        packet.PackResponseSendPacks,
			PacksToDownload: packsToDownload,
		})
		return nil
	} else {
		if r.nextPack != nil {
			close(r.nextPack)
		}
	}
	r.Server.Expect(packet.IDResourcePackStack)
	_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseAllPacksDownloaded})
	return nil
}

// from server
func (r *rpHandler) OnResourcePackDataInfo(pk *packet.ResourcePackDataInfo) error {
	id := strings.Split(pk.UUID, "_")[0]

	pack, ok := r.queue.downloadingPacks[id]
	if !ok {
		// We either already downloaded the pack or we got sent an invalid UUID, that did not match any pack
		// sent in the ResourcePacksInfo packet.
		return fmt.Errorf("unknown pack to download with UUID %v", id)
	}
	if pack.size != pk.Size {
		// Size mismatch: The ResourcePacksInfo packet had a size for the pack that did not match with the
		// size sent here.
		logrus.Warnf("pack %v had a different size in the ResourcePacksInfo packet than the ResourcePackDataInfo packet\n", pk.UUID)
		pack.size = pk.Size
	}

	// Remove the resource pack from the downloading packs and add it to the awaiting packets.
	delete(r.queue.downloadingPacks, id)
	r.queue.awaitingPacks[id] = &pack

	pack.chunkSize = pk.DataChunkSize

	// The client calculates the chunk count by itself: You could in theory send a chunk count of 0 even
	// though there's data, and the client will still download normally.
	chunkCount := uint32(pk.Size / uint64(pk.DataChunkSize))
	if pk.Size%uint64(pk.DataChunkSize) != 0 {
		chunkCount++
	}

	idCopy := pk.UUID
	go func() {
		for i := uint32(0); i < chunkCount; i++ {
			_ = r.Server.WritePacket(&packet.ResourcePackChunkRequest{
				UUID:       idCopy,
				ChunkIndex: i,
			})
			select {
			case <-r.Server.OnDisconnect():
				return
			case frag := <-pack.newFrag:
				// Write the fragment to the full buffer of the downloading resource pack.

				lastData := pack.buf.Len()+int(pack.chunkSize) >= int(pack.size)
				if !lastData && uint32(len(frag)) != pack.chunkSize {
					// The chunk data didn't have the full size and wasn't the last data to be sent for the resource pack,
					// meaning we got too little data.
					logrus.Warnf("resource pack chunk data had a length of %v, but expected %v", len(frag), pack.chunkSize)
					return
				}

				_, _ = pack.buf.Write(frag)
			}
		}
		close(pack.newFrag)
		r.packMu.Lock()
		defer r.packMu.Unlock()

		if pack.buf.Len() != int(pack.size) {
			logrus.Warnf("incorrect resource pack size: expected %v, but got %v\n", pack.size, pack.buf.Len())
			return
		}
		// First parse the resource pack from the total byte buffer we obtained.
		newPack, err := resource.FromBytes(pack.buf.Bytes())
		newPack = newPack.WithContentKey(pack.contentKey)
		if err != nil {
			logrus.Warnf("invalid full resource pack data for UUID %v: %v\n", id, err)
			return
		}
		r.queue.serverPackAmount--
		// Finally we add the resource to the resource packs slice.
		r.resourcePacks = append(r.resourcePacks, newPack)
		err = r.cache.Put(newPack)
		if err != nil {
			logrus.Warnf("failed to cache for UUID %v: %v\n", id, err)
			return
		}
		if r.nextPack != nil {
			r.nextPack <- newPack
		}
		if r.queue.serverPackAmount == 0 {
			if r.nextPack != nil {
				close(r.nextPack)
			}
			r.Server.Expect(packet.IDResourcePackStack)
			_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseAllPacksDownloaded})
		}
	}()

	return nil
}

// from server
func (r *rpHandler) OnResourcePackChunkData(pk *packet.ResourcePackChunkData) error {
	pk.UUID = strings.Split(pk.UUID, "_")[0]
	pack, ok := r.queue.awaitingPacks[pk.UUID]
	if !ok {
		// We haven't received a ResourcePackDataInfo packet from the server, so we can't use this data to
		// download a resource pack.
		return fmt.Errorf("resource pack chunk data for resource pack that was not being downloaded")
	}

	if pk.ChunkIndex != pack.expectedIndex {
		return fmt.Errorf("resource pack chunk data had chunk index %v, but expected %v", pk.ChunkIndex, pack.expectedIndex)
	}
	pack.expectedIndex++
	pack.newFrag <- pk.Data
	return nil
}

// from server
func (r *rpHandler) OnResourcePackStack(pk *packet.ResourcePackStack) error {
	// We currently don't apply resource packs in any way, so instead we just check if all resource packs in
	// the stacks are also downloaded.
	for _, pack := range pk.TexturePacks {
		for i, behaviourPack := range pk.BehaviourPacks {
			if pack.UUID == behaviourPack.UUID {
				// We had a behaviour pack with the same UUID as the texture pack, so we drop the texture
				// pack and log it.
				logrus.Warnf("dropping behaviour pack with UUID %v due to a texture pack with the same UUID\n", pack.UUID)
				pk.BehaviourPacks = append(pk.BehaviourPacks[:i], pk.BehaviourPacks[i+1:]...)
			}
		}
		if !r.hasPack(pack.UUID, pack.Version, false) {
			return fmt.Errorf("texture pack {uuid=%v, version=%v} not downloaded", pack.UUID, pack.Version)
		}
	}
	for _, pack := range pk.BehaviourPacks {
		if !r.hasPack(pack.UUID, pack.Version, true) {
			return fmt.Errorf("behaviour pack {uuid=%v, version=%v} not downloaded", pack.UUID, pack.Version)
		}
	}

	r.stack = pk
	close(r.receivedRemoteStack)

	r.Server.Expect(packet.IDStartGame)
	_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseCompleted})
	return nil
}

// nextResourcePackDownload moves to the next resource pack to download and sends a resource pack data info
// packet with information about it.
func (r *rpHandler) nextResourcePackDownload() error {
	pack, ok := <-r.nextPack
	if !ok { // all remote packs received, send packs from cache
		pack, ok = r.queue.NextPack()
		if !ok {
			return fmt.Errorf("no resource packs to download")
		}
	}

	r.queue.currentPack = pack
	r.queue.currentOffset = 0
	checksum := pack.Checksum()

	var packType byte
	switch {
	case pack.HasWorldTemplate():
		packType = packet.ResourcePackTypeWorldTemplate
	case pack.HasTextures() && (pack.HasBehaviours() || pack.HasScripts()):
		packType = packet.ResourcePackTypeAddon
	case !pack.HasTextures() && (pack.HasBehaviours() || pack.HasScripts()):
		packType = packet.ResourcePackTypeBehaviour
	case pack.HasTextures():
		packType = packet.ResourcePackTypeResources
	default:
		packType = packet.ResourcePackTypeSkins
	}

	if err := r.Client.WritePacket(&packet.ResourcePackDataInfo{
		UUID:          pack.UUID(),
		DataChunkSize: packChunkSize,
		ChunkCount:    uint32(pack.DataChunkCount(packChunkSize)),
		Size:          uint64(pack.Len()),
		Hash:          checksum[:],
		PackType:      packType,
	}); err != nil {
		return fmt.Errorf("error sending resource pack data info packet: %v", err)
	}
	// Set the next expected packet to ResourcePackChunkRequest packets.
	r.Client.Expect(packet.IDResourcePackChunkRequest)
	return nil
}

// from client
func (r *rpHandler) OnResourcePackChunkRequest(pk *packet.ResourcePackChunkRequest) error {
	current := r.queue.currentPack
	if current.UUID() != pk.UUID {
		return fmt.Errorf("resource pack chunk request had unexpected UUID: expected %v, but got %v", current.UUID(), pk.UUID)
	}
	if r.queue.currentOffset != uint64(pk.ChunkIndex)*packChunkSize {
		return fmt.Errorf("resource pack chunk request had unexpected chunk index: expected %v, but got %v", r.queue.currentOffset/packChunkSize, pk.ChunkIndex)
	}
	response := &packet.ResourcePackChunkData{
		UUID:       pk.UUID,
		ChunkIndex: pk.ChunkIndex,
		DataOffset: r.queue.currentOffset,
		Data:       make([]byte, packChunkSize),
	}
	r.queue.currentOffset += packChunkSize
	// We read the data directly into the response's data.
	if n, err := current.ReadAt(response.Data, int64(response.DataOffset)); err != nil {
		// If we hit an EOF, we don't need to return an error, as we've simply reached the end of the content
		// AKA the last chunk.
		if err != io.EOF {
			return fmt.Errorf("error reading resource pack chunk: %v", err)
		}
		response.Data = response.Data[:n]

		defer func() {
			if !r.queue.AllDownloaded() {
				_ = r.nextResourcePackDownload()
			} else {
				r.Client.Expect(packet.IDResourcePackClientResponse)
			}
		}()
	}
	if err := r.Client.WritePacket(response); err != nil {
		return fmt.Errorf("error writing resource pack chunk data packet: %v", err)
	}

	return nil
}

func (r *rpHandler) Request(packs []string) error {
	<-r.receivedRemotePacks

	r.queue.packsToDownload = make(map[string]*resource.Pack)
	for _, packUUID := range packs {
		found := false
		if r.cache.Has(packUUID) {
			pack := r.cache.Get(packUUID)
			for _, pack2 := range r.remotePacks.TexturePacks {
				if pack2.UUID+"_"+pack2.Version == packUUID {
					if pack2.ContentKey != "" {
						pack = pack.WithContentKey(pack2.ContentKey)
						break
					}
				}
			}
			if pack.ContentKey() == "" {
				for _, pack2 := range r.remotePacks.BehaviourPacks {
					if pack2.UUID+"_"+pack2.Version == packUUID {
						if pack2.ContentKey != "" {
							pack = pack.WithContentKey(pack2.ContentKey)
							break
						}
					}
				}
			}

			r.queue.packsToDownload[pack.UUID()] = pack
			found = true
		} else {
			for _, pack := range r.remotePacks.TexturePacks {
				if pack.UUID+"_"+pack.Version == packUUID {
					found = true
					break
				}
			}
			for _, pack := range r.remotePacks.BehaviourPacks {
				if pack.UUID+"_"+pack.Version == packUUID {
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("could not find resource pack %v", packUUID)
		}
	}
	return nil
}

// from client
func (r *rpHandler) OnResourcePackClientResponse(pk *packet.ResourcePackClientResponse) error {
	switch pk.Response {
	case packet.PackResponseRefused:
		// Even though this response is never sent, we handle it appropriately in case it is changed to work
		// correctly again.
		return r.Client.Close()
	case packet.PackResponseSendPacks:
		if err := r.Request(pk.PacksToDownload); err != nil {
			return fmt.Errorf("error looking up resource packs to download: %v", err)
		}
		// Proceed with the first resource pack download. We run all downloads in sequence rather than in
		// parallel, as it's less prone to packet loss.
		if err := r.nextResourcePackDownload(); err != nil {
			return err
		}
	case packet.PackResponseAllPacksDownloaded:
		<-r.receivedRemoteStack
		if err := r.Client.WritePacket(r.stack); err != nil {
			return fmt.Errorf("error writing resource pack stack packet: %v", err)
		}
	case packet.PackResponseCompleted:
		r.Client.SetLoggedIn()
	default:
		return fmt.Errorf("unknown resource pack client response: %v", pk.Response)
	}
	return nil
}

func (r *rpHandler) GetResourcePacksInfo(texturePacksRequired bool) *packet.ResourcePacksInfo {
	<-r.receivedRemotePacks
	return r.remotePacks
}

func (r *rpHandler) ResourcePacks() []*resource.Pack {
	return r.resourcePacks
}

var exemptedPacks = []exemptedResourcePack{
	{
		uuid:    "0fba4063-dba1-4281-9b89-ff9390653530",
		version: "1.0.0",
	},
}

func (r *rpHandler) hasPack(uuid string, version string, hasBehaviours bool) bool {
	for _, exempted := range exemptedPacks {
		if exempted.uuid == uuid && exempted.version == version {
			// The server may send this resource pack on the stack without sending it in the info, as the client
			// always has it downloaded.
			return true
		}
	}
	r.packMu.Lock()
	defer r.packMu.Unlock()

	for _, ignored := range r.ignoredResourcePacks {
		if ignored.uuid == uuid && ignored.version == version {
			return true
		}
	}
	for _, pack := range r.resourcePacks {
		if pack.UUID() == uuid && pack.Version() == version && pack.HasBehaviours() == hasBehaviours {
			return true
		}
	}
	return false
}
