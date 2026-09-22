package ocr

// Word is one recognized word, with the layout information needed to
// reconstruct rows/columns exactly as ingestion/pdf's own positioned-text
// pipeline already does for embedded PDF text (see that package's
// layout.go doc comment) — this is why Word's shape deliberately mirrors
// what a PDF text fragment/word already carries (page, X, Y, width,
// height), rather than being plain OCR-engine output: an OCR Word and a
// PDF-text word are made to be interchangeable inputs to the same
// row/column reconstruction logic.
//
// Coordinates are in pixels of the SOURCE IMAGE that was recognized (top-
// left origin, Y increasing downward — the conventional raster-image
// coordinate system, distinct from a PDF's bottom-up user-space points; see
// ingestion/pdf's OCR integration for how these get converted to the
// same coordinate convention embedded-PDF text already uses before both
// feed the shared layout reconstruction).
type Word struct {
	// Text is the recognized text for this word, exactly as the engine
	// returned it — never modified, corrected, or substituted by this
	// package. See the ingestion/pdf numeric-safety layer for where
	// controlled, auditable correction of NUMERIC cells (not raw OCR
	// words) happens, strictly downstream of this type.
	Text string
	// Confidence is the engine-reported confidence for this word, on
	// whatever scale the engine uses (Tesseract: 0-100; see the package
	// doc comment's "confidence is not a probability" section). Negative
	// means the engine did not report a confidence for this word.
	Confidence float64
	// X, Y are the word's top-left corner, in source-image pixels.
	X, Y int
	// Width, Height are the word's bounding box size, in source-image
	// pixels.
	Width, Height int
	// PageIndex is the 0-based source page this word was recognized on,
	// copied from the originating ImageInput.PageIndex.
	PageIndex int
	// BlockNum, ParNum, LineNum are the engine's own block/paragraph/line
	// grouping identifiers, when the engine supplies them (Tesseract TSV
	// does; see ingestion/ocr/tesseract). -1 means not available. These are
	// preserved as additional layout evidence but are NOT required by
	// ingestion/pdf's row reconstruction, which primarily uses X/Y/Width/
	// Height exactly as it already does for embedded PDF text — see that
	// package's OCR integration doc comment.
	BlockNum, ParNum, LineNum int
}

// Result is everything a single Engine.Recognize call returns for one page
// image.
type Result struct {
	// Words is every recognized word, in the engine's own output order
	// (NOT guaranteed to be reading order — see ingestion/pdf's OCR
	// integration, which sorts by position exactly as it already does for
	// embedded PDF text, never assuming engine output order is meaningful).
	Words []Word
	// EngineName identifies which engine produced this result (e.g.
	// "tesseract"), for Metadata/provenance. Always set by a conforming
	// Engine implementation.
	EngineName string
	// EngineVersion is the engine's own reported version string, when
	// obtainable (e.g. Tesseract's "tesseract 5.3.4"). Empty if the engine
	// could not determine its own version.
	EngineVersion string
}
