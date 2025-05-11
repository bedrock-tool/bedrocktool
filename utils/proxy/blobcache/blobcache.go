package blobcache

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/storage"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

const maxInflightBlobs = 90

type clientWait struct {
	count  int
	hashes []uint64
}

type serverWait struct {
	pkFill packet.Packet
	count  int
}

type PacketFunc = func(pk packet.Packet, timeReceived time.Time, preLogin bool) error

type Blobcache struct {
	db  *leveldb.DB
	mu  sync.Mutex
	log *logrus.Entry

	queued []*packet.ClientCacheBlobStatus

	serverWait map[uint64][]*serverWait
	clientWait map[uint64]*clientWait

	levelChunksWaiting map[protocol.ChunkPos][]uint64
	subs               map[protocol.ChunkPos][]*serverWait

	writeServerPacket func(packet.Packet) error
	getClient         func() minecraft.IConn
	processPacket     PacketFunc
	onHitBlobs        func(blobs []protocol.CacheBlob)
	isReplay          bool
}

func NewBlobCache(
	log *logrus.Entry,
	writeServerPacket func(packet.Packet) error,
	getClient func() minecraft.IConn,
	processPacket PacketFunc,
	onHitBlobs func(blobs []protocol.CacheBlob),
	isReplay bool,
) (*Blobcache, error) {
	db, err := leveldb.OpenFile(utils.PathCache("blobcache"), nil)
	if err != nil {
		if checkShouldReadOnly(err) {
			db, err = leveldb.Open(storage.NewMemStorage(), nil)
		}
		if err != nil {
			return nil, err
		}
	}
	return &Blobcache{
		db:                 db,
		log:                log.WithField("in", "blobcache"),
		serverWait:         make(map[uint64][]*serverWait),
		clientWait:         make(map[uint64]*clientWait),
		levelChunksWaiting: make(map[protocol.ChunkPos][]uint64),
		subs:               make(map[protocol.ChunkPos][]*serverWait),

		writeServerPacket: writeServerPacket,
		getClient:         getClient,
		processPacket:     processPacket,
		onHitBlobs:        onHitBlobs,
		isReplay:          isReplay,
	}, nil
}

func blobKey(h uint64) []byte {
	k := binary.LittleEndian.AppendUint64(nil, h)
	return k
}

func (b *Blobcache) Close() error {
	return b.db.Close()
}

func (b *Blobcache) loadBlob(blobHash uint64) ([]byte, error) {
	blob, err := b.db.Get(blobKey(blobHash), nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return blob, nil
}

func (b *Blobcache) addServerWait(reply *packet.ClientCacheBlobStatus, wait *serverWait, blobHash uint64) {
	_, alreadyWaiting := b.serverWait[blobHash]
	if !alreadyWaiting {
		reply.MissHashes = append(reply.MissHashes, blobHash)
	}

	wait.count++
	b.serverWait[blobHash] = append(b.serverWait[blobHash], wait)
}

// server packets

func (b *Blobcache) HandleLevelChunk(pk *packet.LevelChunk, timeReceived time.Time, preLogin bool) (pkForward packet.Packet, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var reply packet.ClientCacheBlobStatus
	var pkFill = &packet.LevelChunk{
		Position:        pk.Position,
		Dimension:       pk.Dimension,
		HighestSubChunk: pk.HighestSubChunk,
		SubChunkCount:   pk.SubChunkCount,
		CacheEnabled:    pk.CacheEnabled,
		BlobHashes:      pk.BlobHashes,
		RawPayload:      pk.RawPayload,
	}
	var wait = serverWait{pkFill: pkFill}
	var hitBlobs []protocol.CacheBlob

	for _, blobHash := range pk.BlobHashes {
		blob, err := b.loadBlob(blobHash)
		if err != nil {
			return nil, err
		}
		if blob != nil {
			reply.HitHashes = append(reply.HitHashes, blobHash)
			hitBlobs = append(hitBlobs, protocol.CacheBlob{Hash: blobHash, Payload: blob})
		} else {
			b.addServerWait(&reply, &wait, blobHash)
			b.levelChunksWaiting[pk.Position] = append(b.levelChunksWaiting[pk.Position], blobHash)
		}
	}

	return b.finishWait(&reply, &wait, pk, hitBlobs, timeReceived, preLogin)
}

func (b *Blobcache) HandleSubChunk(pk *packet.SubChunk, timeReceived time.Time, preLogin bool) (pkForward packet.Packet, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var reply packet.ClientCacheBlobStatus
	var dependingChunks = make(map[protocol.ChunkPos]struct{})

	var pkFill = &packet.SubChunk{
		CacheEnabled:    false,
		Dimension:       pk.Dimension,
		Position:        pk.Position,
		SubChunkEntries: pk.SubChunkEntries,
	}

	var wait = serverWait{pkFill: pkFill}
	var hitBlobs []protocol.CacheBlob

	for _, entry := range pk.SubChunkEntries {
		if entry.Result != protocol.SubChunkResultSuccess {
			continue
		}

		var (
			absX = pk.Position[0] + int32(entry.Offset[0])
			absZ = pk.Position[2] + int32(entry.Offset[2])
			pos  = protocol.ChunkPos{absX, absZ}
		)
		dependingChunks[pos] = struct{}{}

		blob, err := b.loadBlob(entry.BlobHash)
		if err != nil {
			return nil, err
		}
		if blob != nil {
			reply.HitHashes = append(reply.HitHashes, entry.BlobHash)
			hitBlobs = append(hitBlobs, protocol.CacheBlob{Hash: entry.BlobHash, Payload: blob})
		} else {
			b.addServerWait(&reply, &wait, entry.BlobHash)
		}
	}

	// add reference to this to wait for the level chunk before processing
	for pos := range dependingChunks {
		if _, ok := b.levelChunksWaiting[pos]; ok {
			wait.count++
			b.subs[pos] = append(b.subs[pos], &wait)
		}
	}

	return b.finishWait(&reply, &wait, pk, hitBlobs, timeReceived, preLogin)
}

func (b *Blobcache) finishWait(reply *packet.ClientCacheBlobStatus, wait *serverWait, pk packet.Packet, hitBlobs []protocol.CacheBlob, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	// put the blobs from cache into current pcap if there is one
	if len(hitBlobs) > 0 {
		b.onHitBlobs(hitBlobs)
	}

	// queue up the misses to avoid too many in flight
	if len(reply.MissHashes) > 0 && len(b.serverWait) > maxInflightBlobs && false {
		replyCopy := *reply
		reply.MissHashes = nil
		b.queued = append(b.queued, &replyCopy)
		b.log.Tracef("queued %v", replyCopy.MissHashes)
	}

	// send reply if its not empty
	if len(reply.HitHashes)+len(reply.MissHashes) > 0 {
		//reply.MissHashes = removeDuplicate(reply.MissHashes)
		//reply.HitHashes = removeDuplicate(reply.HitHashes)
		err := b.writeServerPacket(reply)
		if err != nil {
			return nil, err
		}
	}

	// if have all hashes immediately process
	if wait.count == 0 {
		err := b.serverResolve(wait, timeReceived, preLogin)
		if err != nil {
			return nil, err
		}
	}

	// missing some hashes, if the client supports blobs send the unfilled packet
	// otherwise wait for the hashes to be resolved before sending the filled packet
	if client := b.getClient(); client != nil {
		ClientCacheEnabled := client.ClientCacheEnabled()
		if ClientCacheEnabled {
			return pk, nil
		}
		if wait.count == 0 {
			return wait.pkFill, nil
		}
	}
	return nil, nil
}

func (b *Blobcache) HandleClientCacheMissResponse(pk *packet.ClientCacheMissResponse, timeReceived time.Time, preLogin bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, blob := range pk.Blobs {
		// store blob
		err := b.db.Put(blobKey(blob.Hash), blob.Payload, nil)
		if err != nil {
			return err
		}

		// forward blob to client
		if w, ok := b.clientWait[blob.Hash]; ok {
			delete(b.clientWait, blob.Hash)
			w.count--
			if w.count == 0 {
				b.clientResolve(w)
			}
		}

		// get all packets that need this blob
		waiters, ok := b.serverWait[blob.Hash]
		if ok {
			delete(b.serverWait, blob.Hash)

			for _, wait := range waiters {
				wait.count--
				if wait.count == 0 {
					err = b.serverResolve(wait, timeReceived, preLogin)
					if err != nil {
						return err
					}
				}
			}
		} else {
			if !b.isReplay {
				logrus.Warnf("Received Unexpected Blob Hash %d", blob.Hash)
			}
		}
	}

	for len(b.queued) > 0 && len(b.serverWait) < maxInflightBlobs {
		reply := b.queued[0]
		b.queued = b.queued[1:]
		err := b.writeServerPacket(reply)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Blobcache) serverResolve(wait *serverWait, timeReceived time.Time, preLogin bool) error {
	switch pk := wait.pkFill.(type) {
	case *packet.LevelChunk:
		// mark packet as processed
		delete(b.levelChunksWaiting, pk.Position)

		prevPayload := pk.RawPayload
		pk.RawPayload = nil
		for _, hash := range pk.BlobHashes {
			blob, err := b.loadBlob(hash)
			if err != nil {
				logrus.Error(err)
				continue
			}
			pk.RawPayload = append(pk.RawPayload, blob...)
		}
		pk.RawPayload = append(pk.RawPayload, prevPayload...)
		pk.CacheEnabled = false
		pk.BlobHashes = nil
	case *packet.SubChunk:
		for i := 0; i < len(pk.SubChunkEntries); i++ {
			entry := &pk.SubChunkEntries[i]
			if entry.BlobHash == 0 {
				continue
			}
			blob, err := b.loadBlob(entry.BlobHash)
			if err != nil {
				logrus.Error(err)
				continue
			}
			entry.RawPayload = blob
			entry.BlobHash = 0
		}
		pk.CacheEnabled = false
	}

	err := b.processPacket(wait.pkFill, timeReceived, preLogin)
	if err != nil {
		return err
	}

	if client := b.getClient(); client != nil {
		ClientCacheEnabled := client.ClientCacheEnabled()
		if !ClientCacheEnabled {
			err = client.WritePacket(wait.pkFill)
			if err != nil {
				return err
			}
		}
	}

	if pk, ok := wait.pkFill.(*packet.LevelChunk); ok {
		if subs, ok := b.subs[pk.Position]; ok {
			delete(b.subs, pk.Position)
			for _, sub := range subs {
				sub.count--
				if sub.count == 0 {
					err = b.serverResolve(wait, timeReceived, preLogin)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (b *Blobcache) clientResolve(wait *clientWait) {
	var blobs []protocol.CacheBlob
	for _, blobHash := range wait.hashes {
		blob, err := b.loadBlob(blobHash)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if blob == nil {
			logrus.Error("blob waited and not found?")
			continue
		}
		blobs = append(blobs, protocol.CacheBlob{Hash: blobHash, Payload: blob})
	}
	client := b.getClient()
	client.WritePacket(&packet.ClientCacheMissResponse{
		Blobs: blobs,
	})
}

// client packets

func (b *Blobcache) HandleClientCacheBlobStatus(pk *packet.ClientCacheBlobStatus) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var blobs []protocol.CacheBlob
	var wait clientWait

	for _, blobHash := range pk.MissHashes {
		blob, err := b.loadBlob(blobHash)
		if err != nil {
			return err
		}
		if blob == nil {
			wait.count++
			wait.hashes = append(wait.hashes, blobHash)
			b.clientWait[blobHash] = &wait
			continue
		}
		blobs = append(blobs, protocol.CacheBlob{Hash: blobHash, Payload: blob})
	}

	if len(blobs) > 0 {
		client := b.getClient()
		return client.WritePacket(&packet.ClientCacheMissResponse{
			Blobs: blobs,
		})
	}

	return nil
}
