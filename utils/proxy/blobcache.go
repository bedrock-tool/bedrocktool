package proxy

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type waitBlob struct {
	index   int
	isBiome bool
	pos     world.SubChunkPos
	ent     *protocol.SubChunkEntry
}

type Blobcache struct {
	db      *leveldb.DB
	mu      sync.Mutex
	session *Session

	waitingReceive map[uint64][]waitBlob

	clientWait map[uint64]*sync.WaitGroup

	OnBlobs func(blobs []BlobResp, fromCache bool) error
}

func NewBlobCache(session *Session) (*Blobcache, error) {
	db, err := leveldb.OpenFile("blobcache", nil)
	if err != nil {
		return nil, err
	}
	return &Blobcache{
		db:             db,
		session:        session,
		waitingReceive: make(map[uint64][]waitBlob),
		clientWait:     make(map[uint64]*sync.WaitGroup),
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

// server packets

func (b *Blobcache) HandleLevelChunk(pk *packet.LevelChunk) error {
	if !pk.CacheEnabled {
		return nil
	}
	subPos := world.SubChunkPos{pk.Position[0], 0, pk.Position[1]}

	b.mu.Lock()
	defer b.mu.Unlock()

	var reply packet.ClientCacheBlobStatus
	var blobs []BlobResp

	for i, blobHash := range pk.BlobHashes {
		blob, err := b.loadBlob(blobHash)
		if err != nil {
			return err
		}
		if blob == nil {
			_, alreadyWaiting := b.waitingReceive[blobHash]
			if !alreadyWaiting {
				reply.MissHashes = append(reply.MissHashes, blobHash)
			} else {
				reply.HitHashes = append(reply.HitHashes, blobHash)
			}

			b.waitingReceive[blobHash] = append(b.waitingReceive[blobHash], waitBlob{
				index:   i,
				isBiome: i == len(pk.BlobHashes)-1,
				pos:     subPos,
				ent:     nil,
			})
			continue
		}
		reply.HitHashes = append(reply.HitHashes, blobHash)
		blobs = append(blobs, BlobResp{
			Index:    i,
			IsBiome:  i == len(pk.BlobHashes)-1,
			Position: subPos,
			Entry:    nil,
			Payload:  blob,
		})
	}

	if len(reply.HitHashes)+len(reply.MissHashes) > 0 {
		err := b.session.Server.WritePacket(&reply)
		if err != nil {
			return err
		}
	}

	if b.OnBlobs != nil && len(blobs) > 0 {
		err := b.OnBlobs(blobs, true)
		if err != nil {
			return err
		}
	}

	return nil
}

type BlobResp struct {
	Hash     uint64
	Index    int
	IsBiome  bool
	Position world.SubChunkPos
	Entry    *protocol.SubChunkEntry
	Payload  []byte
}

func (b *Blobcache) HandleSubChunk(pk *packet.SubChunk) error {
	if !pk.CacheEnabled {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	var reply packet.ClientCacheBlobStatus
	var blobs []BlobResp
	for _, entry := range pk.SubChunkEntries {
		if entry.Result != protocol.SubChunkResultSuccess {
			continue
		}
		blob, err := b.loadBlob(entry.BlobHash)
		if err != nil {
			return err
		}
		if blob == nil {
			_, alreadyWaiting := b.waitingReceive[entry.BlobHash]
			if !alreadyWaiting {
				reply.MissHashes = append(reply.MissHashes, entry.BlobHash)
			} else {
				reply.HitHashes = append(reply.HitHashes, entry.BlobHash)
			}

			b.waitingReceive[entry.BlobHash] = append(b.waitingReceive[entry.BlobHash], waitBlob{
				pos: world.SubChunkPos(pk.Position),
				ent: &entry,
			})
			continue
		}
		reply.HitHashes = append(reply.HitHashes, entry.BlobHash)
		blobs = append(blobs, BlobResp{
			Hash:     entry.BlobHash,
			Index:    -1,
			Position: world.SubChunkPos(pk.Position),
			Entry:    &entry,
			Payload:  blob,
		})
	}

	if len(reply.HitHashes)+len(reply.MissHashes) > 0 {
		err := b.session.Server.WritePacket(&reply)
		if err != nil {
			return err
		}
	}

	if b.OnBlobs != nil && len(blobs) > 0 {
		err := b.OnBlobs(blobs, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Blobcache) HandleClientCacheMissResponse(pk *packet.ClientCacheMissResponse) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var blobs []BlobResp
	for _, blob := range pk.Blobs {
		waiters, ok := b.waitingReceive[blob.Hash]
		if !ok {
			if !b.session.isReplay {
				logrus.Warnf("Received Unexpected Blob Hash!?!?")
			}
			continue
		}
		delete(b.waitingReceive, blob.Hash)

		err := b.db.Put(blobKey(blob.Hash), blob.Payload, nil)
		if err != nil {
			return err
		}

		for _, wait := range waiters {
			blobs = append(blobs, BlobResp{
				Hash:     blob.Hash,
				Index:    wait.index,
				IsBiome:  wait.isBiome,
				Position: wait.pos,
				Entry:    wait.ent,
				Payload:  blob.Payload,
			})
			w, ok := b.clientWait[blob.Hash]
			if ok {
				delete(b.clientWait, blob.Hash)
				w.Done()
			}
		}
	}

	if b.OnBlobs != nil && len(blobs) > 0 {
		err := b.OnBlobs(blobs, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// client packets

func (b *Blobcache) HandleClientCacheBlobStatus(pk *packet.ClientCacheBlobStatus) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var blobs []protocol.CacheBlob
	var wait []uint64
	var wg sync.WaitGroup
	var haveAll = true

	for _, blobHash := range pk.MissHashes {
		blob, err := b.loadBlob(blobHash)
		if err != nil {
			return err
		}
		if blob == nil {
			wg.Add(1)
			b.clientWait[blobHash] = &wg
			wait = append(wait, blobHash)
			haveAll = false
			continue
		}
		blobs = append(blobs, protocol.CacheBlob{Hash: blobHash, Payload: blob})
	}

	if haveAll {
		return b.session.Client.WritePacket(&packet.ClientCacheMissResponse{
			Blobs: blobs,
		})
	}

	go func() {
		wg.Wait()
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, blobHash := range wait {
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
		b.session.Client.WritePacket(&packet.ClientCacheMissResponse{
			Blobs: blobs,
		})
	}()

	return nil
}
