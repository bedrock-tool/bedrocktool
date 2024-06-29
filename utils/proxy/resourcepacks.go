package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
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

type exemptedResourcePack struct {
	uuid    string
	version string
}

type rpHandler struct {
	Server minecraft.IConn
	Client minecraft.IConn
	ctx    context.Context
	log    *logrus.Entry

	// wait for downloads to be done
	dlwg sync.WaitGroup

	// gives access to stored resource packs in an abstract way so it can be replaced for replay
	cache iPackCache

	// queue for packs the client can receive

	nextPackToClient chan *resource.Pack
	packsFromCache   []*resource.Pack
	addedPacks       []*resource.Pack
	addedPacksDone   int

	downloadQueue *resourcePackQueue
	packDownloads chan *packet.ResourcePackDataInfo

	// closed when decided what packs to download from the server
	knowPacksRequestedFromServer chan struct{}
	packsRequestedFromServer     []string

	// set to true if the client wants any resource packs
	// if its false when the client sends the `done` message, that means the nextPack channel should be closed
	clientHasRequested bool

	// list of packs to not download, this is based on whats in the cache
	ignoredResourcePacks []exemptedResourcePack

	// closed when the proxy has received resource pack info from the server
	receivedRemotePackInfo chan struct{}
	remotePacksInfo        *packet.ResourcePacksInfo

	// closed when the proxy has received the resource pack stack from the server
	receivedRemoteStack chan struct{}
	remoteStack         *packet.ResourcePackStack

	// used when adding a resourcepack to the list after its downloaded
	packMu sync.Mutex

	// all active resource packs for access by the proxy
	resourcePacks []*resource.Pack

	// optional callback when its known what resource packs the server has
	OnResourcePacksInfoCB func()

	// optional callback that is called as soon as a resource pack is added to the proxies list
	OnFinishedPack func(*resource.Pack)
}

func newRpHandler(ctx context.Context, addedPacks []*resource.Pack) *rpHandler {
	r := &rpHandler{
		ctx:        ctx,
		log:        logrus.WithField("part", "ResourcePacks"),
		addedPacks: addedPacks,
		downloadQueue: &resourcePackQueue{
			downloadingPacks: make(map[string]downloadingPack),
			awaitingPacks:    make(map[string]*downloadingPack),
			NextPack:         make(chan *resource.Pack),
		},
		cache: &packCache{
			commit: make(chan struct{}),
		},
		receivedRemotePackInfo: make(chan struct{}),
		receivedRemoteStack:    make(chan struct{}),
	}
	return r
}

func (r *rpHandler) SetServer(c minecraft.IConn) {
	r.Server = c
}

func (r *rpHandler) SetClient(c minecraft.IConn) {
	r.Client = c
	r.nextPackToClient = make(chan *resource.Pack)
	r.knowPacksRequestedFromServer = make(chan struct{})
}

// from server
func (r *rpHandler) OnResourcePacksInfo(pk *packet.ResourcePacksInfo) error {
	if r.OnResourcePacksInfoCB != nil {
		r.OnResourcePacksInfoCB()
	}
	// First create a new resource pack queue with the information in the packet so we can download them
	// properly later.
	totalPacks := len(pk.TexturePacks) + len(pk.BehaviourPacks)
	r.downloadQueue.serverPackAmount = totalPacks
	packsToDownload := make([]string, 0, totalPacks)

	for _, pack := range pk.TexturePacks {
		_, alreadyDownloading := r.downloadQueue.downloadingPacks[pack.UUID]
		alreadyIgnored := slices.ContainsFunc(r.ignoredResourcePacks, func(e exemptedResourcePack) bool { return e.uuid == pack.UUID })
		if alreadyDownloading || alreadyIgnored {
			r.log.Warnf("duplicate texture pack entry %v in resource pack info\n", pack.UUID)
			r.downloadQueue.serverPackAmount--
			continue
		}
		if r.cache.Has(pack.UUID, pack.Version) {
			r.ignoredResourcePacks = append(r.ignoredResourcePacks, exemptedResourcePack{
				uuid:    pack.UUID,
				version: pack.Version,
			})
			newPack := r.cache.Get(pack.UUID, pack.Version).WithContentKey(pack.ContentKey)
			r.resourcePacks = append(r.resourcePacks, newPack)
			r.OnFinishedPack(newPack)
			r.downloadQueue.serverPackAmount--
			continue
		}

		idxURL := slices.IndexFunc(pk.PackURLs, func(pu protocol.PackURL) bool {
			return pack.UUID+"_"+pack.Version == pu.UUIDVersion
		})
		if idxURL != -1 {
			url := pk.PackURLs[idxURL]
			r.ignoredResourcePacks = append(r.ignoredResourcePacks, exemptedResourcePack{
				uuid:    pack.UUID,
				version: pack.Version,
			})
			r.dlwg.Add(1)
			contentKey := pack.ContentKey
			go func() {
				defer r.dlwg.Done()
				r.log.Infof("Downloading Resourcepack: %s", url.URL)
				newPack, err := resource.ReadURL(url.URL)
				if err != nil {
					r.log.Error(err)
					return
				}
				newPack = newPack.WithContentKey(contentKey)
				r.resourcePacks = append(r.resourcePacks, newPack)
				r.OnFinishedPack(newPack)

				if r.Client != nil {
					select {
					case <-r.knowPacksRequestedFromServer:
					case <-r.ctx.Done():
						return
					}
					if slices.Contains(r.packsRequestedFromServer, newPack.UUID()) {
						r.nextPackToClient <- newPack
					}
				}
			}()
			r.downloadQueue.serverPackAmount--
			continue
		}

		// This UUID_Version is a hack Mojang put in place.
		packsToDownload = append(packsToDownload, pack.UUID+"_"+pack.Version)
		r.downloadQueue.downloadingPacks[pack.UUID] = downloadingPack{
			size:       pack.Size,
			buf:        bytes.NewBuffer(make([]byte, 0, pack.Size)),
			newFrag:    make(chan []byte),
			contentKey: pack.ContentKey,
		}
	}
	for _, pack := range pk.BehaviourPacks {
		if _, ok := r.downloadQueue.downloadingPacks[pack.UUID]; ok {
			r.log.Warnf("duplicate behaviour pack entry %v in resource pack info\n", pack.UUID)
			r.downloadQueue.serverPackAmount--
			continue
		}
		if r.cache.Has(pack.UUID, pack.Version) {
			r.ignoredResourcePacks = append(r.ignoredResourcePacks, exemptedResourcePack{
				uuid:    pack.UUID,
				version: pack.Version,
			})
			newPack := r.cache.Get(pack.UUID, pack.Version).WithContentKey(pack.ContentKey)
			r.resourcePacks = append(r.resourcePacks, newPack)
			r.OnFinishedPack(newPack)
			r.downloadQueue.serverPackAmount--
			continue
		}
		// This UUID_Version is a hack Mojang put in place.
		packsToDownload = append(packsToDownload, pack.UUID+"_"+pack.Version)
		r.downloadQueue.downloadingPacks[pack.UUID] = downloadingPack{
			size:       pack.Size,
			buf:        bytes.NewBuffer(make([]byte, 0, pack.Size)),
			newFrag:    make(chan []byte),
			contentKey: pack.ContentKey,
		}
	}

	r.remotePacksInfo = pk
	close(r.receivedRemotePackInfo)
	r.log.Debug("received remote pack infos")
	if r.Client != nil {
		select {
		case <-r.knowPacksRequestedFromServer:
		case <-r.ctx.Done():
			return r.ctx.Err()
		}
	}

	if len(packsToDownload) != 0 {
		// start downloading from server whenever a pack info is sent
		r.packDownloads = make(chan *packet.ResourcePackDataInfo, len(packsToDownload))
		go func() {
			for pk := range r.packDownloads {
				err := r.downloadResourcePack(pk)
				if err != nil {
					r.log.Error(err)
				}
			}
		}()

		r.Server.Expect(packet.IDResourcePackDataInfo, packet.IDResourcePackChunkData)
		_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{
			Response:        packet.PackResponseSendPacks,
			PacksToDownload: packsToDownload,
		})
		return nil
	}

	r.Server.Expect(packet.IDResourcePackStack)
	_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseAllPacksDownloaded})
	return nil
}

func (r *rpHandler) downloadResourcePack(pk *packet.ResourcePackDataInfo) error {
	id := strings.Split(pk.UUID, "_")[0]

	pack, ok := r.downloadQueue.downloadingPacks[id]
	if !ok {
		// We either already downloaded the pack or we got sent an invalid UUID, that did not match any pack
		// sent in the ResourcePacksInfo packet.
		if _, ok := r.cache.(*replayCache); ok {
			return nil
		}
		return fmt.Errorf("unknown pack to download with UUID %v", id)
	}
	if pack.size != pk.Size {
		// Size mismatch: The ResourcePacksInfo packet had a size for the pack that did not match with the
		// size sent here.
		r.log.Infof("pack %v had a different size in the ResourcePacksInfo packet than the ResourcePackDataInfo packet\n", pk.UUID)
		pack.size = pk.Size
	}

	// Remove the resource pack from the downloading packs and add it to the awaiting packets.
	delete(r.downloadQueue.downloadingPacks, id)
	r.downloadQueue.awaitingPacks[id] = &pack
	pack.chunkSize = pk.DataChunkSize

	// The client calculates the chunk count by itself: You could in theory send a chunk count of 0 even
	// though there's data, and the client will still download normally.
	chunkCount := uint32(pk.Size / uint64(pk.DataChunkSize))
	if pk.Size%uint64(pk.DataChunkSize) != 0 {
		chunkCount++
	}

	idCopy := pk.UUID

	for i := uint32(0); i < chunkCount; i++ {
		_ = r.Server.WritePacket(&packet.ResourcePackChunkRequest{
			UUID:       idCopy,
			ChunkIndex: i,
		})
		select {
		case <-r.Server.OnDisconnect():
			return net.ErrClosed
		case frag := <-pack.newFrag:
			// Write the fragment to the full buffer of the downloading resource pack.

			lastData := pack.buf.Len()+int(pack.chunkSize) >= int(pack.size)
			if !lastData && uint32(len(frag)) != pack.chunkSize {
				// The chunk data didn't have the full size and wasn't the last data to be sent for the resource pack,
				// meaning we got too little data.
				return fmt.Errorf("resource pack chunk data had a length of %v, but expected %v", len(frag), pack.chunkSize)
			}

			_, _ = pack.buf.Write(frag)
		}
	}
	close(pack.newFrag)
	r.packMu.Lock()
	defer r.packMu.Unlock()

	if pack.buf.Len() != int(pack.size) {
		return fmt.Errorf("incorrect resource pack size: expected %v, but got %v", pack.size, pack.buf.Len())
	}
	// First parse the resource pack from the total byte buffer we obtained.
	newPack, err := resource.Read(pack.buf)
	newPack = newPack.WithContentKey(pack.contentKey)
	if err != nil {
		return fmt.Errorf("invalid full resource pack data for UUID %v: %v", id, err)
	}
	r.downloadQueue.serverPackAmount--
	// Finally we add the resource to the resource packs slice.
	r.resourcePacks = append(r.resourcePacks, newPack)
	r.OnFinishedPack(newPack)
	r.cache.Put(newPack)

	// if theres a client and the client needs resource packs send it to its queue
	if r.nextPackToClient != nil && slices.Contains(r.packsRequestedFromServer, id) {
		r.log.Debugf("sending pack %s from server to client", id)
		r.nextPackToClient <- newPack
	}
	if r.downloadQueue.serverPackAmount == 0 {
		if r.nextPackToClient != nil {
			close(r.nextPackToClient)
		}
		if r.packDownloads != nil {
			close(r.packDownloads)
		}
		r.Server.Expect(packet.IDResourcePackStack)
		_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseAllPacksDownloaded})
	}

	return nil
}

// from server
func (r *rpHandler) OnResourcePackDataInfo(pk *packet.ResourcePackDataInfo) error {
	r.packDownloads <- pk
	return nil
}

// from server
func (r *rpHandler) OnResourcePackChunkData(pk *packet.ResourcePackChunkData) error {
	pk.UUID = strings.Split(pk.UUID, "_")[0]
	pack, ok := r.downloadQueue.awaitingPacks[pk.UUID]
	if !ok {
		if _, ok := r.cache.(*replayCache); ok {
			return nil
		}
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
				r.log.Warnf("dropping behaviour pack with UUID %v due to a texture pack with the same UUID\n", pack.UUID)
				pk.BehaviourPacks = append(pk.BehaviourPacks[:i], pk.BehaviourPacks[i+1:]...)
			}
		}
		if !r.hasPack(pack.UUID, pack.Version, false) {
			m := fmt.Errorf("texture pack {uuid=%v, version=%v} not downloaded", pack.UUID, pack.Version)
			if _, ok := r.cache.(*replayCache); ok {
				r.log.Warn(m)
			} else {
				return m
			}
		}
	}
	for _, pack := range pk.BehaviourPacks {
		if !r.hasPack(pack.UUID, pack.Version, true) {
			return fmt.Errorf("behaviour pack {uuid=%v, version=%v} not downloaded", pack.UUID, pack.Version)
		}
	}

	// r.addedPacks to the stack
	var addPacks []protocol.StackResourcePack
	for _, p := range r.addedPacks {
		addPacks = append(addPacks, protocol.StackResourcePack{
			UUID:        p.UUID(),
			Version:     p.Version(),
			SubPackName: p.Name(),
		})
	}
	pk.TexturePacks = append(addPacks, pk.TexturePacks...)

	r.remoteStack = pk
	close(r.receivedRemoteStack)
	r.log.Debug("received remote resourcepack stack, starting game")

	r.Server.Expect(packet.IDStartGame)
	_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseCompleted})
	if r.Client == nil {
		r.cache.Close()
	}
	return nil
}

// nextResourcePackDownload moves to the next resource pack to download and sends a resource pack data info
// packet with information about it.
func (r *rpHandler) nextResourcePackDownload() (ok bool, err error) {
	var pack *resource.Pack

	// select from one of the 3 sources in order
	// 1. addedPacks
	// 2. serverPacks
	// 3. cachedPacks
	if r.addedPacksDone < len(r.addedPacks) {
		pack = r.addedPacks[r.addedPacksDone]
		r.addedPacksDone++
	} else {
		select {
		case pack, ok = <-r.nextPackToClient:
		case <-r.ctx.Done():
			return false, r.ctx.Err()
		}

		if !ok {
			r.log.Info("finished sending client resource packs")
			return false, nil
		}
	}

	r.log.Debugf("next pack %s", pack.Name())

	r.downloadQueue.currentPack = pack
	r.downloadQueue.currentOffset = 0
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

	_ = r.Client.WritePacket(&packet.ResourcePackDataInfo{
		UUID:          pack.UUID(),
		DataChunkSize: packChunkSize,
		ChunkCount:    uint32(pack.DataChunkCount(packChunkSize)),
		Size:          uint64(pack.Len()),
		Hash:          checksum[:],
		PackType:      packType,
	})
	// Set the next expected packet to ResourcePackChunkRequest packets.
	r.Client.Expect(packet.IDResourcePackChunkRequest)
	return true, nil
}

// from client
func (r *rpHandler) OnResourcePackChunkRequest(pk *packet.ResourcePackChunkRequest) error {
	current := r.downloadQueue.currentPack
	if current.UUID() != pk.UUID {
		return fmt.Errorf("resource pack chunk request had unexpected UUID: expected %v, but got %v", current.UUID(), pk.UUID)
	}
	if r.downloadQueue.currentOffset != uint64(pk.ChunkIndex)*packChunkSize {
		return fmt.Errorf("resource pack chunk request had unexpected chunk index: expected %v, but got %v", r.downloadQueue.currentOffset/packChunkSize, pk.ChunkIndex)
	}
	response := &packet.ResourcePackChunkData{
		UUID:       pk.UUID,
		ChunkIndex: pk.ChunkIndex,
		DataOffset: r.downloadQueue.currentOffset,
		Data:       make([]byte, packChunkSize),
	}
	r.downloadQueue.currentOffset += packChunkSize
	// We read the data directly into the response's data.
	if n, err := current.ReadAt(response.Data, int64(response.DataOffset)); err != nil {
		// If we hit an EOF, we don't need to return an error, as we've simply reached the end of the content
		// AKA the last chunk.
		if err != io.EOF {
			return fmt.Errorf("error reading resource pack chunk: %v", err)
		}
		response.Data = response.Data[:n]

		defer func() {
			ok, err := r.nextResourcePackDownload()
			if err != nil {
				r.log.Error(err)
			}
			if !ok {
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
	r.clientHasRequested = true
	<-r.receivedRemotePackInfo

	r.nextPackToClient = make(chan *resource.Pack, len(packs))

	for _, packUUID := range packs {
		uuid_ver := strings.Split(packUUID, "_")

		found := false
		if r.cache.Has(uuid_ver[0], uuid_ver[1]) {
			r.log.Debug("using", packUUID, "from cache")

			pack := r.cache.Get(uuid_ver[0], uuid_ver[1])

			// add key
			for _, pack2 := range r.remotePacksInfo.TexturePacks {
				if pack2.UUID+"_"+pack2.Version == packUUID {
					if pack2.ContentKey != "" {
						pack = pack.WithContentKey(pack2.ContentKey)
						break
					}
				}
			}
			if pack.ContentKey() == "" {
				for _, pack2 := range r.remotePacksInfo.BehaviourPacks {
					if pack2.UUID+"_"+pack2.Version == packUUID {
						if pack2.ContentKey != "" {
							pack = pack.WithContentKey(pack2.ContentKey)
							break
						}
					}
				}
			}

			r.nextPackToClient <- pack
			r.packsFromCache = append(r.packsFromCache, pack)
			found = true
		}

		if !found {
			for _, pack := range r.addedPacks {
				if pack.UUID()+"_"+pack.Version() == packUUID {
					found = true
					// not sent to channel, is forced as first
					break
				}
			}
		}

		found = found || slices.ContainsFunc(r.remotePacksInfo.PackURLs, func(pu protocol.PackURL) bool {
			return packUUID == pu.UUIDVersion
		})

		if !found {
			for _, pack := range r.remotePacksInfo.TexturePacks {
				if pack.UUID+"_"+pack.Version == packUUID {
					found = true
					r.packsRequestedFromServer = append(r.packsRequestedFromServer, strings.Split(packUUID, "_")[0])
					break
				}
			}
		}

		if !found {
			for _, pack := range r.remotePacksInfo.BehaviourPacks {
				if pack.UUID+"_"+pack.Version == packUUID {
					found = true
					r.packsRequestedFromServer = append(r.packsRequestedFromServer, strings.Split(packUUID, "_")[0])
					break
				}
			}
		}

		if !found {
			return fmt.Errorf("could not find resource pack %v", packUUID)
		}
	}

	if len(r.packsFromCache)+len(r.packsRequestedFromServer)+len(r.addedPacks) < len(packs) {
		r.log.Errorf("BUG: not enough packs sent to client, client will stall %d + %d  %d", len(r.packsFromCache), len(r.packsRequestedFromServer), len(packs))
	}

	close(r.knowPacksRequestedFromServer)
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
		_, err := r.nextResourcePackDownload()
		if err != nil {
			return err
		}
	case packet.PackResponseAllPacksDownloaded:
		if !r.clientHasRequested {
			close(r.knowPacksRequestedFromServer)
		}

		r.log.Debug("waiting for remote stack")
		select {
		case <-r.receivedRemoteStack:
		case <-r.ctx.Done():
			return r.ctx.Err()
		}

		r.cache.Close()
		if err := r.Client.WritePacket(r.remoteStack); err != nil {
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
	select {
	case <-r.receivedRemotePackInfo:
	case <-r.ctx.Done():
	}

	var pk packet.ResourcePacksInfo
	if r.remotePacksInfo != nil {
		pk = *r.remotePacksInfo
	}

	// add r.addedPacks to the info
	for _, p := range r.addedPacks {
		pk.TexturePacks = append(pk.TexturePacks, protocol.TexturePackInfo{
			UUID:            p.UUID(),
			Version:         p.Version(),
			Size:            uint64(p.Len()),
			ContentKey:      p.ContentKey(),
			SubPackName:     p.Name(),
			ContentIdentity: "",
			HasScripts:      false,
			RTXEnabled:      false,
		})
	}

	return &pk
}

func (r *rpHandler) ResourcePacks() []*resource.Pack {
	select {
	case <-r.receivedRemoteStack:
	case <-r.ctx.Done():
	case <-r.Server.OnDisconnect():
	}
	r.dlwg.Wait()
	// wait for the whole receiving process to be done
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
