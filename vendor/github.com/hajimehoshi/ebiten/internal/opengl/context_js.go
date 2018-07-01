// Copyright 2014 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build js

package opengl

import (
	"errors"
	"fmt"

	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/webgl"

	"github.com/hajimehoshi/ebiten/internal/web"
)

// Note that `type Texture *js.Object` doesn't work.
// There is no way to get the internal object in that case.

type (
	Texture         interface{}
	Framebuffer     interface{}
	Shader          interface{}
	Program         interface{}
	Buffer          interface{}
	uniformLocation interface{}
)

type attribLocation int

type programID int

var InvalidTexture = Texture((*js.Object)(nil))

func getProgramID(p Program) programID {
	return programID(p.(*js.Object).Get("__ebiten_programId").Int())
}

func init() {
	// Accessing the prototype is rquired on Safari.
	c := js.Global.Get("WebGLRenderingContext").Get("prototype")
	VertexShader = ShaderType(c.Get("VERTEX_SHADER").Int())
	FragmentShader = ShaderType(c.Get("FRAGMENT_SHADER").Int())
	ArrayBuffer = BufferType(c.Get("ARRAY_BUFFER").Int())
	ElementArrayBuffer = BufferType(c.Get("ELEMENT_ARRAY_BUFFER").Int())
	DynamicDraw = BufferUsage(c.Get("DYNAMIC_DRAW").Int())
	StaticDraw = BufferUsage(c.Get("STATIC_DRAW").Int())
	Triangles = Mode(c.Get("TRIANGLES").Int())
	Lines = Mode(c.Get("LINES").Int())
	Short = DataType(c.Get("SHORT").Int())
	Float = DataType(c.Get("FLOAT").Int())

	zero = operation(c.Get("ZERO").Int())
	one = operation(c.Get("ONE").Int())
	srcAlpha = operation(c.Get("SRC_ALPHA").Int())
	dstAlpha = operation(c.Get("DST_ALPHA").Int())
	oneMinusSrcAlpha = operation(c.Get("ONE_MINUS_SRC_ALPHA").Int())
	oneMinusDstAlpha = operation(c.Get("ONE_MINUS_DST_ALPHA").Int())
}

type context struct {
	gl            *webgl.Context
	loseContext   *js.Object
	lastProgramID programID
}

func Init() error {
	if web.IsNodeJS() {
		return fmt.Errorf("opengl: Node.js is not supported")
	}

	// TODO: Define id?
	canvas := js.Global.Get("document").Call("querySelector", "canvas")
	gl, err := webgl.NewContext(canvas, &webgl.ContextAttributes{
		Alpha:              true,
		PremultipliedAlpha: true,
	})
	if err != nil {
		return err
	}
	c := &Context{}
	c.gl = gl

	// Getting an extension might fail after the context is lost, so
	// it is required to get the extension here.
	c.loseContext = gl.GetExtension("WEBGL_lose_context")
	if c.loseContext != nil {
		// This testing function name is temporary.
		js.Global.Set("_ebiten_loseContextForTesting", func() {
			c.loseContext.Call("loseContext")
		})
	}
	theContext = c
	return nil
}

func (c *Context) Reset() error {
	c.locationCache = newLocationCache()
	c.lastTexture = nil
	c.lastFramebuffer = nil
	c.lastViewportWidth = 0
	c.lastViewportHeight = 0
	c.lastCompositeMode = CompositeModeUnknown
	gl := c.gl
	gl.Enable(gl.BLEND)
	c.BlendFunc(CompositeModeSourceOver)
	f := gl.GetParameter(gl.FRAMEBUFFER_BINDING)
	c.screenFramebuffer = f
	return nil
}

func (c *Context) BlendFunc(mode CompositeMode) {
	if c.lastCompositeMode == mode {
		return
	}
	c.lastCompositeMode = mode
	s, d := operations(mode)
	gl := c.gl
	gl.BlendFunc(int(s), int(d))
}

func (c *Context) NewTexture(width, height int) (Texture, error) {
	gl := c.gl
	t := gl.CreateTexture()
	if t == nil {
		return nil, errors.New("opengl: glGenTexture failed")
	}
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	c.BindTexture(t)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// void texImage2D(GLenum target, GLint level, GLenum internalformat,
	//     GLsizei width, GLsizei height, GLint border, GLenum format,
	//     GLenum type, ArrayBufferView? pixels);
	gl.Call("texImage2D", gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)

	return t, nil
}

func (c *Context) bindFramebufferImpl(f Framebuffer) {
	gl := c.gl
	gl.BindFramebuffer(gl.FRAMEBUFFER, f.(*js.Object))
}

func (c *Context) FramebufferPixels(f Framebuffer, width, height int) ([]byte, error) {
	gl := c.gl

	c.bindFramebuffer(f)

	pixels := js.Global.Get("Uint8Array").New(4 * width * height)
	gl.ReadPixels(0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, pixels)
	if e := gl.GetError(); e != gl.NO_ERROR {
		return nil, errors.New(fmt.Sprintf("opengl: error: %d", e))
	}
	return pixels.Interface().([]byte), nil
}

func (c *Context) bindTextureImpl(t Texture) {
	gl := c.gl
	gl.BindTexture(gl.TEXTURE_2D, t.(*js.Object))
}

func (c *Context) DeleteTexture(t Texture) {
	gl := c.gl
	if !gl.IsTexture(t.(*js.Object)) {
		return
	}
	if c.lastTexture == t {
		c.lastTexture = nil
	}
	gl.DeleteTexture(t.(*js.Object))
}

func (c *Context) IsTexture(t Texture) bool {
	gl := c.gl
	b := gl.IsTexture(t.(*js.Object))
	return b
}

func (c *Context) TexSubImage2D(p []byte, x, y, width, height int) {
	gl := c.gl
	// void texSubImage2D(GLenum target, GLint level, GLint xoffset, GLint yoffset,
	//                    GLsizei width, GLsizei height,
	//                    GLenum format, GLenum type, ArrayBufferView? pixels);
	gl.Call("texSubImage2D", gl.TEXTURE_2D, 0, x, y, width, height, gl.RGBA, gl.UNSIGNED_BYTE, p)
}

func (c *Context) NewFramebuffer(t Texture) (Framebuffer, error) {
	gl := c.gl
	f := gl.CreateFramebuffer()
	c.bindFramebuffer(f)

	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, t.(*js.Object), 0)
	s := gl.CheckFramebufferStatus(gl.FRAMEBUFFER)
	if s != gl.FRAMEBUFFER_COMPLETE {
		return nil, errors.New(fmt.Sprintf("opengl: creating framebuffer failed: %d", s))
	}

	return f, nil
}

func (c *Context) setViewportImpl(width, height int) {
	gl := c.gl
	gl.Viewport(0, 0, width, height)
}

func (c *Context) DeleteFramebuffer(f Framebuffer) {
	gl := c.gl
	if !gl.IsFramebuffer(f.(*js.Object)) {
		return
	}
	// If a framebuffer to be deleted is bound, a newly bound framebuffer
	// will be a default framebuffer.
	// https://www.khronos.org/opengles/sdk/docs/man/xhtml/glDeleteFramebuffers.xml
	if c.lastFramebuffer == f {
		c.lastFramebuffer = nil
		c.lastViewportWidth = 0
		c.lastViewportHeight = 0
	}
	gl.DeleteFramebuffer(f.(*js.Object))
}

func (c *Context) NewShader(shaderType ShaderType, source string) (Shader, error) {
	gl := c.gl
	s := gl.CreateShader(int(shaderType))
	if s == nil {
		return nil, fmt.Errorf("opengl: glCreateShader failed: shader type: %d", shaderType)
	}

	gl.ShaderSource(s, source)
	gl.CompileShader(s)

	if !gl.GetShaderParameterb(s, gl.COMPILE_STATUS) {
		log := gl.GetShaderInfoLog(s)
		return nil, fmt.Errorf("opengl: shader compile failed: %s", log)
	}
	return s, nil
}

func (c *Context) DeleteShader(s Shader) {
	gl := c.gl
	gl.DeleteShader(s.(*js.Object))
}

func (c *Context) NewProgram(shaders []Shader) (Program, error) {
	gl := c.gl
	p := gl.CreateProgram()
	if p == nil {
		return nil, errors.New("opengl: glCreateProgram failed")
	}
	p.Set("__ebiten_programId", c.lastProgramID)
	c.lastProgramID++

	for _, shader := range shaders {
		gl.AttachShader(p, shader.(*js.Object))
	}
	gl.LinkProgram(p)
	if !gl.GetProgramParameterb(p, gl.LINK_STATUS) {
		return nil, errors.New("opengl: program error")
	}
	return p, nil
}

func (c *Context) UseProgram(p Program) {
	gl := c.gl
	gl.UseProgram(p.(*js.Object))
}

func (c *Context) DeleteProgram(p Program) {
	gl := c.gl
	if !gl.IsProgram(p.(*js.Object)) {
		return
	}
	gl.DeleteProgram(p.(*js.Object))
}

func (c *Context) getUniformLocationImpl(p Program, location string) uniformLocation {
	gl := c.gl
	return gl.GetUniformLocation(p.(*js.Object), location)
}

func (c *Context) UniformInt(p Program, location string, v int) {
	gl := c.gl
	l := c.locationCache.GetUniformLocation(c, p, location)
	gl.Uniform1i(l.(*js.Object), v)
}

func (c *Context) UniformFloat(p Program, location string, v float32) {
	gl := c.gl
	l := c.locationCache.GetUniformLocation(c, p, location)
	gl.Uniform1f(l.(*js.Object), v)
}

func (c *Context) UniformFloats(p Program, location string, v []float32) {
	gl := c.gl
	l := c.locationCache.GetUniformLocation(c, p, location)
	switch len(v) {
	case 2:
		gl.Call("uniform2fv", l.(*js.Object), v)
	case 4:
		gl.Call("uniform4fv", l.(*js.Object), v)
	case 16:
		gl.UniformMatrix4fv(l.(*js.Object), false, v)
	default:
		panic("not reached")
	}
}

func (c *Context) getAttribLocationImpl(p Program, location string) attribLocation {
	gl := c.gl
	return attribLocation(gl.GetAttribLocation(p.(*js.Object), location))
}

func (c *Context) VertexAttribPointer(p Program, location string, size int, dataType DataType, stride int, offset int) {
	gl := c.gl
	l := c.locationCache.GetAttribLocation(c, p, location)
	gl.VertexAttribPointer(int(l), size, int(dataType), false, stride, offset)
}

func (c *Context) EnableVertexAttribArray(p Program, location string) {
	gl := c.gl
	l := c.locationCache.GetAttribLocation(c, p, location)
	gl.EnableVertexAttribArray(int(l))
}

func (c *Context) DisableVertexAttribArray(p Program, location string) {
	gl := c.gl
	l := c.locationCache.GetAttribLocation(c, p, location)
	gl.DisableVertexAttribArray(int(l))
}

func (c *Context) NewArrayBuffer(size int) Buffer {
	gl := c.gl
	b := gl.CreateBuffer()
	gl.BindBuffer(int(ArrayBuffer), b)
	gl.BufferData(int(ArrayBuffer), size, int(DynamicDraw))
	return b
}

func (c *Context) NewElementArrayBuffer(indices []uint16) Buffer {
	gl := c.gl
	b := gl.CreateBuffer()
	gl.BindBuffer(int(ElementArrayBuffer), b)
	gl.BufferData(int(ElementArrayBuffer), indices, int(StaticDraw))
	return b
}

func (c *Context) BindElementArrayBuffer(b Buffer) {
	gl := c.gl
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, b.(*js.Object))
}

func (c *Context) BufferSubData(bufferType BufferType, data []float32) {
	gl := c.gl
	gl.BufferSubData(int(bufferType), 0, data)
}

func (c *Context) DeleteBuffer(b Buffer) {
	gl := c.gl
	gl.DeleteBuffer(b.(*js.Object))
}

func (c *Context) DrawElements(mode Mode, len int, offsetInBytes int) {
	gl := c.gl
	gl.DrawElements(int(mode), len, gl.UNSIGNED_SHORT, offsetInBytes)
}

func (c *Context) maxTextureSizeImpl() int {
	gl := c.gl
	return gl.GetParameter(gl.MAX_TEXTURE_SIZE).Int()
}

func (c *Context) Flush() {
	gl := c.gl
	gl.Flush()
}

func (c *Context) IsContextLost() bool {
	gl := c.gl
	return gl.IsContextLost()
}

func (c *Context) RestoreContext() {
	if c.loseContext != nil {
		c.loseContext.Call("restoreContext")
	}
}
