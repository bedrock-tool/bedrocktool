package resourcepacks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/google/uuid"
)

// packChunkSize is the size of a single chunk of data from a resource pack: 512 kB or 0.5 MB
const packChunkSize = 1024 * 128

// downloadingPack is a resource pack that is being downloaded by a client connection.
type downloadingPack struct {
	chunkSize     uint32
	size          uint64
	expectedIndex uint32
	newFrag       chan *packet.ResourcePackChunkData
	contentKey    string

	ID      uuid.UUID
	Version string
}

type uploadingPack struct {
	Pack          resource.Pack
	currentOffset uint64
}

type ResourcePackHandler struct {
	ctx context.Context
	log *logrus.Entry

	Server minecraft.IConn
	Client minecraft.IConn

	//
	// callbacks
	//

	// optional callback when its known what resource packs the server has
	OnResourcePacksInfoCB func()
	// optional callback that is called as soon as a resource pack is added to the proxies list
	OnFinishedPack              func(resource.Pack) error
	FilterDownloadResourcePacks func(id string) bool

	//
	// common
	//

	// gives access to stored resource packs in an abstract way so it can be replaced for replay
	cache      PackCache
	addedPacks []resource.Pack

	// all active resource packs for access by the proxy
	resourcePacks     []resource.Pack
	finishedPacks     []string
	lockResourcePacks sync.Mutex

	//
	// for download
	//

	// map[pack uuid]map[pack size]pack
	// this way servers with multiple packs on the same uuid work
	downloadingPacks map[uuid.UUID]map[uint64]downloadingPack
	awaitingPack     *downloadingPack
	packDownloads    chan *packet.ResourcePackDataInfo

	// wait for downloads to be done
	dlwg sync.WaitGroup

	// closed when the proxy has received resource pack info from the server
	receivedRemotePackInfo chan struct{}
	remotePacksInfo        *packet.ResourcePacksInfo

	// closed when the proxy has received the resource pack stack from the server
	receivedRemoteStack chan struct{}
	remoteStack         *packet.ResourcePackStack

	//
	// for upload
	//

	// closed when decided what packs to download from the server
	knowPacksRequestedFromServer chan struct{}
	packsRequestedFromServer     []string

	// set to true if the client wants any resource packs
	// if its false when the client sends the `done` message, that means the nextPack channel should be closed
	clientHasRequested bool
	allPacksDownloaded bool

	// queue for packs the client can receive
	nextPackToClient chan resource.Pack
	uploads          map[uuid.UUID]*uploadingPack
	uploadLock       sync.Mutex
	clientDone       chan struct{}
}

func NewResourcePackHandler(ctx context.Context, addedPacks []resource.Pack) *ResourcePackHandler {
	r := &ResourcePackHandler{
		ctx:        ctx,
		log:        logrus.WithField("part", "ResourcePacks"),
		addedPacks: addedPacks,

		cache:                  &packCache{},
		receivedRemotePackInfo: make(chan struct{}),
		receivedRemoteStack:    make(chan struct{}),
		downloadingPacks:       make(map[uuid.UUID]map[uint64]downloadingPack),
	}
	return r
}

func (r *ResourcePackHandler) SetCache(c PackCache) {
	r.cache = c
}

func (r *ResourcePackHandler) SetServer(c minecraft.IConn) {
	r.Server = c
}

func (r *ResourcePackHandler) SetClient(c minecraft.IConn) {
	r.Client = c
	r.knowPacksRequestedFromServer = make(chan struct{})
	r.clientDone = make(chan struct{})
}

var httpClient = http.Client{
	Transport: &http.Transport{
		ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{
			NextProtos: []string{"http/1.1"},
		},
	},
}

func (r *ResourcePackHandler) downloadFromUrl(pack protocol.TexturePackInfo) error {
	defer r.dlwg.Done()
	r.log.Infof("Downloading Resourcepack: %s", pack.DownloadURL)

	f, err := r.cache.Create(pack.UUID, pack.Version)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", pack.DownloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	size, err := io.Copy(f, res.Body)
	if err != nil {
		return err
	}

	err = f.Move()
	if err != nil {
		return err
	}

	newPack, err := resource.FromReaderAt(f, size)
	if err != nil {
		return err
	}
	if len(pack.ContentKey) > 0 {
		newPack = newPack.WithContentKey(pack.ContentKey)
	}
	newPack, err = utils.PackFromBase(newPack)
	if err != nil {
		return err
	}

	r.lockResourcePacks.Lock()
	r.resourcePacks = append(r.resourcePacks, newPack)
	r.finishedPacks = append(r.finishedPacks, pack.UUID.String()+"_"+pack.Version)
	err = r.OnFinishedPack(newPack)
	r.lockResourcePacks.Unlock()
	if err != nil {
		return err
	}

	if r.nextPackToClient != nil {
		select {
		case <-r.knowPacksRequestedFromServer:
		case <-r.ctx.Done():
			return r.ctx.Err()
		}
		if slices.Contains(r.packsRequestedFromServer, pack.UUID.String()+"_"+pack.Version) {
			r.nextPackToClient <- newPack
		}
	}
	return nil
}

// from server
func (r *ResourcePackHandler) OnResourcePacksInfo(pk *packet.ResourcePacksInfo) error {
	if r.OnResourcePacksInfoCB != nil {
		r.OnResourcePacksInfoCB()
	}
	// First create a new resource pack queue with the information in the packet so we can download them
	// properly later.
	packsToDownload := make([]string, 0, len(pk.TexturePacks))

	var urlDownloads []protocol.TexturePackInfo
	for _, pack := range pk.TexturePacks {
		packID := pack.UUID.String() + "_" + pack.Version
		if r.FilterDownloadResourcePacks(packID) {
			continue
		}

		_, alreadyDownloading := r.downloadingPacks[pack.UUID]
		if alreadyDownloading {
			r.log.Warnf("duplicate texture pack entry %v in resource pack info", pack.UUID)
			continue
		}

		if r.cache.Has(pack.UUID, pack.Version) {
			newPack, err := r.cache.Get(pack.UUID, pack.Version)
			if err != nil {
				return fmt.Errorf("opening cached Resourcepack: %s (delete packcache as a fix)", err)
			}

			if len(pack.ContentKey) > 0 {
				newPack = newPack.WithContentKey(pack.ContentKey)
			}
			newPack, err = utils.PackFromBase(newPack)
			if err != nil {
				return err
			}

			r.lockResourcePacks.Lock()
			r.resourcePacks = append(r.resourcePacks, newPack)
			r.finishedPacks = append(r.finishedPacks, pack.UUID.String()+"_"+pack.Version)
			err = r.OnFinishedPack(newPack)
			r.lockResourcePacks.Unlock()
			if err != nil {
				return err
			}
			continue
		}

		if pack.DownloadURL != "" {
			urlDownloads = append(urlDownloads, pack)
			continue
		}

		packsToDownload = append(packsToDownload, packID)
		m, ok := r.downloadingPacks[pack.UUID]
		if !ok {
			m = make(map[uint64]downloadingPack)
			r.downloadingPacks[pack.UUID] = m
		}
		m[pack.Size] = downloadingPack{
			size:       pack.Size,
			newFrag:    make(chan *packet.ResourcePackChunkData),
			contentKey: pack.ContentKey,
			ID:         pack.UUID,
			Version:    pack.Version,
		}
	}

	if len(urlDownloads) > 0 {
		r.dlwg.Add(len(urlDownloads))
		go func() {
			for _, dl := range urlDownloads {
				if err := r.downloadFromUrl(dl); err != nil {
					r.log.Errorf("download %s %s", dl.DownloadURL, err)
				}
			}
		}()
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

	if len(packsToDownload) == 0 {
		r.dlwg.Wait()
		r.Server.Expect(packet.IDResourcePackStack)
		return r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseAllPacksDownloaded})
	}

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
	return r.Server.WritePacket(&packet.ResourcePackClientResponse{
		Response:        packet.PackResponseSendPacks,
		PacksToDownload: packsToDownload,
	})
}

func (r *ResourcePackHandler) downloadResourcePack(pk *packet.ResourcePackDataInfo) error {
	packID, err := uuid.Parse(strings.Split(pk.UUID, "_")[0])
	if err != nil {
		return err
	}
	packMap, ok := r.downloadingPacks[packID]
	if !ok {
		// We either already downloaded the pack or we got sent an invalid UUID, that did not match any pack
		// sent in the ResourcePacksInfo packet.
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
		delete(r.downloadingPacks, packID)
	}
	r.awaitingPack = &pack
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
	h := sha256.New()
	w := io.MultiWriter(f, h)

	chunksRequested := 0
	dataWritten := 0
	// request first
	err = r.Server.WritePacket(&packet.ResourcePackChunkRequest{
		UUID:       pk.UUID,
		ChunkIndex: 0,
	})
	if err != nil {
		return err
	}
	chunksRequested++
	for {
		var frag *packet.ResourcePackChunkData
		var ok bool
		select {
		case <-r.Server.Context().Done():
			return net.ErrClosed
		case frag, ok = <-pack.newFrag:
		}
		if !ok {
			break
		}

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

		_, err := w.Write(frag.Data)
		if err != nil {
			return err
		}
		dataWritten += len(frag.Data)

		if lastData {
			break
		}

		if chunksRequested < int(chunkCount) {
			err = r.Server.WritePacket(&packet.ResourcePackChunkRequest{
				UUID:       pk.UUID,
				ChunkIndex: uint32(chunksRequested),
			})
			if err != nil {
				return err
			}
			chunksRequested++
		}
	}

	/*
		sf, ok := r.Server.(interface {
			Stats() *raknet.RakNetStatistics
		})
		if ok {
			stats := sf.Stats()
			utils.DumpStruct(os.Stdout, stats.Total)
		}
	*/

	if dataWritten != int(pack.size) {
		return fmt.Errorf("incorrect resource pack size: expected %v, but got %v", pack.size, dataWritten)
	}

	// check for hash to match
	sum := h.Sum(nil)
	if !bytes.Equal(pk.Hash, sum) {
		return fmt.Errorf("resource pack download error, hash mismatch in download %s", packID)
	}

	// rename the cache file to its final name
	err = f.Move()
	if err != nil {
		return err
	}

	newPack, err := resource.FromReaderAt(f, int64(pack.size))
	if err != nil {
		return fmt.Errorf("invalid full resource pack data for UUID %v: %v", pk.UUID, err)
	}
	newPack, err = utils.PackFromBase(newPack.WithContentKey(pack.contentKey))
	if err != nil {
		return fmt.Errorf("invalid full resource pack data for UUID %v: %v", pk.UUID, err)
	}

	// Finally we add the resource to the resource packs slice.
	r.lockResourcePacks.Lock()
	r.resourcePacks = append(r.resourcePacks, newPack)
	r.finishedPacks = append(r.finishedPacks, pack.ID.String()+"_"+pack.Version)
	err = r.OnFinishedPack(newPack)
	r.lockResourcePacks.Unlock()
	if err != nil {
		return err
	}

	// if theres a client and the client needs resource packs send it to its queue
	if r.nextPackToClient != nil {
		if slices.Contains(r.packsRequestedFromServer, pack.ID.String()+"_"+pack.Version) {
			r.log.Debugf("sending pack %s from server to client", newPack.Name())
			r.nextPackToClient <- newPack
		}
	}

	// finished downloading
	if len(r.downloadingPacks) == 0 {
		r.dlwg.Wait()
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
func (r *ResourcePackHandler) OnResourcePackDataInfo(pk *packet.ResourcePackDataInfo) error {
	if _, ok := r.cache.(*ReplayCache); ok {
		return nil
	}

	r.packDownloads <- pk
	return nil
}

// from server
func (r *ResourcePackHandler) OnResourcePackChunkData(pk *packet.ResourcePackChunkData) error {
	if _, ok := r.cache.(*ReplayCache); ok {
		return nil
	}

	pack := r.awaitingPack
	if pack == nil {
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
func (r *ResourcePackHandler) OnResourcePackStack(pk *packet.ResourcePackStack) error {
	// We currently don't apply resource packs in any way, so instead we just check if all resource packs in
	// the stacks are also downloaded.
	for _, pack := range pk.TexturePacks {
		for i, behaviourPack := range pk.BehaviourPacks {
			if pack.UUID == behaviourPack.UUID {
				// We had a behaviour pack with the same UUID as the texture pack, so we drop the texture
				// pack and log it.
				r.log.Warnf("dropping behaviour pack with UUID %v due to a texture pack with the same UUID", pack.UUID)
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

	// r.addedPacks to the stack
	var addPacks []protocol.StackResourcePack
	for _, p := range r.addedPacks {
		addPacks = append(addPacks, protocol.StackResourcePack{
			UUID:        p.UUID().String(),
			Version:     p.Version(),
			SubPackName: p.Name(),
		})
	}
	pk.TexturePacks = append(addPacks, pk.TexturePacks...)

	r.remoteStack = pk
	close(r.receivedRemoteStack)

	if r.clientDone != nil {
		r.log.Debug("waiting for client to finish downloading")
		<-r.clientDone
	}

	r.dlwg.Wait()
	r.log.Debug("starting game")
	r.Server.Expect(packet.IDItemRegistry, packet.IDStartGame)
	_ = r.Server.WritePacket(&packet.ResourcePackClientResponse{Response: packet.PackResponseCompleted})
	return nil
}

// from client
func (r *ResourcePackHandler) OnResourcePackChunkRequest(pk *packet.ResourcePackChunkRequest) error {
	packID, err := uuid.Parse(pk.UUID)
	if err != nil {
		return err
	}
	r.uploadLock.Lock()
	upload, ok := r.uploads[packID]
	r.uploadLock.Unlock()
	if !ok {
		return fmt.Errorf("client requested an unknown resourcepack chunk %s", packID)
	}

	if upload.Pack.UUID() != packID {
		return fmt.Errorf("resource pack chunk request had unexpected UUID: expected %v, but got %v", upload.Pack.UUID(), packID)
	}
	if upload.currentOffset != uint64(pk.ChunkIndex)*packChunkSize {
		return fmt.Errorf("resource pack chunk request had unexpected chunk index: expected %v, but got %v", upload.currentOffset/packChunkSize, pk.ChunkIndex)
	}
	response := &packet.ResourcePackChunkData{
		UUID:       pk.UUID,
		ChunkIndex: pk.ChunkIndex,
		DataOffset: upload.currentOffset,
		Data:       make([]byte, packChunkSize),
	}

	upload.currentOffset += packChunkSize
	// We read the data directly into the response's data.
	var done bool
	if n, err := upload.Pack.ReadAt(response.Data, int64(response.DataOffset)); err != nil {
		// If we hit an EOF, we don't need to return an error, as we've simply reached the end of the content
		// AKA the last chunk.
		if err != io.EOF {
			return fmt.Errorf("error reading resource pack chunk: %v", err)
		}
		response.Data = response.Data[:n]
		done = true
	}
	if err := r.Client.WritePacket(response); err != nil {
		return fmt.Errorf("error writing resource pack chunk data packet: %v", err)
	}

	r.uploadLock.Lock()
	if !done {
		// expect next chunk for this pack to be requested
		r.Client.Expect(packet.IDResourcePackChunkRequest)
	} else {
		// this pack is done
		delete(r.uploads, packID)
	}

	// when theres nothing left to upload, the client is done
	if len(r.uploads) == 0 {
		r.Client.Expect(packet.IDResourcePackClientResponse)
	}
	r.uploadLock.Unlock()
	return nil
}

func (r *ResourcePackHandler) processClientRequest(packs []string) error {
	<-r.receivedRemotePackInfo
	var packsFromCache []resource.Pack
	var addedPacksRequested []resource.Pack

	if len(packs) == 0 {
		close(r.knowPacksRequestedFromServer)
		return nil
	}

	var contentKeys = make(map[string]string)
	for _, pack := range r.remotePacksInfo.TexturePacks {
		contentKeys[pack.UUID.String()+"_"+pack.Version] = pack.ContentKey
	}

loopPacks:
	for _, packUUID := range packs {
		uuid_ver := strings.Split(packUUID, "_")
		id, err := uuid.Parse(uuid_ver[0])
		if err != nil {
			return err
		}
		ver := uuid_ver[1]

		if r.cache.Has(id, ver) {
			r.log.Debugf("using %s from cache", packUUID)

			pack, err := r.cache.Get(id, ver)
			if err != nil {
				return err
			}
			if contentKey, ok := contentKeys[packUUID]; ok {
				pack = pack.WithContentKey(contentKey)
			}
			packsFromCache = append(packsFromCache, pack)
			continue
		}

		for _, pack := range r.addedPacks {
			if pack.UUID().String()+"_"+pack.Version() == packUUID {
				addedPacksRequested = append(addedPacksRequested, pack)
				continue loopPacks
			}
		}

		for _, pack := range r.remotePacksInfo.TexturePacks {
			if pack.UUID.String()+"_"+pack.Version == packUUID {
				r.packsRequestedFromServer = append(r.packsRequestedFromServer, packUUID)
				continue loopPacks
			}
		}

		return fmt.Errorf("could not find resource pack %v", packUUID)
	}

	if len(packsFromCache)+len(r.packsRequestedFromServer)+len(addedPacksRequested) < len(packs) {
		r.log.Errorf("BUG: not enough packs sent to client, client will stall %d + %d  %d", len(packsFromCache), len(r.packsRequestedFromServer), len(packs))
	}

	r.nextPackToClient = make(chan resource.Pack, len(packs))
	for _, pack := range addedPacksRequested {
		r.nextPackToClient <- pack
	}
	for _, pack := range packsFromCache {
		r.nextPackToClient <- pack
	}
	if len(r.packsRequestedFromServer) == 0 {
		close(r.nextPackToClient)
	}

	close(r.knowPacksRequestedFromServer)
	return nil
}

// from client
func (r *ResourcePackHandler) OnResourcePackClientResponse(pk *packet.ResourcePackClientResponse) error {
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

		r.uploads = make(map[uuid.UUID]*uploadingPack)
		go func() {
			for {
				select {
				case pack, ok := <-r.nextPackToClient:
					if !ok {
						r.log.Info("finished sending client resource packs")
						return
					}

					r.log.Debugf("next pack %s", pack.Name())
					r.uploadLock.Lock()
					r.uploads[pack.UUID()] = &uploadingPack{
						Pack:          pack,
						currentOffset: 0,
					}
					r.uploadLock.Unlock()

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

					checksum := pack.Checksum()
					r.Client.Expect(packet.IDResourcePackChunkRequest)
					err := r.Client.WritePacket(&packet.ResourcePackDataInfo{
						UUID:          pack.UUID().String(),
						DataChunkSize: packChunkSize,
						ChunkCount:    uint32(pack.DataChunkCount(packChunkSize)),
						Size:          uint64(pack.Len()),
						Hash:          checksum[:],
						PackType:      packType,
					})
					if err != nil {
						r.log.Error(err)
						return
					}

				case <-r.ctx.Done():
					return
				}
			}
		}()

	case packet.PackResponseAllPacksDownloaded:
		if r.allPacksDownloaded {
			return nil
		}
		r.allPacksDownloaded = true
		if !r.clientHasRequested {
			close(r.knowPacksRequestedFromServer)
		}

		r.log.Debug("waiting for remote stack")
		select {
		case <-r.receivedRemoteStack:
		case <-r.ctx.Done():
			return context.Cause(r.ctx)
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

func (r *ResourcePackHandler) GetResourcePacksInfo(texturePacksRequired bool) *packet.ResourcePacksInfo {
	select {
	case <-r.receivedRemotePackInfo:
	case <-r.ctx.Done():
	}

	var pk packet.ResourcePacksInfo
	if r.remotePacksInfo != nil {
		pk.TexturePackRequired = r.remotePacksInfo.TexturePackRequired
		pk.HasAddons = r.remotePacksInfo.HasAddons
		pk.HasScripts = r.remotePacksInfo.HasScripts
		pk.TexturePacks = append(pk.TexturePacks, r.remotePacksInfo.TexturePacks...)
	}

	// add r.addedPacks to the info
	for _, p := range r.addedPacks {
		pk.TexturePacks = append(pk.TexturePacks, protocol.TexturePackInfo{
			UUID:            p.UUID(),
			Version:         p.Version(),
			Size:            uint64(p.Len()),
			ContentKey:      p.ContentKey(),
			SubPackName:     p.Name(),
			ContentIdentity: p.UUID().String(),
			HasScripts:      false,
			RTXEnabled:      false,
		})
	}

	return &pk
}

func (r *ResourcePackHandler) ResourcePacks() []resource.Pack {
	select {
	case <-r.receivedRemoteStack:
	case <-r.ctx.Done():
	case <-r.Server.Context().Done():
	}
	r.dlwg.Wait()
	// wait for the whole receiving process to be done
	return r.resourcePacks
}

var exemptedPacks = map[string]bool{
	"0fba4063-dba1-4281-9b89-ff9390653530_1.0.0": true,
}

func (r *ResourcePackHandler) hasPack(uuid string, version string, hasBehaviours bool) bool {
	if exemptedPacks[uuid+"_"+version] {
		// The server may send this resource pack on the stack without sending it in the info, as the client
		// always has it downloaded.
		return true
	}

	if uuid == "" {
		return true
	}

	search := uuid + "_" + version
	return slices.Contains(r.finishedPacks, search)
}
