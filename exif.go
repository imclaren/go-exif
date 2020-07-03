package exif

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"encoding/binary"

	log "github.com/dsoprea/go-logging"

	exifcommon "github.com/imclaren/go-exif/common"
)

const (
	// ExifAddressableAreaStart is the absolute offset in the file that all
	// offsets are relative to.
	ExifAddressableAreaStart = uint32(0x0)

	// ExifDefaultFirstIfdOffset is essentially the number of bytes in addition
	// to `ExifAddressableAreaStart` that you have to move in order to escape
	// the rest of the header and get to the earliest point where we can put
	// stuff (which has to be the first IFD). This is the size of the header
	// sequence containing the two-character byte-order, two-character fixed-
	// bytes, and the four bytes describing the first-IFD offset.
	ExifDefaultFirstIfdOffset = uint32(2 + 2 + 4)
)

const (
	// ExifSignatureLength is the number of bytes in the EXIF signature (which
	// customarily includes the first IFD offset).
	ExifSignatureLength = 8
)

var (
	exifLogger = log.NewLogger("exif.exif")

	ExifBigEndianSignature    = [4]byte{'M', 'M', 0x00, 0x2a}
	ExifLittleEndianSignature = [4]byte{'I', 'I', 0x2a, 0x00}
)

var (
	ErrNoExif          = errors.New("no exif data")
	ErrExifHeaderError = errors.New("exif header error")
)

// SearchAndExtractExif searches for an EXIF blob in the byte-slice.
func SearchAndExtractExif(data []byte) (rawExif []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	r := bytes.NewReader(data)
	rawExif, err = SearchAndExtractExifWithReadSeeker(r, int64(len(data)))
	if err != nil {
		if err == ErrNoExif {
			return nil, err
		}

		log.Panic(err)
	}

	return rawExif, nil
}

// SearchAndExtractExifWithReadSeeker searches for an EXIF blob using an
// `io.ReadSeeker`.
func SearchAndExtractExifWithReadSeeker(r io.ReadSeeker, size int64) (rawExif []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Search for the beginning of the EXIF information. The EXIF is near the
	// beginning of most JPEGs, so this likely doesn't have a high cost (at
	// least, again, with JPEGs).

	s, err := NewScanner(r, size)
	log.PanicIf(err)

	exifLogger.Debugf(nil, "Found EXIF blob (%d) bytes from initial position.", s.Start)

	rawExif, err = s.ReadAll()
	log.PanicIf(err)

	return rawExif, nil
}

// SearchFileAndExtractExif returns a slice from the beginning of the EXIF data
// to the end of the file (it's not practical to try and calculate where the
// data actually ends).
func SearchFileAndExtractExif(filepath string) (rawExif []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Open the file.

	f, err := os.Open(filepath)
	log.PanicIf(err)

	defer f.Close()

	fi, err := f.Stat()
	log.PanicIf(err)

	rawExif, err = SearchAndExtractExifWithReadSeeker(f, fi.Size())
	log.PanicIf(err)

	return rawExif, nil
}

type ExifHeader struct {
	ByteOrder      binary.ByteOrder
	FirstIfdOffset uint32
}

func (eh ExifHeader) String() string {
	return fmt.Sprintf("ExifHeader<BYTE-ORDER=[%v] FIRST-IFD-OFFSET=(0x%02x)>", eh.ByteOrder, eh.FirstIfdOffset)
}

// ParseExifHeader parses the bytes at the very top of the header.
//
// This will panic with ErrNoExif on any data errors so that we can double as
// an EXIF-detection routine.
func ParseExifHeader(data []byte) (eh ExifHeader, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Good reference:
	//
	//      CIPA DC-008-2016; JEITA CP-3451D
	//      -> http://www.cipa.jp/std/documents/e/DC-008-Translation-2016-E.pdf

	if len(data) < ExifSignatureLength {
		exifLogger.Warningf(nil, "Not enough data for EXIF header: (%d)", len(data))
		return eh, ErrNoExif
	}

	if bytes.Equal(data[:4], ExifBigEndianSignature[:]) == true {
		eh.ByteOrder = binary.BigEndian
	} else if bytes.Equal(data[:4], ExifLittleEndianSignature[:]) == true {
		eh.ByteOrder = binary.LittleEndian
	} else {
		return eh, ErrNoExif
	}

	eh.FirstIfdOffset = eh.ByteOrder.Uint32(data[4:8])

	return eh, nil
}

// Visit recursively invokes a callback for every tag.
func Visit(s *Scanner, rootIfdIdentity *exifcommon.IfdIdentity, ifdMapping *exifcommon.IfdMapping, tagIndex *TagIndex, visitor TagVisitorFn) (eh ExifHeader, furthestOffset uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	window, err := s.Peek(ExifSignatureLength)
	log.PanicIf(err)

	eh, err = ParseExifHeader(window)
	log.PanicIf(err)

	ie := NewIfdEnumerate(s, ifdMapping, tagIndex, eh.ByteOrder)

	_, err = ie.Scan(rootIfdIdentity, eh.FirstIfdOffset, visitor)
	log.PanicIf(err)

	furthestOffset = ie.FurthestOffset()

	return eh, furthestOffset, nil
}

// Collect recursively builds a static structure of all IFDs and tags.
func Collect(s *Scanner, ifdMapping *exifcommon.IfdMapping, tagIndex *TagIndex) (eh ExifHeader, index IfdIndex, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	window, err := s.Peek(ExifSignatureLength)
	log.PanicIf(err)

	eh, err = ParseExifHeader(window)
	log.PanicIf(err)

	ie := NewIfdEnumerate(s, ifdMapping, tagIndex, eh.ByteOrder)

	index, err = ie.Collect(eh.FirstIfdOffset)
	log.PanicIf(err)

	return eh, index, nil
}

// BuildExifHeader constructs the bytes that go at the front of the stream.
func BuildExifHeader(byteOrder binary.ByteOrder, firstIfdOffset uint32) (headerBytes []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	b := new(bytes.Buffer)

	var signatureBytes []byte
	if byteOrder == binary.BigEndian {
		signatureBytes = ExifBigEndianSignature[:]
	} else {
		signatureBytes = ExifLittleEndianSignature[:]
	}

	_, err = b.Write(signatureBytes)
	log.PanicIf(err)

	err = binary.Write(b, byteOrder, firstIfdOffset)
	log.PanicIf(err)

	return b.Bytes(), nil
}
