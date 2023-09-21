package worlds

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/go-gl/mathgl/mgl32"
	"golang.design/x/lockfree"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

const ViewMapID = 0x424242

// MapItemPacket tells the client that it has a map with id 0x424242 in the offhand
var MapItemPacket = packet.InventoryContent{
	WindowID: 119,
	Content: []protocol.ItemInstance{
		{
			StackNetworkID: 1, // random if auth inv
			Stack: protocol.ItemStack{
				ItemType: protocol.ItemType{
					NetworkID:     420, // overwritten in onconnect
					MetadataValue: 0,
				},
				BlockRuntimeID: 0,
				Count:          1,
				NBTData: map[string]interface{}{
					"map_name_index": int64(1),
					"map_uuid":       int64(ViewMapID),
				},
			},
		},
	},
}

func (m *MapUI) GetBounds() (Min, Max protocol.ChunkPos) {
	if len(m.renderedChunks) == 0 {
		return
	}
	Min = protocol.ChunkPos{math.MaxInt32, math.MaxInt32}
	for chunk := range m.renderedChunks {
		Min[0] = min(Min[0], chunk[0])
		Min[1] = min(Min[1], chunk[1])
		Max[0] = max(Max[0], chunk[0])
		Max[1] = max(Max[1], chunk[1])
	}
	return
}

type renderElem struct {
	ch  *chunk.Chunk
	pos protocol.ChunkPos

	isDeferredState bool
}

type MapUI struct {
	img            *image.RGBA // rendered image
	renderQueue    *lockfree.Queue
	renderedChunks map[protocol.ChunkPos]*image.RGBA // prerendered chunks
	oldRendered    map[protocol.ChunkPos]*image.RGBA
	ticker         *time.Ticker
	w              *worldsHandler

	l  sync.Mutex
	wg sync.WaitGroup

	zoomLevel  int  // pixels per chunk
	needRedraw bool // when the map has updated this is true
	showOnGui  bool
}

func NewMapUI(w *worldsHandler) *MapUI {
	m := &MapUI{
		img:            image.NewRGBA(image.Rect(0, 0, 128, 128)),
		zoomLevel:      16,
		renderQueue:    lockfree.NewQueue(),
		renderedChunks: make(map[protocol.ChunkPos]*image.RGBA),
		oldRendered:    make(map[protocol.ChunkPos]*image.RGBA),
		needRedraw:     true,
		w:              w,
	}
	return m
}

func (m *MapUI) Start(ctx context.Context) {
	r := m.w.ui.Message(messages.CanShowImages{})
	if r.Ok {
		m.showOnGui = true
	}

	// init map
	err := m.w.proxy.ClientWritePacket(&packet.ClientBoundMapItemData{
		MapID:          ViewMapID,
		Scale:          4,
		MapsIncludedIn: []int64{ViewMapID},
		UpdateFlags:    packet.MapUpdateFlagInitialisation,
	})
	if err != nil {
		logrus.Error(err)
		return
	}

	m.ticker = time.NewTicker(33 * time.Millisecond)
	m.wg.Add(1)
	go func() {
		lookup, _ := utils.ResolveColors(m.w.customBlocks, m.w.serverState.packs, true)
		m.w.ui.Message(messages.MapLookup{Lookup: lookup})
		m.wg.Done()
	}()
	go func() {
		var oldPos mgl32.Vec3
		for range m.ticker.C {
			if ctx.Err() != nil {
				return
			}
			newPos := m.w.proxy.Player.Position
			if int(oldPos.X()) != int(newPos.X()) || int(oldPos.Z()) != int(newPos.Z()) {
				m.needRedraw = true
				oldPos = newPos
			}

			if m.needRedraw {
				m.needRedraw = false
				m.Redraw()

				if err := m.w.proxy.ClientWritePacket(&packet.ClientBoundMapItemData{
					MapID:       ViewMapID,
					Scale:       4,
					Width:       128,
					Height:      128,
					Pixels:      utils.Img2rgba(m.img),
					UpdateFlags: packet.MapUpdateFlagTexture,
				}); err != nil {
					logrus.Error(err)
					return
				}
			}
		}
	}()
	go func() { // send map item
		t := time.NewTicker(1 * time.Second)
		for range t.C {
			if m.w.proxy.Client == nil {
				return
			}
			err := m.w.proxy.ClientWritePacket(&MapItemPacket)
			if err != nil {
				logrus.Error(err)
				return
			}
		}
	}()
}

func (m *MapUI) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
}

// Reset resets the map to inital state
func (m *MapUI) Reset() {
	m.l.Lock()
	m.renderedChunks = make(map[protocol.ChunkPos]*image.RGBA)
	m.oldRendered = make(map[protocol.ChunkPos]*image.RGBA)
	m.w.ui.Message(messages.UpdateMap{
		ChunkCount: -1,
	})
	m.l.Unlock()
	m.SchedRedraw()
}

// ChangeZoom adds to the zoom value and goes around to 32 once it hits 128
func (m *MapUI) ChangeZoom() {
	m.zoomLevel /= 2
	if m.zoomLevel == 0 {
		m.zoomLevel = 16
	}
	m.SchedRedraw()
}

// SchedRedraw tells the map to redraw the next time its sent
func (m *MapUI) SchedRedraw() {
	m.needRedraw = true
}

var red = image.NewUniform(color.RGBA{R: 0xff, G: 0, B: 0, A: 128})

func (m *MapUI) processQueue() []protocol.ChunkPos {
	m.wg.Wait()
	updatedChunks := make([]protocol.ChunkPos, 0, m.renderQueue.Length())
	for {
		r, ok := m.renderQueue.Dequeue().(*renderElem)
		if !ok {
			break
		}
		if r.ch != nil {
			img := utils.Chunk2Img(r.ch)
			if r.isDeferredState {
				if old, ok := m.renderedChunks[r.pos]; ok {
					m.oldRendered[r.pos] = old
				}
				draw.Draw(img, img.Rect, red, image.Point{}, draw.Over)
			}

			img2 := image.NewRGBA(img.Rect)
			draw.Draw(img2, img.Rect, img, image.Pt(0, 0), draw.Over)
			m.renderedChunks[r.pos] = img2
			updatedChunks = append(updatedChunks, r.pos)
		} else {
			if img, ok := m.oldRendered[r.pos]; ok {
				m.renderedChunks[r.pos] = img
			} else {
				delete(m.renderedChunks, r.pos)
			}
		}
	}
	return updatedChunks
}

// Redraw draws chunk images to the map image
func (m *MapUI) Redraw() {
	m.l.Lock()
	defer m.l.Unlock()
	updatedChunks := m.processQueue()

	// draw ingame map
	middle := protocol.ChunkPos{
		int32(m.w.proxy.Player.Position.X()),
		int32(m.w.proxy.Player.Position.Z()),
	}
	chunksPerLine := float64(128 / m.zoomLevel)
	pxPerBlock := 128 / chunksPerLine / 16 // how many pixels per block
	pxSizeChunk := int(math.Floor(pxPerBlock * 16))

	for i := 0; i < len(m.img.Pix); i++ { // clear canvas
		m.img.Pix[i] = 0
	}
	for _ch := range m.renderedChunks {
		relativeMiddleX := float64(_ch.X()*16 - middle.X())
		relativeMiddleZ := float64(_ch.Z()*16 - middle.Z())
		px := image.Point{ // bottom left corner of the chunk on the map
			X: int(math.Floor(relativeMiddleX*pxPerBlock)) + 64,
			Y: int(math.Floor(relativeMiddleZ*pxPerBlock)) + 64,
		}

		if !m.img.Rect.Intersect(image.Rect(px.X, px.Y, px.X+pxSizeChunk, px.Y+pxSizeChunk)).Empty() {
			utils.DrawImgScaledPos(m.img, m.renderedChunks[_ch], px, pxSizeChunk)
		}
	}

	// send tiles to gui map
	if m.showOnGui {
		m.w.ui.Message(messages.UpdateMap{
			ChunkCount:    len(m.renderedChunks),
			Rotation:      m.w.proxy.Player.Yaw,
			UpdatedChunks: updatedChunks,
			Chunks:        m.renderedChunks,
		})
	}
}

func (m *MapUI) ToImage() *image.RGBA {
	m.l.Lock()
	defer m.l.Unlock()
	m.processQueue()
	// get the chunk coord bounds
	min, max := m.GetBounds()
	chunksX := int(max[0] - min[0] + 1) // how many chunk lengths is x coordinate
	chunksY := int(max[1] - min[1] + 1)

	img := image.NewRGBA(image.Rect(0, 0, chunksX*16, chunksY*16))

	for pos, tile := range m.renderedChunks {
		px := image.Pt(
			int((pos.X()-min.X())*16),
			int((pos.Z()-min.Z())*16),
		)
		draw.Draw(img, image.Rect(
			px.X, px.Y,
			px.X+16, px.Y+16,
		), tile, image.Point{}, draw.Src)
	}
	return img
}

func (m *MapUI) SetChunk(pos world.ChunkPos, ch *chunk.Chunk, isDeferredState bool) {
	m.renderQueue.Enqueue(&renderElem{
		ch:              ch,
		pos:             (protocol.ChunkPos)(pos),
		isDeferredState: isDeferredState,
	})
	m.SchedRedraw()
}
