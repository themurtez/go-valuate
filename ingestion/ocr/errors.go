package ocr

// ErrorCode is a stable identifier for a fatal OCR failure, mirroring
// ingestion.ErrorCode's design (a small closed set of stable string
// constants, never a raw wrapped error as the primary signal a caller
// branches on).
type ErrorCode string

const (
	// ErrCodeEngineUnavailable means the requested OCR engine could not run
	// at all — e.g. ingestion/ocr/tesseract's configured executable is not
	// installed or not on PATH. This is a RUNTIME condition, never a
	// compile-time one: see the ingestion/ocr/tesseract package doc comment
	// for why this repository builds and passes its full test suite on a
	// machine with no OCR engine installed, discovering unavailability only
	// when OCR is actually requested.
	ErrCodeEngineUnavailable ErrorCode = "OCR_ENGINE_UNAVAILABLE"
	// ErrCodeEngineFailed means the engine ran but reported a failure (a
	// non-zero exit code, malformed output it could not parse, etc.).
	ErrCodeEngineFailed ErrorCode = "OCR_ENGINE_FAILED"
	// ErrCodeTimeout means recognition did not complete within the
	// caller-supplied timeout/context deadline.
	ErrCodeTimeout ErrorCode = "OCR_TIMEOUT"
)

// Error is a fatal OCR error. Implements the standard error interface,
// mirroring ingestion.Error's shape exactly (Code/Message/Detail) so a
// caller already handling *ingestion.Error can handle *ocr.Error with the
// same pattern.
type Error struct {
	Code    ErrorCode
	Message string
	Detail  string
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return "ocr: " + string(e.Code) + ": " + e.Message + " (" + e.Detail + ")"
	}
	return "ocr: " + string(e.Code) + ": " + e.Message
}
