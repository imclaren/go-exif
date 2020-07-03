package exif

import (
	"bytes"
	"io"
	"io/ioutil"

	log "github.com/dsoprea/go-logging"
	exifcommon "github.com/imclaren/go-exif/common"
	exifundefined "github.com/imclaren/go-exif/undefined"
)

const (
	// DefaultScanLimit is the default scan limit of 1MB. Note that Exif metadata
	// is restricted in size to 64 kB in JPEG images because according to the JPEG
	// specification this information must be contained within a single JPEG APP1
	// segment.
	DefaultScanLimit = (1 << 10) << 10 // 1 MB
)

// Scanner is the Scanner struct
type Scanner struct {
	r         io.ReadSeeker
	scanLimit int64
	Size      int64
	Start     int64
	Current   int64
}

// NewScanner creates a new Scanner.
// The variables are an io.ReadSeeker and the size of the bytes in the io.ReadSeeker.
// NewScanner reads the default scan limit of 1MB into memory. Note that Exif metadata
// is restricted in size to 64 kB in JPEG images because according to the JPEG specification
// this information must be contained within a single JPEG APP1 segment.
func NewScanner(r io.ReadSeeker, size int64) (s *Scanner, err error) {
	return NewScannerLimit(r, size, DefaultScanLimit)
}

// NewScannerNoLimit creates a new Scanner with no scan size limit.
// The variables are an io.ReadSeeker and the size of the bytes in the io.ReadSeeker.
// All the contents of the io.Readseeker from the start of the exif block (if any)
// will be held in memory.
func NewScannerNoLimit(r io.ReadSeeker, size int64) (s *Scanner, err error) {
	return NewScannerLimit(r, size, 0)
}

// NewScannerLimit creates a new Scanner.
// The variables are an io.ReadSeeker, the size of the bytes in the io.ReadSeeker,
// and the scan limit.
func NewScannerLimit(r io.ReadSeeker, size, scanLimit int64) (s *Scanner, err error) {
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

	// Scan for exif
	s = &Scanner{
		r:         r,
		Size:      size,
		scanLimit: scanLimit,
	}
	for {
		window, err := s.Peek(ExifSignatureLength)
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

				_, err := s.Discard(1)
				if err != nil {
					return nil, err
				}
				s.Start++

				continue
			}

			// Some other error.
			if err != nil {
				return nil, err
			}
		}

		break
	}

	exifLogger.Debugf(nil, "Found EXIF blob (%d) bytes from initial position.", s.Start)

	return s, nil
}

// NewScannerLimitFromBytes creates a new Scanner.
// The variables are the bytes and the scan limit.
func NewScannerLimitFromBytes(b []byte, scanLimit int64) (s *Scanner, err error) {
	r := bytes.NewReader(b)
	return NewScannerLimit(r, int64(len(b)), scanLimit)
}

// Remaining returns the number of bytes remaining in the
func (s *Scanner) Remaining() int64 {
	return s.Size - s.Current
}

// Read reads into p []byte
func (s *Scanner) Read(p []byte) (n int, err error) {
	n, err = s.r.Read(p)
	s.Current += int64(n)
	return n, err
}

// ReadAll reads all the remaining bytes
func (s *Scanner) ReadAll() (b []byte, err error) {
	b, err = ioutil.ReadAll(s.r)
	s.Current += int64(len(b))
	return b, err
}

// Peek reads n bytes
func (s *Scanner) Peek(n int64) (b []byte, err error) {
	if n == 0 {
		return s.peekAll()
	}
	oldCurrent := s.Current
	b = make([]byte, n)
	readN, err := s.Read(b)
	s.Current += int64(readN)
	if err != nil {
		return b, err
	}
	s.Current, err = s.r.Seek(oldCurrent, io.SeekStart)
	return b, err
}

func (s *Scanner) peekAll() (b []byte, err error) {
	oldCurrent := s.Current
	b, err = s.ReadAll()
	if err != nil {
		return b, err
	}
	s.Current, err = s.r.Seek(oldCurrent, io.SeekStart)
	return b, err
}

// Discard discards n bytes
func (s *Scanner) Discard(n int64) (int, error) {
	var err error
	oldCurrent := s.Current
	s.Current, err = s.r.Seek(oldCurrent+n, io.SeekStart)
	return int(s.Current - oldCurrent), err
}

// GetFlatExifData returns a simple, flat representation of all tags.
func (s *Scanner) GetFlatExifData() (exifTags []ExifTag, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	window, err := s.Peek(ExifSignatureLength)
	log.PanicIf(err)

	eh, err := ParseExifHeader(window)
	log.PanicIf(err)

	im := NewIfdMappingWithStandard()
	ti := NewTagIndex()

	ie := NewIfdEnumerate(s, im, ti, eh.ByteOrder)

	exifTags = make([]ExifTag, 0)

	visitor := func(fqIfdPath string, ifdIndex int, ite *IfdTagEntry) (err error) {
		// This encodes down to base64. Since this an example tool and we do not
		// expect to ever decode the output, we are not worried about
		// specifically base64-encoding it in order to have a measure of
		// control.
		valueBytes, err := ite.GetRawBytes()
		if err != nil {
			if err == exifundefined.ErrUnparseableValue {
				return nil
			}

			log.Panic(err)
		}

		value, err := ite.Value()
		if err != nil {
			if err == exifcommon.ErrUnhandledUndefinedTypedTag {
				value = exifundefined.UnparseableUnknownTagValuePlaceholder
			} else {
				log.Panic(err)
			}
		}

		et := ExifTag{
			IfdPath:      fqIfdPath,
			TagId:        ite.TagId(),
			TagName:      ite.TagName(),
			UnitCount:    ite.UnitCount(),
			TagTypeId:    ite.TagType(),
			TagTypeName:  ite.TagType().String(),
			Value:        value,
			ValueBytes:   valueBytes,
			ChildIfdPath: ite.ChildIfdPath(),
		}

		et.Formatted, err = ite.Format()
		log.PanicIf(err)

		et.FormattedFirst, err = ite.FormatFirst()
		log.PanicIf(err)

		exifTags = append(exifTags, et)

		return nil
	}

	_, err = ie.Scan(exifcommon.IfdStandardIfdIdentity, eh.FirstIfdOffset, visitor)
	log.PanicIf(err)

	return exifTags, nil
}
