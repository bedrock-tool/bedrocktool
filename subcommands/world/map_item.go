package world

import (
	"bytes"
	"image"
	"image/draw"
	"math"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"golang.design/x/lockfree"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/image/bmp"
)

const ViewMapID = 0x424242

// MapItemPacket tells the client that it has a map with id 0x424242 in the offhand
var MapItemPacket packet.InventoryContent = packet.InventoryContent{
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

func (m *MapUI) getBounds() (min, max protocol.ChunkPos) {
	// get the chunk coord bounds
	for _ch := range m.renderedChunks {
		if _ch.X() < min.X() {
			min[0] = _ch.X()
		}
		if _ch.Z() < min.Z() {
			min[1] = _ch.Z()
		}
		if _ch.X() > max.X() {
			max[0] = _ch.X()
		}
		if _ch.Z() > max.Z() {
			max[1] = _ch.Z()
		}
	}
	return
}

type RenderElem struct {
	pos protocol.ChunkPos
	ch  *chunk.Chunk
}

type MapUI struct {
	img            *image.RGBA // rendered image
	zoomLevel      int         // pixels per chunk
	renderQueue    *lockfree.Queue
	renderedChunks map[protocol.ChunkPos]*image.RGBA // prerendered chunks
	needRedraw     bool                              // when the map has updated this is true

	ticker *time.Ticker
	w      *WorldState
}

func NewMapUI(w *WorldState) *MapUI {
	m := &MapUI{
		img:            image.NewRGBA(image.Rect(0, 0, 128, 128)),
		zoomLevel:      16,
		renderQueue:    lockfree.NewQueue(),
		renderedChunks: make(map[protocol.ChunkPos]*image.RGBA),
		needRedraw:     true,
		w:              w,
	}
	return m
}

func (m *MapUI) Start() {
	// init map
	if m.w.proxy.Client != nil {
		if err := m.w.proxy.Client.WritePacket(&packet.ClientBoundMapItemData{
			MapID:          ViewMapID,
			Scale:          4,
			MapsIncludedIn: []int64{ViewMapID},
			Width:          0,
			Height:         0,
			Pixels:         nil,
			UpdateFlags:    packet.MapUpdateFlagInitialisation,
		}); err != nil {
			logrus.Error(err)
			return
		}
	}

	m.ticker = time.NewTicker(33 * time.Millisecond)
	go func() {
		for range m.ticker.C {
			if m.needRedraw {
				m.needRedraw = false
				m.Redraw()

				if m.w.proxy.Client != nil {
					if err := m.w.proxy.Client.WritePacket(&packet.ClientBoundMapItemData{
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
		}
	}()
	go func() { // send map item
		t := time.NewTicker(1 * time.Second)
		for range t.C {
			if m.w.ctx.Err() != nil {
				return
			}
			if m.w.proxy.Client != nil {
				err := m.w.proxy.Client.WritePacket(&MapItemPacket)
				if err != nil {
					logrus.Error(err)
					return
				}
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
	m.renderedChunks = make(map[protocol.ChunkPos]*image.RGBA)
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

// Redraw draws chunk images to the map image
func (m *MapUI) Redraw() {
	for {
		r, ok := m.renderQueue.Dequeue().(*RenderElem)
		if !ok {
			break
		}
		if r.ch != nil {
			m.renderedChunks[r.pos] = Chunk2Img(r.ch)
		} else {
			m.renderedChunks[r.pos] = black16x16
		}
	}

	middle := protocol.ChunkPos{
		int32(m.w.PlayerPos.Position.X()),
		int32(m.w.PlayerPos.Position.Z()),
	}

	// total_width := 32 * math.Ceil(float64(chunks_x)/32)
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

	drawFull := false

	if drawFull {
		img2 := m.ToImage()
		buf := bytes.NewBuffer(nil)
		bmp.Encode(buf, img2)
		os.WriteFile("test.bmp", buf.Bytes(), 0o777)
	}
}

func (m *MapUI) ToImage() *image.RGBA {
	// get the chunk coord bounds
	min, max := m.getBounds()
	chunksX := int(max[0] - min[0] + 1) // how many chunk lengths is x coordinate
	chunksY := int(max[1] - min[1] + 1)

	img2 := image.NewRGBA(image.Rect(0, 0, chunksX*16, chunksY*16))

	middleBlockX := chunksX / 2 * 16
	middleBlockY := chunksY / 2 * 16

	for pos := range m.renderedChunks {
		px := image.Point{
			X: int(pos.X()*16) - middleBlockX + img2.Rect.Dx(),
			Y: int(pos.Z()*16) - middleBlockY + img2.Rect.Dy(),
		}
		draw.Draw(img2, image.Rect(
			px.X,
			px.Y,
			px.X+16,
			px.Y+16,
		), m.renderedChunks[pos], image.Point{}, draw.Src)
	}
	return img2
}

func (m *MapUI) SetChunk(pos protocol.ChunkPos, ch *chunk.Chunk) {
	m.renderQueue.Enqueue(&RenderElem{pos, ch})
	m.SchedRedraw()
}

func (w *WorldState) ProcessAnimate(pk *packet.Animate) {
	if pk.ActionType == packet.AnimateActionSwingArm {
		w.ui.ChangeZoom()
		w.proxy.SendPopup(locale.Loc("zoom_level", locale.Strmap{"Level": w.ui.zoomLevel}))
	}
}

func (w *WorldState) processMapPacketsClient(pk packet.Packet, forward *bool) packet.Packet {
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		w.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
	case *packet.PlayerAuthInput:
		w.SetPlayerPos(pk.Position, pk.Pitch, pk.Yaw, pk.HeadYaw)
	case *packet.MapInfoRequest:
		if pk.MapID == ViewMapID {
			w.ui.SchedRedraw()
			*forward = false
		}
	case *packet.Animate:
		w.ProcessAnimate(pk)
	}
	return pk
}
