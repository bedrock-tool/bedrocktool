package utils

import (
	"io"
	"sync"

	"github.com/klauspost/compress/flate"
)

type closePutback struct {
	*flate.Writer
}

func (c closePutback) Close() error {
	deflate.ReturnWriter(c.Writer)
	return nil
}

type DeflatePool struct {
	pool sync.Pool
}

var deflate DeflatePool

func (pool *DeflatePool) GetWriter(dst io.Writer) (writer *flate.Writer) {
	if w := pool.pool.Get(); w != nil {
		writer = w.(*flate.Writer)
		writer.Reset(dst)
	} else {
		writer, _ = flate.NewWriter(dst, flate.HuffmanOnly)
	}
	return writer
}

func (pool *DeflatePool) ReturnWriter(writer *flate.Writer) {
	_ = writer.Close()
	pool.pool.Put(writer)
}
