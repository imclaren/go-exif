package exif

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	log "github.com/dsoprea/go-logging"

	exifcommon "github.com/imclaren/go-exif/common"
)

var (
	utilityLogger = log.NewLogger("exif.utility")
)

var (
	timeType = reflect.TypeOf(time.Time{})
)

// ParseExifFullTimestamp parses dates like "2018:11:30 13:01:49" into a UTC
// `time.Time` struct.
func ParseExifFullTimestamp(fullTimestampPhrase string) (timestamp time.Time, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	parts := strings.Split(fullTimestampPhrase, " ")
	datestampValue, timestampValue := parts[0], parts[1]

	dateParts := strings.Split(datestampValue, ":")

	year, err := strconv.ParseUint(dateParts[0], 10, 16)
	if err != nil {
		log.Panicf("could not parse year")
	}

	month, err := strconv.ParseUint(dateParts[1], 10, 8)
	if err != nil {
		log.Panicf("could not parse month")
	}

	day, err := strconv.ParseUint(dateParts[2], 10, 8)
	if err != nil {
		log.Panicf("could not parse day")
	}

	timeParts := strings.Split(timestampValue, ":")

	hour, err := strconv.ParseUint(timeParts[0], 10, 8)
	if err != nil {
		log.Panicf("could not parse hour")
	}

	minute, err := strconv.ParseUint(timeParts[1], 10, 8)
	if err != nil {
		log.Panicf("could not parse minute")
	}

	second, err := strconv.ParseUint(timeParts[2], 10, 8)
	if err != nil {
		log.Panicf("could not parse second")
	}

	timestamp = time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
	return timestamp, nil
}

// ExifFullTimestampString produces a string like "2018:11:30 13:01:49" from a
// `time.Time` struct. It will attempt to convert to UTC first.
func ExifFullTimestampString(t time.Time) (fullTimestampPhrase string) {

	// RELEASE(dustin): Dump this for the next release. It duplicates the same function now in exifcommon.

	return exifcommon.ExifFullTimestampString(t)
}

// ExifTag is one simple representation of a tag in a flat list of all of them.
type ExifTag struct {
	// IfdPath is the fully-qualified IFD path (even though it is not named as
	// such).
	IfdPath string `json:"ifd_path"`

	// TagId is the tag-ID.
	TagId uint16 `json:"id"`

	// TagName is the tag-name. This is never empty.
	TagName string `json:"name"`

	// UnitCount is the recorded number of units constution of the value.
	UnitCount uint32 `json:"unit_count"`

	// TagTypeId is the type-ID.
	TagTypeId exifcommon.TagTypePrimitive `json:"type_id"`

	// TagTypeName is the type name.
	TagTypeName string `json:"type_name"`

	// Value is the decoded value.
	Value interface{} `json:"value"`

	// ValueBytes is the raw, encoded value.
	ValueBytes []byte `json:"value_bytes"`

	// Formatted is the human representation of the first value (tag values are
	// always an array).
	FormattedFirst string `json:"formatted_first"`

	// Formatted is the human representation of the complete value.
	Formatted string `json:"formatted"`

	// ChildIfdPath is the name of the child IFD this tag represents (if it
	// represents any). Otherwise, this is empty.
	ChildIfdPath string `json:"child_ifd_path"`
}

// String returns a string representation.
func (et ExifTag) String() string {
	return fmt.Sprintf(
		"ExifTag<"+
			"IFD-PATH=[%s] "+
			"TAG-ID=(0x%02x) "+
			"TAG-NAME=[%s] "+
			"TAG-TYPE=[%s] "+
			"VALUE=[%v] "+
			"VALUE-BYTES=(%d) "+
			"CHILD-IFD-PATH=[%s]",
		et.IfdPath, et.TagId, et.TagName, et.TagTypeName, et.FormattedFirst,
		len(et.ValueBytes), et.ChildIfdPath)
}

// RELEASE(dustin): In the next release, add an options struct to Scan() and GetFlatExifData(), and put the MiscellaneousExifData in the return.

// GetFlatExifData returns a simple, flat representation of all tags.
func GetFlatExifDataFromBytes(exifDataIn []byte) (exifTags []ExifTag, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	r := bytes.NewReader(exifDataIn)

	s, err := NewScanner(r, int64(len(exifDataIn)))
	log.PanicIf(err)

	return s.GetFlatExifData()
}

// GetFlatExifDataNoLimit returns a simple, flat representation of all tags.
// The scan will have with no size limit.
// All the contents of exifDataIn from the start of the exif block (if any)
// will be held in memory.
func GetFlatExifDataFromBytesNoLimit(exifDataIn []byte) (exifTags []ExifTag, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	r := bytes.NewReader(exifDataIn)

	s, err := NewScannerNoLimit(r, int64(len(exifDataIn)))
	log.PanicIf(err)

	return s.GetFlatExifData()
}

// GpsDegreesEquals returns true if the two `GpsDegrees` are identical.
func GpsDegreesEquals(gi1, gi2 GpsDegrees) bool {
	if gi2.Orientation != gi1.Orientation {
		return false
	}

	degreesRightBound := math.Nextafter(gi1.Degrees, gi1.Degrees+1)
	minutesRightBound := math.Nextafter(gi1.Minutes, gi1.Minutes+1)
	secondsRightBound := math.Nextafter(gi1.Seconds, gi1.Seconds+1)

	if gi2.Degrees < gi1.Degrees || gi2.Degrees >= degreesRightBound {
		return false
	} else if gi2.Minutes < gi1.Minutes || gi2.Minutes >= minutesRightBound {
		return false
	} else if gi2.Seconds < gi1.Seconds || gi2.Seconds >= secondsRightBound {
		return false
	}

	return true
}

// IsTime returns true if the value is a `time.Time`.
func IsTime(v interface{}) bool {
	return reflect.TypeOf(v) == timeType
}
