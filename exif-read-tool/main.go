// This tool dumps EXIF information from images.
//
// Example command-line:
//
//   exif-read-tool -filepath <file-path>
//
// Example Output:
//
//   IFD=[IfdIdentity<PARENT-NAME=[] NAME=[IFD]>] ID=(0x010f) NAME=[Make] COUNT=(6) TYPE=[ASCII] VALUE=[Canon]
//   IFD=[IfdIdentity<PARENT-NAME=[] NAME=[IFD]>] ID=(0x0110) NAME=[Model] COUNT=(22) TYPE=[ASCII] VALUE=[Canon EOS 5D Mark III]
//   IFD=[IfdIdentity<PARENT-NAME=[] NAME=[IFD]>] ID=(0x0112) NAME=[Orientation] COUNT=(1) TYPE=[SHORT] VALUE=[1]
//   IFD=[IfdIdentity<PARENT-NAME=[] NAME=[IFD]>] ID=(0x011a) NAME=[XResolution] COUNT=(1) TYPE=[RATIONAL] VALUE=[72/1]
//   ...
package main

import (
	"flag"
	"fmt"
	"os"

	"encoding/json"
	"io/ioutil"

	log "github.com/dsoprea/go-logging"
	"github.com/jessevdk/go-flags"

	"github.com/imclaren/go-exif"
	exifcommon "github.com/imclaren/go-exif/common"
)

const (
	thumbnailFilenameIndexPlaceholder = "<index>"
)

var (
	mainLogger = log.NewLogger("main.main")
)

// IfdEntry is a JSON model for representing a single tag.
type IfdEntry struct {
	IfdPath     string                      `json:"ifd_path"`
	FqIfdPath   string                      `json:"fq_ifd_path"`
	IfdIndex    int                         `json:"ifd_index"`
	TagId       uint16                      `json:"tag_id"`
	TagName     string                      `json:"tag_name"`
	TagTypeId   exifcommon.TagTypePrimitive `json:"tag_type_id"`
	TagTypeName string                      `json:"tag_type_name"`
	UnitCount   uint32                      `json:"unit_count"`
	Value       interface{}                 `json:"value"`
	ValueString string                      `json:"value_string"`
}

type parameters struct {
	Filepath                string `short:"f" long:"filepath" required:"true" description:"File-path of image"`
	PrintAsJson             bool   `short:"j" long:"json" description:"Print out as JSON"`
	IsVerbose               bool   `short:"v" long:"verbose" description:"Print logging"`
	ThumbnailOutputFilepath string `short:"t" long:"thumbnail-output-filepath" description:"File-path to write thumbnail to (if present)"`
	DoNotPrintTags          bool   `short:"n" long:"no-tags" description:"Do not actually print tags. Good for auditing the logs or merely checking the EXIF structure for errors"`
	ScanLimit               int64  `short:"s" long:"scan-limit" description:"Limit size of EXIF data search.  Set as 0 for no limit.  The default is 1MB"`
}

var (
	arguments = new(parameters)
)

func main() {
	defer func() {
		if errRaw := recover(); errRaw != nil {
			err := errRaw.(error)
			log.PrintError(err)

			os.Exit(-2)
		}
	}()

	_, err := flags.Parse(arguments)
	if err != nil {
		os.Exit(-1)
	}

	if arguments.IsVerbose == true {
		cla := log.NewConsoleLogAdapter()
		log.AddAdapter("console", cla)

		scp := log.NewStaticConfigurationProvider()
		scp.SetLevelName(log.LevelNameDebug)

		log.LoadConfiguration(scp)
	}

	scanLimit := int64(exif.DefaultScanLimit)
	if isFlagPassed("s") {
		scanLimit = arguments.ScanLimit

	}

	f, err := os.Open(arguments.Filepath)
	log.PanicIf(err)
	defer f.Close()

	fi, err := f.Stat()
	log.PanicIf(err)

	s, err := exif.NewScannerLimit(f, fi.Size(), scanLimit)
	if err != nil {
		if err == exif.ErrNoExif {
			fmt.Printf("No EXIF data.\n")
			os.Exit(1)
		}

		log.Panic(err)
	}

	mainLogger.Debugf(nil, "EXIF blob starts at (%d).", s.Current)

	// Run the parse.

	entries, err := s.GetFlatExifData()
	log.PanicIf(err)

	// Write the thumbnail is requested and present.

	thumbnailOutputFilepath := arguments.ThumbnailOutputFilepath
	if thumbnailOutputFilepath != "" {
		im := exif.NewIfdMappingWithStandard()
		ti := exif.NewTagIndex()

		_, index, err := exif.Collect(s, im, ti)
		log.PanicIf(err)

		var thumbnail []byte
		if ifd, found := index.Lookup[exif.ThumbnailFqIfdPath]; found == true {
			thumbnail, err = ifd.Thumbnail()
			if err != nil && err != exif.ErrNoThumbnail {
				log.Panic(err)
			}
		}

		if thumbnail == nil {
			mainLogger.Debugf(nil, "No thumbnails found.")
		} else {
			if arguments.PrintAsJson == false {
				fmt.Printf("Writing (%d) bytes for thumbnail: [%s]\n", len(thumbnail), thumbnailOutputFilepath)
				fmt.Printf("\n")
			}

			err := ioutil.WriteFile(thumbnailOutputFilepath, thumbnail, 0644)
			log.PanicIf(err)
		}
	}

	if arguments.DoNotPrintTags == false {
		if arguments.PrintAsJson == true {
			data, err := json.MarshalIndent(entries, "", "    ")
			log.PanicIf(err)

			fmt.Println(string(data))
		} else {
			thumbnailTags := 0
			for _, entry := range entries {
				fmt.Printf("IFD-PATH=[%s] ID=(0x%04x) NAME=[%s] COUNT=(%d) TYPE=[%s] VALUE=[%s]\n", entry.IfdPath, entry.TagId, entry.TagName, entry.UnitCount, entry.TagTypeName, entry.Formatted)
			}

			fmt.Printf("\n")

			if thumbnailTags == 2 {
				fmt.Printf("There is a thumbnail.\n")
				fmt.Printf("\n")
			}
		}
	}
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
