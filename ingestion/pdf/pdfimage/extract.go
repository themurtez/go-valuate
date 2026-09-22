package pdfimage

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strconv"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"golang.org/x/image/tiff"
)

// Error is a fatal page-image-extraction error, mirroring ingestion.Error's
// Code/Message/Detail shape.
type Error struct {
	Code    string
	Message string
	Detail  string
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return "pdfimage: " + e.Code + ": " + e.Message + " (" + e.Detail + ")"
	}
	return "pdfimage: " + e.Code + ": " + e.Message
}

const (
	// ErrCodeInvalidPDF means pdfcpu could not read/validate the input as a
	// PDF at all.
	ErrCodeInvalidPDF = "INVALID_FILE"
	// ErrCodePageOutOfRange means the requested 1-based page number exceeds
	// the document's page count.
	ErrCodePageOutOfRange = "PAGE_OUT_OF_RANGE"
)

// newConfig returns a pdfcpu configuration with UnsupportedResourceSkip so
// one unsupported image (e.g. a filter this package's decode step does not
// recognize — see decodeImage) never aborts extraction of every other
// supported image on the same page; the caller (ExtractPageImages) still
// learns about the skip via the returned error, which it treats as
// non-fatal.
func newConfig() *model.Configuration {
	conf := model.NewDefaultConfiguration()
	conf.UnsupportedResourcePolicy = model.UnsupportedResourceSkip
	return conf
}

// PageCount returns r's page count. r must support io.Seek (every general
// -purpose PDF library needs random access, exactly as
// github.com/ledongthuc/pdf already requires in ingestion/pdf's own
// extract.go — see this package's doc comment).
func PageCount(r io.ReadSeeker) (int, error) {
	n, err := api.PageCount(r, newConfig())
	if err != nil {
		return 0, &Error{Code: ErrCodeInvalidPDF, Message: "failed to read PDF page count", Detail: err.Error()}
	}
	return n, nil
}

// PageDims returns every page's MediaBox dimensions, in document order
// (1-based page N is pd[N-1]).
func PageDims(r io.ReadSeeker) ([]PageDimensions, error) {
	dims, err := api.PageDims(r, newConfig())
	if err != nil {
		return nil, &Error{Code: ErrCodeInvalidPDF, Message: "failed to read PDF page dimensions", Detail: err.Error()}
	}
	out := make([]PageDimensions, len(dims))
	for i, d := range dims {
		out[i] = PageDimensions{WidthPoints: d.Width, HeightPoints: d.Height}
	}
	return out, nil
}

// ExtractPageImages returns every decoded embedded raster image found on
// the given 1-based page number, plus a non-fatal skipped count for any
// image this package could not decode (an unsupported filter/encoding —
// see decodeImage's doc comment), which the caller can surface as a
// warning rather than a hard failure, matching pdfcpu's own
// UnsupportedResourceSkip semantics (this package never fails a whole page
// merely because ONE embedded image, e.g. a logo using an unusual filter,
// could not be decoded — see SelectDominantImage for how skipped
// candidates are otherwise treated).
func ExtractPageImages(r io.ReadSeeker, pageNumber int) (images []PageImage, skipped int, err error) {
	selected := []string{strconv.Itoa(pageNumber)}
	rawPages, extractErr := api.ExtractImagesRaw(r, selected, newConfig())
	if extractErr != nil && rawPages == nil {
		return nil, 0, &Error{
			Code:    ErrCodeInvalidPDF,
			Message: fmt.Sprintf("failed to extract images from page %d", pageNumber),
			Detail:  extractErr.Error(),
		}
	}

	for _, pageImages := range rawPages {
		for objNr, img := range pageImages {
			data, readErr := io.ReadAll(img)
			if readErr != nil {
				skipped++
				continue
			}
			decoded, format, decodeErr := decodeImage(data, img.FileType)
			if decodeErr != nil {
				skipped++
				continue
			}
			bounds := decoded.Bounds()
			images = append(images, PageImage{
				Image:        decoded,
				Format:       format,
				PixelWidth:   bounds.Dx(),
				PixelHeight:  bounds.Dy(),
				ObjectNumber: objNr,
			})
		}
	}

	return images, skipped, nil
}

// decodeImage decodes raw bytes into a standard image.Image, dispatching
// on pdfcpu's reported FileType ("jpg"/"png"/"tif" — see this package's
// doc comment on which pdfcpu-supported filters map to which decoded
// output format). An image whose FileType this package does not recognize
// (pdfcpu's own JBIG2/JPXDecode raw-passthrough output, "jbig2"/"jpx" —
// see the package doc comment's scope note) is NOT decoded: this package
// has no JBIG2/JPEG2000 decoder, and deliberately does not add one rather
// than risk silently misinterpreting a raw compressed stream as a
// different format. The caller (ExtractPageImages) treats this as one
// skipped candidate, not a fatal error for the whole page.
func decodeImage(data []byte, fileType string) (image.Image, ImageFormat, error) {
	switch fileType {
	case "jpg", "jpeg":
		img, err := jpeg.Decode(bytes.NewReader(data))
		return img, ImageFormatJPEG, err
	case "png":
		img, err := png.Decode(bytes.NewReader(data))
		return img, ImageFormatPNG, err
	case "tif", "tiff":
		img, err := tiff.Decode(bytes.NewReader(data))
		return img, ImageFormatTIFF, err
	default:
		return nil, ImageFormatUnknown, fmt.Errorf("unsupported/undecodable embedded image type %q", fileType)
	}
}
