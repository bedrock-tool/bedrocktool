package proxy

import (
	"fmt"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

// hasPack checks if the connection has a resource pack downloaded with the UUID and version passed, provided
// the pack either has or does not have behaviours in it.
func (conn *replayConnector) hasPack(uuid string, version string, hasBehaviours bool) bool {
	conn.packMu.Lock()
	defer conn.packMu.Unlock()

	for _, pack := range conn.resourcePacks {
		if pack.UUID() == uuid && pack.Version() == version && pack.HasBehaviours() == hasBehaviours {
			return true
		}
	}
	return false
}

// handleResourcePackStack handles a ResourcePackStack packet sent by the server. The stack defines the order
// that resource packs are applied in.
func (conn *replayConnector) handleResourcePackStack(pk *packet.ResourcePackStack) error {
	// We currently don't apply resource packs in any way, so instead we just check if all resource packs in
	// the stacks are also downloaded.
	for _, pack := range pk.TexturePacks {
		for i, behaviourPack := range pk.BehaviourPacks {
			if pack.UUID == behaviourPack.UUID {
				// We had a behaviour pack with the same UUID as the texture pack, so we drop the texture
				// pack and log it.
				logrus.Printf("dropping behaviour pack with UUID %v due to a texture pack with the same UUID\n", pack.UUID)
				pk.BehaviourPacks = append(pk.BehaviourPacks[:i], pk.BehaviourPacks[i+1:]...)
			}
		}
		if !conn.hasPack(pack.UUID, pack.Version, false) {
			return fmt.Errorf("texture pack {uuid=%v, version=%v} not downloaded", pack.UUID, pack.Version)
		}
	}
	for _, pack := range pk.BehaviourPacks {
		if !conn.hasPack(pack.UUID, pack.Version, true) {
			return fmt.Errorf("behaviour pack {uuid=%v, version=%v} not downloaded", pack.UUID, pack.Version)
		}
	}
	return nil
}

// handleResourcePackDataInfo handles a resource pack data info packet, which initiates the downloading of the
// pack by the client.
func (conn *replayConnector) handleResourcePackDataInfo(pk *packet.ResourcePackDataInfo) error {
	id := strings.Split(pk.UUID, "_")[0]

	pack, ok := conn.downloadingPacks[id]
	if !ok {
		// We either already downloaded the pack or we got sent an invalid UUID, that did not match any pack
		// sent in the ResourcePacksInfo packet.
		return fmt.Errorf("unknown pack to download with UUID %v", id)
	}
	if pack.size != pk.Size {
		// Size mismatch: The ResourcePacksInfo packet had a size for the pack that did not match with the
		// size sent here.
		logrus.Printf("pack %v had a different size in the ResourcePacksInfo packet than the ResourcePackDataInfo packet\n", pk.UUID)
		pack.size = pk.Size
	}

	// Remove the resource pack from the downloading packs and add it to the awaiting packets.
	delete(conn.downloadingPacks, id)
	conn.awaitingPacks[id] = &pack

	pack.chunkSize = pk.DataChunkSize

	// The client calculates the chunk count by itself: You could in theory send a chunk count of 0 even
	// though there's data, and the client will still download normally.
	chunkCount := uint32(pk.Size / uint64(pk.DataChunkSize))
	if pk.Size%uint64(pk.DataChunkSize) != 0 {
		chunkCount++
	}

	go func() {
		for i := uint32(0); i < chunkCount; i++ {
			select {
			case <-conn.close:
				return
			case frag := <-pack.newFrag:
				// Write the fragment to the full buffer of the downloading resource pack.
				_, _ = pack.buf.Write(frag)
			}
		}
		conn.packMu.Lock()
		defer conn.packMu.Unlock()

		if pack.buf.Len() != int(pack.size) {
			logrus.Printf("incorrect resource pack size: expected %v, but got %v\n", pack.size, pack.buf.Len())
			return
		}
		// First parse the resource pack from the total byte buffer we obtained.
		newPack, err := resource.FromBytes(pack.buf.Bytes())
		if err != nil {
			logrus.Printf("invalid full resource pack data for UUID %v: %v\n", id, err)
			return
		}
		// Finally we add the resource to the resource packs slice.
		conn.resourcePacks = append(conn.resourcePacks, newPack.WithContentKey(pack.contentKey))
	}()
	return nil
}

// handleResourcePackChunkData handles a resource pack chunk data packet, which holds a fragment of a resource
// pack that is being downloaded.
func (conn *replayConnector) handleResourcePackChunkData(pk *packet.ResourcePackChunkData) error {
	pk.UUID = strings.Split(pk.UUID, "_")[0]
	pack, ok := conn.awaitingPacks[pk.UUID]
	if !ok {
		// We haven't received a ResourcePackDataInfo packet from the server, so we can't use this data to
		// download a resource pack.
		return fmt.Errorf("resource pack chunk data for resource pack that was not being downloaded")
	}
	lastData := int(pack.loaded)+len(pk.Data) == int(pack.size)
	if !lastData && uint32(len(pk.Data)) != pack.chunkSize {
		// The chunk data didn't have the full size and wasn't the last data to be sent for the resource pack,
		// meaning we got too little data.
		return fmt.Errorf("resource pack chunk data had a length of %v, but expected %v", len(pk.Data), pack.chunkSize)
	}
	if pk.ChunkIndex != pack.expectedIndex {
		return fmt.Errorf("resource pack chunk data had chunk index %v, but expected %v", pk.ChunkIndex, pack.expectedIndex)
	}
	pack.expectedIndex++
	pack.newFrag <- pk.Data
	pack.loaded += uint64(len(pk.Data))
	return nil
}
