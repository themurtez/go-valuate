package pdf

import (
	"bytes"
	"fmt"
	"io"

	upstream "github.com/ledongthuc/pdf"

	"github.com/themurtez/go-valuate/ingestion"
)

// Dependency documents the external library this package wraps, for
// ingestion.Metadata.Dependency and the README. github.com/ledongthuc/pdf
// is a pure-Go, dependency-free PDF reader (a maintained fork of the
// archived rsc.io/pdf, itself originally written by The Go Authors),
// BSD-3-Clause licensed. It implements no JavaScript engine, no
// hyperlink/attachment/embedded-file execution, and no macro runtime of any
// kind — it is a structural/content-stream reader only, so there is no
// execution surface to disable (unlike, say, a full-featured PDF renderer).
const Dependency = "github.com/ledongthuc/pdf (BSD-3-Clause)"

// fragment is this package's own copy of a single positioned text
// primitive, decoupled from upstream.Text so no ledongthuc/pdf type
// crosses this package's exported API (see the package doc comment's
// isolation requirement). One fragment is typically a single glyph or a
// short run of glyphs sharing the same font/size/Y — see the package doc
// comment on row/word reconstruction for how these are grouped into
// logical cells.
type fragment struct {
	pageIndex int
	font      string
	fontSize  float64
	x         float64
	y         float64
	w         float64
	s         string
}

// extractResult is everything extractDocument reads from a PDF before any
// layout/statement interpretation runs.
type extractResult struct {
	pageCount int
	fragments []fragment
	// rectsByPage counts drawn-rectangle primitives per page, used only as
	// one signal in hasTextLayer/OCR-required detection (a page with
	// drawn rectangles but zero text fragments looks like a
	// table-of-lines image or a form, not a text statement).
	rectsByPage map[int]int
}

// extractDocument opens data as a PDF and extracts every page's positioned
// text (up to limits), converting any ledongthuc/pdf panic (that library
// uses panic-based internal error handling for malformed object graphs —
// see its Reader.resolve) into a returned *ingestion.Error instead of
// letting it crash the caller, since a PDF is untrusted input and a
// malicious or corrupt file must never be able to bring down the process
// via an unrecovered panic. This is the ONLY function in this package that
// imports github.com/ledongthuc/pdf directly.
func extractDocument(data []byte, limits ingestion.Limits) (result extractResult, ierr *ingestion.Error) {
	defer func() {
		if r := recover(); r != nil {
			result = extractResult{}
			ierr = &ingestion.Error{
				Code:    ingestion.ErrCodeInvalidFile,
				Message: "failed to parse PDF structure",
				Detail:  fmt.Sprintf("%v", r),
			}
		}
	}()

	reader, err := upstream.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return extractResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodeInvalidFile,
			Message: "failed to open PDF document",
			Detail:  err.Error(),
		}
	}

	pageCount := reader.NumPage()
	if pageCount <= 0 {
		return extractResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "PDF document contains no pages",
		}
	}
	if pageCount > limits.MaxPages {
		return extractResult{}, &ingestion.Error{
			Code:    ingestion.ErrCodePDFPageLimitExceeded,
			Message: "PDF document exceeds maximum page count",
			Detail:  fmt.Sprintf("limit is %d pages, document has %d", limits.MaxPages, pageCount),
		}
	}

	out := extractResult{
		pageCount:   pageCount,
		rectsByPage: make(map[int]int),
	}
	totalFragments := 0

	for i := 1; i <= pageCount; i++ {
		pageIndex := i - 1
		content, perr := pageContent(reader, i)
		if perr != nil {
			return extractResult{}, perr
		}

		out.rectsByPage[pageIndex] = len(content.Rect)

		pageTextLen := 0
		for _, t := range content.Text {
			if t.S == "" {
				continue
			}
			totalFragments++
			if totalFragments > limits.MaxTextFragments {
				return extractResult{}, &ingestion.Error{
					Code:    ingestion.ErrCodePDFTextLimitExceeded,
					Message: "PDF document exceeds maximum text fragment count",
					Detail:  fmt.Sprintf("limit is %d fragments", limits.MaxTextFragments),
				}
			}
			pageTextLen += len(t.S)
			if pageTextLen > limits.MaxTextLengthPerPage {
				return extractResult{}, &ingestion.Error{
					Code:    ingestion.ErrCodePDFTextLimitExceeded,
					Message: "PDF page exceeds maximum text length",
					Detail:  fmt.Sprintf("page %d exceeds the %d-rune-per-page limit", i, limits.MaxTextLengthPerPage),
				}
			}
			out.fragments = append(out.fragments, fragment{
				pageIndex: pageIndex,
				font:      t.Font,
				fontSize:  t.FontSize,
				x:         t.X,
				y:         t.Y,
				w:         t.W,
				s:         t.S,
			})
		}
	}

	return out, nil
}

// pageContent isolates the single call to upstream Page.Content() (the one
// ledongthuc/pdf method in the hot path with no panic recovery of its own —
// see its page.go) behind a dedicated recover, so a single malformed page
// deep in a large document fails the whole parse with a clear
// ErrCodeInvalidFile rather than an unrecovered panic, while still keeping
// the recover scoped narrowly (versus the outer extractDocument recover,
// kept as defense in depth for anything this function doesn't anticipate).
func pageContent(reader *upstream.Reader, pageNum int) (c upstream.Content, ierr *ingestion.Error) {
	defer func() {
		if r := recover(); r != nil {
			ierr = &ingestion.Error{
				Code:    ingestion.ErrCodeInvalidFile,
				Message: fmt.Sprintf("failed to read PDF page %d", pageNum),
				Detail:  fmt.Sprintf("%v", r),
			}
		}
	}()
	page := reader.Page(pageNum)
	if page.V.IsNull() {
		return upstream.Content{}, nil
	}
	return page.Content(), nil
}

// readAllLimited reads r fully, up to maxBytes+1 (so an over-limit input is
// detected without needing to know its size up front), mirroring
// ingestion/csv and ingestion/xlsx's identical MaxFileSizeBytes enforcement
// pattern.
func readAllLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	return io.ReadAll(limited)
}
