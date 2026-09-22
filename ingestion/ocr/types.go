// Package ocr defines an engine-agnostic OCR abstraction: a contract for
// turning a single page raster image into positioned text, independent of
// any specific OCR engine.
//
// This package itself performs no OCR and depends on no OCR engine. It
// exists so ingestion/pdf (and, in principle, any future caller) can drive
// OCR through one small interface (Engine) without depending on
// ingestion/ocr/tesseract or any other concrete adapter directly — see
// Engine's doc comment. A caller that never wants OCR never needs to import
// this package's tesseract subpackage at all, and go build ./... succeeds
// on a machine with no OCR engine installed, since nothing here shells out
// or links against a native library; see ingestion/ocr/tesseract's doc
// comment for how that adapter defers the actual external-process
// dependency to runtime.
//
// # Confidence is not a probability
//
// Every OCR engine reports some notion of per-word "confidence," but the
// scale, calibration, and meaning of that number are entirely
// engine-specific (Tesseract's is a 0-100 heuristic score with no
// statistical grounding). Confidence values in this package are carried
// through unmodified from whatever the underlying engine reported, and
// must never be treated as a calibrated probability of correctness, an
// error rate, or any other statistically meaningful quantity — it is only
// useful as a relative, engine-internal signal ("this word scored lower
// than that one"), and only within results from the SAME engine. See
// Word.Confidence.
package ocr

import (
	"context"
	"image"
)

// ImageInput is a single page's raster image to run OCR against, already
// decoded to Go's standard image.Image (see ingestion/pdf/pdfimage for how
// a scanned PDF page's embedded image gets here). Engine implementations
// never receive a PDF, a file path, or engine-specific image types — only
// a decoded image plus this small set of hints.
type ImageInput struct {
	// Image is the decoded page raster. Required.
	Image DecodedImage
	// PageIndex is the 0-based source page this image was extracted from,
	// carried through to every Result element for provenance. Purely
	// informational to the engine (most engines never need it), but always
	// available so a caller assembling PageLayout data does not need to
	// track it out-of-band.
	PageIndex int
	// DPI is the image's known or estimated resolution in dots per inch,
	// when available (0 means unknown). Some engines (including Tesseract)
	// use this as a hint to improve recognition; see
	// ingestion/pdf/pdfimage's DPI-estimation doc comment for how this is
	// derived from a PDF page's MediaBox and the embedded image's pixel
	// dimensions.
	DPI float64
}

// DecodedImage is Go's standard image.Image, aliased under this package's
// own name so callers/docs can talk about "the decoded image this package
// needs" without every reference spelling out the standard library type —
// every standard decoded image (image/jpeg, image/png, etc.) already
// satisfies it directly, so this package never needs its own image type or
// a dependency on any image-decoding library beyond the standard library.
type DecodedImage = image.Image

// Options controls how an Engine performs recognition. Every field is
// engine-agnostic; an adapter that cannot honor a particular field (e.g. an
// engine with no language-pack concept) simply ignores it rather than
// failing.
type Options struct {
	// Languages lists the OCR language pack(s) to use, in engine-specific
	// identifier form (e.g. Tesseract's "eng"). Empty means the engine's
	// own default (English for this repository's supported MVP scope — see
	// the ingestion/ocr/tesseract package doc comment).
	Languages []string
	// Timeout bounds how long a single Recognize call may run, independent
	// of any deadline already present on ctx — whichever is shorter
	// applies. Zero means no additional timeout is imposed beyond ctx's own
	// deadline/cancellation.
	Timeout Timeout
}

// Timeout is a caller-supplied duration in whole seconds, kept as its own
// named type (rather than a bare int or time.Duration) so a zero value
// reads unambiguously as "no additional timeout" at every call site in this
// package and its adapters.
type Timeout int

// Engine is the contract every OCR backend implements. ingestion/pdf and
// any other caller in this repository depend only on this interface, never
// on a concrete adapter type — see ingestion/ocr/tesseract for the one
// adapter this repository ships, and this package's doc comment for why
// that separation keeps OCR a purely opt-in runtime dependency.
type Engine interface {
	// Recognize runs OCR against image and returns positioned recognition
	// results. Implementations must respect ctx cancellation/deadline and
	// opts.Timeout (whichever is more restrictive), must not retain image
	// or any part of Result beyond the call (no shared mutable state
	// between calls — see the ingestion/ocr/tesseract package doc comment's
	// "no global mutable state" requirement), and must return a non-nil
	// error (ideally an *Error with a stable Code — see errors.go) on any
	// failure rather than a partial/zero Result.
	Recognize(ctx context.Context, image ImageInput, opts Options) (Result, error)
}
