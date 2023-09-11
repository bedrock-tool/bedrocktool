//go:build experimental

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
)

type Map3 struct {
	q           gpu.Buffer
	pipe        gpu.Pipeline
	uniformData *gpu.BlitUniforms
	uniforms    *gpu.UniformBuffer
}

func NewMap3() *Map3 {
	m := &Map3{}
	m.uniformData = new(gpu.BlitUniforms)
	return m
}

func (m *Map3) Layout(gtx layout.Context) layout.Dimensions {
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
	println("prepare")
	if m.pipe == nil {
		var err error
		m.q, err = dev.NewImmutableBuffer(2,
			byteSlice([]float32{
				-1, -1, 0, 0,
				+1, -1, 1, 0,
				-1, +1, 0, 1,
				+1, +1, 1, 1,
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
				vec4 uvTransformR1;
				vec4 uvTransformR2;
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
					}, {
						Name:   "_block.uvTransformR1",
						Type:   0x0,
						Size:   4,
						Offset: 16,
					}, {
						Name:   "_block.uvTransformR2",
						Type:   0x0,
						Size:   4,
						Offset: 32,
					},
				},
				Size: 48,
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
	
				void main() {
					fragColor = vec4(vUV.x, vUV.y, 0.5, 1.0);
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
}

func (m *Map3) render(dev gpu.Device, scale, off f32.Point) {
	println("render")
	dev.BindPipeline(m.pipe)
	dev.BindVertexBuffer(m.q, 0)

	off = off.Sub(f32.Pt(0, 50.0/600))

	m.uniformData.Transform = [4]float32{scale.X, scale.Y, off.X, off.Y}
	m.uniforms.Upload()
	dev.BindUniforms(m.uniforms.Buf)

	dev.DrawArrays(0, 4)
}

func (m *Map3) Release() {
	m.pipe.Release()
	m.uniforms.Release()
	m.q.Release()
}
