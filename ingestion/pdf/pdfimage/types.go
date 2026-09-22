// Package pdfimage extracts embedded full-page raster images from a
// scanned/image-only PDF page, isolating github.com/pdfcpu/pdfcpu behind
// this package's own types — no pdfcpu type appears in this package's
// exported API (mirroring ingestion/pdf's identical isolation of
// github.com/ledongthuc/pdf, and this repository's general
// dependency-isolation discipline; see extract.go, the only file here that
// imports pdfcpu directly).
//
// # Scope: full-page raster extraction, not PDF rendering
//
// This package does exactly one thing: given a PDF page, find the single
// embedded raster image that plausibly represents that page's entire
// scanned content, decode it to a standard image.Image, and hand it back.
// It never rasterizes/renders a PDF page itself (no vector-graphics
// interpretation of paths, text, or arbitrary content-stream drawing
// operations — pdfcpu is a PDF structure/content editor, not a page
// renderer, and this package does not attempt to work around that). A page
// whose scanned content cannot be represented as one extractable embedded
// raster image (true vector content, an unsupported image filter, multiple
// equally-plausible candidate images, or no image at all) is reported via
// a structured Reason rather than silently guessed at — see Result and
// SelectDominantImage's doc comments.
//
// # Dependency
//
// github.com/pdfcpu/pdfcpu (pinned at the version recorded in
// go.mod/go.sum), Apache-2.0 license. Verified directly (not assumed):
// actively maintained (releases roughly monthly), a clean dependency tree
// with no GPL/AGPL transitive dependencies, and its pkg/api image
// extraction functions (ExtractImagesRaw, PageDims, PageCount) operate on
// io.ReadSeeker — never requiring a filesystem path — matching this
// package's own io.Reader-based public API (see extract.go). pdfcpu is a
// pure-Go library: no CGO, no native link dependency, so it imposes no
// additional build requirement on this module beyond what `go build`
// already needs for every other dependency here.
package pdfimage

import "image"

// ImageFormat identifies the decoded format of an extracted page image, for
// caller-side reporting/provenance. This package always returns an already
// -decoded image.Image regardless of format — ImageFormat is informational
// only.
type ImageFormat string

const (
	ImageFormatJPEG    ImageFormat = "jpeg"
	ImageFormatPNG     ImageFormat = "png"
	ImageFormatTIFF    ImageFormat = "tiff"
	ImageFormatUnknown ImageFormat = "unknown"
)

// PageImage is one embedded raster image found on a PDF page, decoded and
// carrying the metadata SelectDominantImage needs to judge whether it
// plausibly represents the whole page (versus a logo/icon/decoration).
type PageImage struct {
	// Image is the decoded raster.
	Image image.Image
	// Format identifies the source encoding, for provenance only.
	Format ImageFormat
	// PixelWidth, PixelHeight are the image's own intrinsic pixel
	// dimensions (equal to Image.Bounds()'s size, carried separately so
	// callers needing only the numbers don't need to import "image").
	PixelWidth, PixelHeight int
	// ObjectNumber is the PDF object number pdfcpu extracted this image
	// from, for provenance/debugging (never exposed as a pdfcpu type — a
	// plain int).
	ObjectNumber int
}

// PageDimensions is a PDF page's MediaBox size, in PDF points (1/72 inch) —
// the same unit ingestion/pdf's own CellBounds already uses, so a caller
// combining OCR provenance with existing PDF provenance never needs a unit
// conversion.
type PageDimensions struct {
	WidthPoints, HeightPoints float64
}

// Reason is a stable identifier for why SelectDominantImage did not return
// a usable single dominant page image, mirroring ingestion.WarningCode/
// ErrorCode's small-stable-string-constant convention.
type Reason string

const (
	// ReasonNone means a dominant image WAS found — Reason is only
	// meaningful when SelectDominantImage's ok return is false.
	ReasonNone Reason = ""
	// ReasonNoImages means the page has no extractable embedded images at
	// all.
	ReasonNoImages Reason = "NO_DOMINANT_PAGE_IMAGE"
	// ReasonMultiplePlausible means two or more images on the page each
	// independently meet the minimum page-coverage ratio, and this package
	// declines to guess which one is the "real" page scan.
	ReasonMultiplePlausible Reason = "MULTIPLE_PAGE_IMAGES"
	// ReasonUnsupportedLayout means the page's dominant visual content is
	// not representable as a single extractable embedded raster image at
	// all (e.g. genuine vector content, or every embedded image uses an
	// unsupported filter/encoding pdfcpu could not decode — see
	// extract.go's JBIG2 handling).
	ReasonUnsupportedLayout Reason = "UNSUPPORTED_SCANNED_PDF_LAYOUT"
)
