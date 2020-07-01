package exif

import (
	"io"
	"io/ioutil"

	log "github.com/dsoprea/go-logging"
)

type ExifScanner struct {
	r       io.ReadSeeker
	Size    int64
	Start   int64
	current int64
}

func NewExifScanner(r io.ReadSeeker, size int64) (es *ExifScanner, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Search for the beginning of the EXIF information. The EXIF is near the
	// beginning of most JPEGs, so this likely doesn't have a high cost (at
	// least, again, with JPEGs).

	// Reset io.ReadSeeker
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	es = &ExifScanner{
		r:    r,
		Size: size,
	}

	if err != nil {
		return nil, err
	}

	for {
		window, err := es.Peek(ExifSignatureLength)
		if err != nil {
			if err == io.EOF {
				return nil, ErrNoExif
			}
			return nil, err
		}

		_, err = ParseExifHeader(window)
		if err != nil {
			if log.Is(err, ErrNoExif) == true {
				// No EXIF. Move forward by one byte.

				_, err := es.Discard(1)
				if err != nil {
					return nil, err
				}
				es.Start++

				continue
			}

			// Some other error.
			if err != nil {
				return nil, err
			}
		}

		break
	}

	exifLogger.Debugf(nil, "Found EXIF blob (%d) bytes from initial position.", es.Start)

	//rawExif, err = es.ReadAll()
	//if err != nil {
	//	return nil, err
	//}

	return es, nil
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
