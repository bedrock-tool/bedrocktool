package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
	"testing"
)

type nullReader struct{}

func (nullReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}

func Benchmark_cfb8(b *testing.B) {
	cfb8 := NewCfb8(nullReader{}, make([]byte, 32))
	var buf = make([]byte, 10e6)

	for i := 0; i < b.N; i++ {
		cfb8.Read(buf)
	}
}

func Benchmark_oldCfb8(b *testing.B) {
	cfb8 := NewOldCfb8(nullReader{}, make([]byte, 32))
	var buf = make([]byte, 128000)

	for i := 0; i < b.N; i++ {
		cfb8.Read(buf)
	}
}

type oldCfb8 struct {
	r             io.Reader
	cipher        cipher.Block
	shiftRegister []byte
	iv            []byte
}

func NewOldCfb8(r io.Reader, key []byte) io.Reader {
	c := &oldCfb8{
		r: r,
	}
	c.cipher, _ = aes.NewCipher(key)
	c.shiftRegister = make([]byte, 16)
	copy(c.shiftRegister, key[:16])
	c.iv = make([]byte, 16)
	return c
}

func (c *oldCfb8) Read(dst []byte) (n int, err error) {
	n, err = c.r.Read(dst)
	if n > 0 {
		c.shiftRegister = append(c.shiftRegister, dst[:n]...)
		for off := 0; off < n; off += 1 {
			c.cipher.Encrypt(c.iv, c.shiftRegister)
			dst[off] ^= c.iv[0]
			c.shiftRegister = c.shiftRegister[1:]
		}
	}
	return
}
