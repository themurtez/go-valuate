package ocr

import "context"

// FakeEngine is a deterministic, fully in-memory Engine implementation with
// no external process/dependency of any kind, used to unit-test every layer
// downstream of Engine (ingestion/pdf's OCR integration, row/column
// reconstruction, numeric-safety handling, fallback-mode selection, mixed
// text/OCR-page handling, timeout/error propagation) without requiring
// Tesseract — or any real OCR engine — to be installed. See the
// ingestion/ocr package doc comment and this repository's task contract's
// "OCR engine testability" requirement.
//
// Not in a _test.go file (and not gated by a build tag) specifically so
// ingestion/pdf's own tests, in a different package, can import and use it
// directly — a package's _test.go-only helpers are invisible to importers
// outside that package's own test binary.
type FakeEngine struct {
	// Pages maps a 0-based page index to the Result FakeEngine returns for
	// that page. A page index with no entry returns an empty Result (zero
	// words), never an error, unless ErrOnPage says otherwise.
	Pages map[int]Result
	// ErrOnPage, when non-nil, is returned verbatim (instead of the mapped
	// Result) for any page index present as a key, letting a test exercise
	// engine-failure/timeout propagation deterministically.
	ErrOnPage map[int]error
	// Calls records every PageIndex Recognize was invoked with, in call
	// order, so a test can assert exactly which pages were actually sent
	// to OCR (e.g. confirming OCR_AUTO skipped a page with usable embedded
	// text).
	Calls []int
}

// Recognize implements Engine. It never touches ctx's deadline itself
// (a fake has no real work to bound), but still honors an already-expired
// context by returning ctx.Err(), so ingestion/pdf's own timeout-handling
// code path can be exercised without a real slow engine.
func (f *FakeEngine) Recognize(ctx context.Context, image ImageInput, opts Options) (Result, error) {
	f.Calls = append(f.Calls, image.PageIndex)

	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if f.ErrOnPage != nil {
		if err, ok := f.ErrOnPage[image.PageIndex]; ok {
			return Result{}, err
		}
	}
	if f.Pages != nil {
		if res, ok := f.Pages[image.PageIndex]; ok {
			return res, nil
		}
	}
	return Result{EngineName: "fake"}, nil
}
