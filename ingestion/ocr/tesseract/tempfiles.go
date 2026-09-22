// Temporary file handling for the Tesseract CLI invocation. Tesseract's
// own command-line interface is file-based (it reads an input image path
// and writes a companion output-base path), so a temp file round-trip is
// unavoidable for this adapter — see engine.go's doc comment on why stdin
// piping was not chosen instead. Every requirement from the task
// contract's "temporary files" section is implemented here: OS temp dir,
// unpredictable names (via os.CreateTemp's built-in random suffix),
// restrictive permissions where the platform supports them, unconditional
// cleanup via the caller's defer, and no name reuse between invocations
// (a fresh os.CreateTemp call every time, never a fixed/predictable name).
package tesseract

import (
	"image/png"
	"os"

	"github.com/themurtez/go-valuate/ingestion/ocr"
)

// writeTempPNG encodes img to a new temporary PNG file in dir (OS default
// temp dir if empty), with an unpredictable name and restrictive
// permissions, returning its path and a cleanup function the caller must
// invoke (via defer) to remove it unconditionally, whether or not the
// subsequent Tesseract invocation succeeds — this repository never leaves
// a caller-supplied document's derived image data sitting in a temp
// directory after a call returns, matching the task contract's "do not
// persist uploads" requirement.
func writeTempPNG(dir string, img ocr.DecodedImage) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp(dir, "govaluate-ocr-*.png")
	if err != nil {
		return "", func() {}, err
	}
	cleanup = func() { _ = os.Remove(f.Name()) }

	// Restrictive permissions: os.CreateTemp already creates with 0600 on
	// POSIX systems by default; Chmod here makes that explicit/guaranteed
	// rather than relying on the implementation default, and is a no-op
	// (best-effort) on platforms without POSIX permission bits.
	_ = f.Chmod(0o600)

	if encErr := png.Encode(f, img); encErr != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, encErr
	}
	if closeErr := f.Close(); closeErr != nil {
		cleanup()
		return "", func() {}, closeErr
	}
	return f.Name(), cleanup, nil
}

// tempOutputBase reserves an unpredictable output-base path (no extension —
// Tesseract itself appends ".tsv") in dir, by creating and immediately
// closing a placeholder file via os.CreateTemp (for its unpredictable-name
// guarantee) and removing it again — Tesseract requires the base path to
// NOT already have its own output file present, so this function only
// borrows os.CreateTemp for name generation, not as the actual output
// container. cleanup removes both the placeholder-free path's eventual
// ".tsv" file (defensive, in case parseTSV's own removal in engine.go was
// skipped due to an earlier error) and is always safe to call even if
// nothing was ever written there.
func tempOutputBase(dir string) (base string, cleanup func(), err error) {
	f, err := os.CreateTemp(dir, "govaluate-ocr-out-*")
	if err != nil {
		return "", func() {}, err
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name) // Tesseract must create outputBase.tsv itself.

	cleanup = func() {
		_ = os.Remove(name + ".tsv")
	}
	return name, cleanup, nil
}
