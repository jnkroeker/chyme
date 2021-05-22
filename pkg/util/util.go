package util

import (
	"io"
)

// CountingReader wraps an io.Reader and counts the bytes read through it.
type CountingReader struct {
	io.Reader
	BytesRead int
}

func (r *CountingReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.BytesRead += n
	return
}

// CountingReaders aggregates counts from a collection of individual readers.
type CountingReaders []*CountingReader

// Sum returns the total bytes read by all of the underlying readers.
func (cr CountingReaders) Sum() (sum int64) {
	for _, r := range cr {
		sum += int64(r.BytesRead)
	}
	return
}


// CountingWriterAt wraps an io.WriterAt and counts the bytes written through it.
type CountingWriterAt struct {
	io.WriterAt
	BytesWritten int
}

func (w *CountingWriterAt) WriteAt(p []byte, off int64) (n int, err error) {
	n, err = w.WriterAt.WriteAt(p, off)
	w.BytesWritten += n
	return
}

// CountingWriterAts aggregates counts from a collection of individual writers.
type CountingWriterAts []*CountingWriterAt

// Sum returns the total bytes written by all of the underlying readers.
func (cw CountingWriterAts) Sum() (sum int64) {
	for _, w := range cw {
		sum += int64(w.BytesWritten)
	}
	return sum
}