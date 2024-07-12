package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/utils"
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
	currentPack   resource.Pack
	currentOffset uint64

	serverPackAmount int
	// map[pack uuid]map[pack size]pack
	// this way servers with multiple packs on the same uuid work
	downloadingPacks map[string]map[uint64]downloadingPack
	awaitingPacks    map[string]*downloadingPack
	NextPack         chan resource.Pack
}

// downloadingPack is a resource pack that is being downloaded by a client connection.
type downloadingPack struct {
	chunkSize     uint32
	size          uint64
	expectedIndex uint32
	newFrag       chan *packet.ResourcePackChunkData
	contentKey    string

	ID      string
	Version string
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

	filterDownloadResourcePacks func(id string) bool

	// wait for downloads to be done
	dlwg sync.WaitGroup

	// gives access to stored resource packs in an abstract way so it can be replaced for replay
	cache iPackCache

	// queue for packs the client can receive

	nextPackToClient chan resource.Pack
	packsFromCache   []resource.Pack
	addedPacks       []resource.Pack
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
	resourcePacks []resource.Pack

	// optional callback when its known what resource packs the server has
	OnResourcePacksInfoCB func()

	// optional callback that is called as soon as a resource pack is added to the proxies list
	OnFinishedPack func(resource.Pack) error

	clientDone chan struct{}
}

func newRpHandler(ctx context.Context, addedPacks []resource.Pack, filterDownloadResourcePacks func(string) bool) *rpHandler {
	r := &rpHandler{
		ctx:        ctx,
		log:        logrus.WithField("part", "ResourcePacks"),
		addedPacks: addedPacks,
		downloadQueue: &resourcePackQueue{
			downloadingPacks: make(map[string]map[uint64]downloadingPack),
			awaitingPacks:    make(map[string]*downloadingPack),
			NextPack:         make(chan resource.Pack),
		},
		cache:                  &packCache{},
		receivedRemotePackInfo: make(chan struct{}),
		receivedRemoteStack:    make(chan struct{}),

		filterDownloadResourcePacks: filterDownloadResourcePacks,
	}
	return r
}

func (r *rpHandler) SetServer(c minecraft.IConn) {
	r.Server = c
}

func (r *rpHandler) SetClient(c minecraft.IConn) {
	r.Client = c
	r.nextPackToClient = make(chan resource.Pack)
	r.knowPacksRequestedFromServer = make(chan struct{})
	r.clientDone = make(chan struct{})
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
		packID := pack.UUID + "_" + pack.Version

		_, alreadyDownloading := r.downloadQueue.downloadingPacks[pack.UUID]
		alreadyIgnored := slices.ContainsFunc(r.ignoredResourcePacks, func(e exemptedResourcePack) bool { return e.uuid == pack.UUID })
		if alreadyDownloading || alreadyIgnored {
			r.log.Warnf("duplicate texture pack entry %v in resource pack info", pack.UUID)
			r.downloadQueue.serverPackAmount--
			continue
		}
		if r.cache.Has(pack.UUID, pack.Version) {
			r.ignoredResourcePacks = append(r.ignoredResourcePacks, exemptedResourcePack{
				uuid:    pack.UUID,
				version: pack.Version,
			})
			newPack, err := utils.PackFromBase(r.cache.Get(pack.UUID, pack.Version).WithContentKey(pack.ContentKey))
			if err != nil {
				r.log.Error(err)
			} else {
				r.resourcePacks = append(r.resourcePacks, newPack)
				err = r.OnFinishedPack(newPack)
				if err != nil {
					return err
				}
			}
			r.downloadQueue.serverPackAmount--
			continue
		}

		idxURL := slices.IndexFunc(pk.PackURLs, func(pu protocol.PackURL) bool { return pu.UUIDVersion == packID })
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
				newPack, err = utils.PackFromBase(newPack.WithContentKey(contentKey))
				if err != nil {
					r.log.Error(err)
					return
				}
				r.resourcePacks = append(r.resourcePacks, newPack)
				err = r.OnFinishedPack(newPack)
				if err != nil {
					r.log.Error(err)
					return
				}

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

		packsToDownload = append(packsToDownload, packID)

		m, ok := r.downloadQueue.downloadingPacks[pack.UUID]
		if !ok {
			m = make(map[uint64]downloadingPack)
			r.downloadQueue.downloadingPacks[pack.UUID] = m
		}
		m[pack.Size] = downloadingPack{
			size:       pack.Size,
			newFrag:    make(chan *packet.ResourcePackChunkData),
			contentKey: pack.ContentKey,
			ID:         pack.UUID,
			Version:    pack.Version,
		}
	}
	for _, pack := range pk.BehaviourPacks {
		packID := pack.UUID + "_" + pack.Version

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
			err := r.OnFinishedPack(newPack)
			if err != nil {
				return err
			}
			r.downloadQueue.serverPackAmount--
			continue
		}
		// This UUID_Version is a hack Mojang put in place.
		packsToDownload = append(packsToDownload, packID)

		m, ok := r.downloadQueue.downloadingPacks[pack.UUID]
		if !ok {
			m = make(map[uint64]downloadingPack)
			r.downloadQueue.downloadingPacks[pack.UUID] = m
		}
		m[pack.Size] = downloadingPack{
			size:       pack.Size,
			newFrag:    make(chan *packet.ResourcePackChunkData),
			contentKey: pack.ContentKey,
			ID:         pack.UUID,
			Version:    pack.Version,
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

	packsToDownload = slices.DeleteFunc(packsToDownload, func(id string) bool {
		ignore := r.filterDownloadResourcePacks(id)
		if ignore {
			idsplit := strings.Split(id, "_")
			r.ignoredResourcePacks = append(r.ignoredResourcePacks, exemptedResourcePack{
				uuid:    idsplit[0],
				version: idsplit[1],
			})
		}
		return ignore
	})

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
	packID := strings.Split(pk.UUID, "_")[0]
	packMap, ok := r.downloadQueue.downloadingPacks[packID]
	if !ok {
		// We either already downloaded the pack or we got sent an invalid UUID, that did not match any pack
		// sent in the ResourcePacksInfo packet.
		if _, ok := r.cache.(*replayCache); ok {
			return nil
		}
		return fmt.Errorf("unknown pack to download with UUID %v", pk.UUID)
	}

	pack, ok := packMap[pk.Size]
	if !ok {
		r.log.Infof("pack %v with size from ResourcePackDataInfo packet doesnt exist", pk.UUID)
		for _, p := range packMap {
			pack = p
			break
		}
	}

	if pack.size != pk.Size {
		// Size mismatch: The ResourcePacksInfo packet had a size for the pack that did not match with the
		// size sent here.
		r.log.Infof("pack %v had a different size in the ResourcePacksInfo packet than the ResourcePackDataInfo packet", pk.UUID)
		pack.size = pk.Size
	}

	// Remove the resource pack from the downloading packs and add it to the awaiting packets.
	delete(packMap, pk.Size)
	if len(packMap) == 0 {
		delete(r.downloadQueue.downloadingPacks, packID)
	}
	r.downloadQueue.awaitingPacks[pk.UUID] = &pack
	pack.chunkSize = pk.DataChunkSize

	// The client calculates the chunk count by itself: You could in theory send a chunk count of 0 even
	// though there's data, and the client will still download normally.
	chunkCount := uint32(pk.Size / uint64(pk.DataChunkSize))
	if pk.Size%uint64(pk.DataChunkSize) != 0 {
		chunkCount++
	}

	f, err := r.cache.Create(pack.ID, pack.Version)
	if err != nil {
		return err
	}
	dataWritten := 0

	for i := uint32(0); i < chunkCount; i++ {
		_ = r.Server.WritePacket(&packet.ResourcePackChunkRequest{
			UUID:       pk.UUID,
			ChunkIndex: i,
		})
		select {
		case <-r.Server.OnDisconnect():
			return net.ErrClosed
		case frag := <-pack.newFrag:
			// Write the fragment to the full buffer of the downloading resource pack.

			lastData := dataWritten+int(pack.chunkSize) >= int(pack.size)
			if !lastData && uint32(len(frag.Data)) != pack.chunkSize {
				// The chunk data didn't have the full size and wasn't the last data to be sent for the resource pack,
				// meaning we got too little data.
				return fmt.Errorf("resource pack chunk data had a length of %v, but expected %v", len(frag.Data), pack.chunkSize)
			}

			if frag.DataOffset != uint64(dataWritten) {
				return fmt.Errorf("resourcepack current offset %d != %d fragment offset", dataWritten, frag.DataOffset)
			}

			_, err := f.Write(frag.Data)
			if err != nil {
				return err
			}
			dataWritten += len(frag.Data)
		}
	}
	close(pack.newFrag)
	r.packMu.Lock()
	defer r.packMu.Unlock()

	if dataWritten != int(pack.size) {
		return fmt.Errorf("incorrect resource pack size: expected %v, but got %v", pack.size, dataWritten)
	}

	err = f.Close()
	if err != nil {
		return err
	}

	newPack, err := resource.ReadPath(f.FinalName)
	if err != nil {
		return fmt.Errorf("invalid full resource pack data for UUID %v: %v", pk.UUID, err)
	}
	newPack, err = utils.PackFromBase(newPack.WithContentKey(pack.contentKey))
	if err != nil {
		return fmt.Errorf("invalid full resource pack data for UUID %v: %v", pk.UUID, err)
	}

	r.downloadQueue.serverPackAmount--
	// Finally we add the resource to the resource packs slice.
	r.resourcePacks = append(r.resourcePacks, newPack)
	err = r.OnFinishedPack(newPack)
	if err != nil {
		return err
	}

	// if theres a client and the client needs resource packs send it to its queue
	if r.nextPackToClient != nil {
		if slices.Contains(r.packsRequestedFromServer, pk.UUID) {
			r.log.Debugf("sending pack %s from server to client", newPack.Name())
			r.nextPackToClient <- newPack
		}
	}

	// finished downloading
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
	pack.newFrag <- pk
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

	if r.clientDone != nil {
		<-r.clientDone
	}

	r.Server.Expect(packet.IDStartGame)
	_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseCompleted})
	return nil
}

// nextResourcePackDownload moves to the next resource pack to download and sends a resource pack data info
// packet with information about it.
func (r *rpHandler) nextResourcePackDownload() (ok bool, err error) {
	var pack resource.Pack

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

	err = r.Client.WritePacket(&packet.ResourcePackDataInfo{
		UUID:          pack.UUID(),
		DataChunkSize: packChunkSize,
		ChunkCount:    uint32(pack.DataChunkCount(packChunkSize)),
		Size:          uint64(pack.Len()),
		Hash:          checksum[:],
		PackType:      packType,
	})
	if err != nil {
		return false, err
	}
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
	packDone := false
	if n, err := current.ReadAt(response.Data, int64(response.DataOffset)); err != nil {
		// If we hit an EOF, we don't need to return an error, as we've simply reached the end of the content
		// AKA the last chunk.
		if err != io.EOF {
			return fmt.Errorf("error reading resource pack chunk: %v", err)
		}
		response.Data = response.Data[:n]
		packDone = true
	}
	if err := r.Client.WritePacket(response); err != nil {
		return fmt.Errorf("error writing resource pack chunk data packet: %v", err)
	}

	if packDone {
		ok, err := r.nextResourcePackDownload()
		if err != nil {
			return err
		}
		if !ok {
			r.Client.Expect(packet.IDResourcePackClientResponse)
		}
	}

	return nil
}

func (r *rpHandler) processClientRequest(packs []string) error {
	<-r.receivedRemotePackInfo

	r.nextPackToClient = make(chan resource.Pack, len(packs))

	for _, packUUID := range packs {
		uuid_ver := strings.Split(packUUID, "_")
		id, ver := uuid_ver[0], uuid_ver[1]

		found := false
		if r.cache.Has(id, ver) {
			r.log.Debugf("using %s from cache", packUUID)

			pack := r.cache.Get(id, ver)

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
					r.packsRequestedFromServer = append(r.packsRequestedFromServer, packUUID)
					break
				}
			}
		}

		if !found {
			for _, pack := range r.remotePacksInfo.BehaviourPacks {
				if pack.UUID+"_"+pack.Version == packUUID {
					found = true
					r.packsRequestedFromServer = append(r.packsRequestedFromServer, packUUID)
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
		close(r.clientDone)
		return r.Client.Close()
	case packet.PackResponseSendPacks:
		r.clientHasRequested = true
		if err := r.processClientRequest(pk.PacksToDownload); err != nil {
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

		if err := r.Client.WritePacket(r.remoteStack); err != nil {
			return fmt.Errorf("error writing resource pack stack packet: %v", err)
		}
	case packet.PackResponseCompleted:
		close(r.clientDone)
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
		for _, bp := range r.remotePacksInfo.BehaviourPacks {
			pk.BehaviourPacks = append(pk.BehaviourPacks, bp)
		}
		for _, rp := range r.remotePacksInfo.TexturePacks {
			rp.RTXEnabled = false
			pk.TexturePacks = append(pk.TexturePacks, rp)
		}
		pk.TexturePackRequired = r.remotePacksInfo.TexturePackRequired
		pk.HasAddons = r.remotePacksInfo.HasAddons
		pk.HasScripts = r.remotePacksInfo.HasScripts
		pk.ForcingServerPacks = r.remotePacksInfo.ForcingServerPacks
		pk.PackURLs = r.remotePacksInfo.PackURLs
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

func (r *rpHandler) ResourcePacks() []resource.Pack {
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
