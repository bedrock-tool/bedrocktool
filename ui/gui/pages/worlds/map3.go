// experimental 3d stuff

package worlds

import (
	"image"
	"reflect"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/gpu"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/shader"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"golang.design/x/lockfree"
)

type tile struct {
	Tex gpu.Texture
}

type uniforms struct {
	Transform [4]float32
}

type Map3 struct {
	queue    *lockfree.Queue
	mapInput mapInput

	verts         gpu.Buffer
	pipe          gpu.Pipeline
	uniformData   *uniforms
	uniforms      *gpu.UniformBuffer
	lookupTexture gpu.Texture
	lookupImage   *image.RGBA
	tiles         map[image.Point]*tile
}

func NewMap3() *Map3 {
	m := &Map3{
		queue: lockfree.NewQueue(),
		tiles: make(map[image.Point]*tile),
	}
	m.uniformData = new(uniforms)
	return m
}

func (m *Map3) Layout(gtx layout.Context) layout.Dimensions {
	m.mapInput.Layout(gtx)

	r := clip.Rect{
		Min: image.Pt(0, 0),
		Max: gtx.Constraints.Max,
	}.Push(gtx.Ops)
	paint.RenderOp{
		PrepareFunc: m.prepare,
		RenderFunc:  m.render,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	r.Pop()

	return layout.Dimensions{
		Size: gtx.Constraints.Max,
	}
}

// Slice returns a byte slice view of a slice.
func byteSlice(s interface{}) []byte {
	v := reflect.ValueOf(s)
	first := v.Index(0)
	sz := int(first.Type().Size())
	res := unsafe.Slice((*byte)(unsafe.Pointer(v.Pointer())), sz*v.Cap())
	return res[:sz*v.Len()]
}

func (m *Map3) prepare(dev gpu.Device) {
	if m.pipe == nil {
		var err error
		m.verts, err = dev.NewImmutableBuffer(2,
			byteSlice([]float32{
				-1, -1, 0, 0,
				+1, -1, 1, 0,
				-1, +1, 0, 1,

				+1, +1, 0, 0,
				-1, +1, 0, 1,
				+1, -1, 1, 0,
			}),
		)
		if err != nil {
			panic(err)
		}

		layout := gpu.VertexLayout{
			Inputs: []gpu.InputDesc{
				{Type: shader.DataTypeFloat, Size: 2, Offset: 0},
				{Type: shader.DataTypeFloat, Size: 2, Offset: 4 * 2},
			},
			Stride: 4 * 4,
		}

		vert, err := dev.NewVertexShader(shader.Sources{
			Name: "Test",
			GLSL100ES: `
			#version 310 es

			precision highp float;

			struct m3x2 {
				vec3 r0;
				vec3 r1;
			};			

			const m3x2 windowTransform = m3x2(
			#if defined(LANG_VULKAN)
				vec3(1.0, 0.0, 0.0),
				vec3(0.0, 1.0, 0.0)
			#else
				vec3(1.0, 0.0, 0.0),
				vec3(0.0, -1.0, 0.0)
			#endif
			);
			
			vec3 transform3x2(m3x2 t, vec3 v) {
				return vec3(dot(t.r0, v), dot(t.r1, v), dot(vec3(0.0, 0.0, 1.0), v));
			}
			

			struct Block
			{
				vec4 transform;
			};

			uniform Block _block;

			
			layout(location = 0) in vec2 pos;
			layout(location = 1) in vec2 uv;
			
			layout(location = 0) out vec2 vUV;
			
			void main() {
				vec2 p = pos*_block.transform.xy + _block.transform.zw;
				gl_Position = vec4(transform3x2(windowTransform, vec3(p, 0)), 1);
				vUV = uv;
			}
			`,
			Inputs: []shader.InputLocation{
				{
					Name:          "pos",
					Location:      0,
					Semantic:      "TEXCOORD",
					SemanticIndex: 0,
					Type:          0x0,
					Size:          2,
				}, {
					Name:          "uv",
					Location:      1,
					Semantic:      "TEXCOORD",
					SemanticIndex: 1,
					Type:          0x0,
					Size:          2,
				},
			},

			Uniforms: shader.UniformsReflection{
				Locations: []shader.UniformLocation{
					{
						Name:   "_block.transform",
						Type:   0x0,
						Size:   4,
						Offset: 0,
					},
				},
				Size: 16,
			},
		})
		if err != nil {
			panic(err)
		}

		frag, err := dev.NewFragmentShader(shader.Sources{
			Name: "Test",
			GLSL100ES: `
				#version 310 es
	
				precision mediump float;
	
				layout(location = 0) in highp vec2 vUV;
				layout(location = 0) out vec4 fragColor;

				layout(binding=0) uniform sampler2D lookupTex;
				layout(binding=1) uniform sampler2D mapTex;

				const vec2 mapSize = vec2(256., 256.);
				const vec2 atlasSize = vec2(1024., 512.);
				const vec2 atlasBlockSize = vec2(16., 16.);
	
				void main() {
					vec4 blockIndex = texture(mapTex, vUV);
					if(blockIndex.a < 1.) {
						return;
					}

					vec2 blockPixel = vUV * mapSize;
					vec2 off = blockPixel-floor(blockPixel);
					vec2 blockPixelUV = (off)/atlasSize*atlasBlockSize;

					vec2 blockBeginUV = vec2((blockIndex.xy * 255.0 * atlasBlockSize) / atlasSize);

					vec4 blockColor = texture(lookupTex, blockBeginUV+blockPixelUV);

					fragColor = blockColor;

					//fragColor = vec4(vUV.x, vUV.y, 0.5, 1.0);
				}
			`,
		})
		if err != nil {
			panic(err)
		}

		m.pipe, err = dev.NewPipeline(gpu.PipelineDesc{
			FragmentShader: frag,
			VertexShader:   vert,
			VertexLayout:   layout,
			PixelFormat:    0,
			Topology:       1,
			BlendDesc: gpu.BlendDesc{
				Enable:    true,
				SrcFactor: 0,
				DstFactor: 0,
			},
		})
		if err != nil {
			panic(err)
		}

		m.uniforms = gpu.NewUniformBuffer(dev, m.uniformData)
	}

	m.processQueue(dev)

	if m.lookupImage != nil && m.lookupTexture == nil {
		t, err := dev.NewTexture(2, m.lookupImage.Rect.Dx(), m.lookupImage.Rect.Dy(), 0, 0, 8)
		if err != nil {
			panic(err)
		}
		t.Upload(image.Pt(0, 0), m.lookupImage.Rect.Max, m.lookupImage.Pix, m.lookupImage.Stride)
		m.lookupTexture = t
	}
}

func (m *Map3) render(dev gpu.Device, scale, off f32.Point) {
	if m.lookupTexture == nil {
		return
	}

	dev.BindPipeline(m.pipe)
	dev.BindVertexBuffer(m.verts, 0)
	dev.BindTexture(0, m.lookupTexture)

	for _, t := range m.tiles {
		dev.BindTexture(1, t.Tex)
		dev.BindUniforms(m.uniforms.Buf)
		m.uniformData.Transform = [4]float32{scale.X, scale.Y, off.X, off.Y}
		m.uniforms.Upload()
		dev.DrawArrays(0, 6)
	}
}

func (m *Map3) Release() {
	m.pipe.Release()
	m.uniforms.Release()
	m.verts.Release()
	for p, t := range m.tiles {
		t.Tex.Release()
		delete(m.tiles, p)
	}
}

type queuedTile struct {
	ChunkPos protocol.ChunkPos
	Img      *image.RGBA
}

func (m *Map3) Update(u *messages.UpdateMap) {
	for _, cp := range u.UpdatedChunks {
		m.queue.Enqueue(&queuedTile{
			ChunkPos: cp,
			Img:      u.Chunks[cp],
		})
	}
}

func (m *Map3) SetLookupTexture(img *image.RGBA) {
	m.lookupImage = img
}

func (m *Map3) processQueue(dev gpu.Device) {
	for {
		e, ok := m.queue.Dequeue().(*queuedTile)
		if !ok {
			break
		}
		tilePos, posInTile := chunkPosToTilePos(e.ChunkPos)
		t, ok := m.tiles[tilePos]
		if !ok {
			t = &tile{}
			m.tiles[tilePos] = t
			var err error
			t.Tex, err = dev.NewTexture(2, tileSize, tileSize, 0, 0, 8)
			if err != nil {
				panic(err)
			}
		}
		t.Tex.Upload(posInTile, image.Pt(16, 16), e.Img.Pix, e.Img.Stride)
	}
}
