package exif

import (
	"io"
	"io/ioutil"
)

type ExifScanner struct {
	r       io.ReadSeeker
	Size    int64
	Start   int64
	current int64
}

func NewExifScanner(r io.ReadSeeker, size int64) (es *ExifScanner, err error) {
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return &ExifScanner{
		r:    r,
		Size: size,
	}, nil
}

func (es *ExifScanner) Read(p []byte) (n int, err error) {
	n, err = es.r.Read(p)
	es.current += int64(n)
	return n, err
}

func (es *ExifScanner) ReadAll() (b []byte, err error) {
	b, err = ioutil.ReadAll(es.r)
	es.current += int64(len(b))
	return b, err
}

func (es *ExifScanner) Peek(n int64) (b []byte, err error) {
	oldCurrent := es.current
	b = make([]byte, n)
	readN, err := es.Read(b)
	es.current += int64(readN)
	if err != nil {
		return b, err
	}
	es.current, err = es.r.Seek(oldCurrent, io.SeekStart)
	return b, err
}

func (es *ExifScanner) Discard(n int64) (int, error) {
	var err error
	oldCurrent := es.current
	es.current, err = es.r.Seek(oldCurrent+n, io.SeekStart)
	return int(es.current - oldCurrent), err
}
