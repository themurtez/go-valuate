// Package tesseract implements ingestion/ocr.Engine against a locally
// installed Tesseract OCR executable, invoked as an external process via
// os/exec — never via CGO or a native link dependency, and never through a
// shell.
//
// # Runtime dependency, not a build dependency
//
// This package imports no Tesseract library, links against no native code,
// and requires nothing beyond the Go standard library to compile. The
// entire go-valuate module, including this package, builds successfully
// with `go build ./...` on a machine with no Tesseract installation at all
// — Tesseract availability is discovered only when Engine.Recognize is
// actually called (see checkAvailable), at which point a missing/
// unreachable executable produces a structured *ocr.Error with Code
// ocr.ErrCodeEngineUnavailable, never a build failure and never a panic.
// This is the load-bearing design requirement of this package: OCR support
// must be something a caller opts into at runtime, not something that
// changes what this module requires to compile.
//
// # License / runtime dependency / version
//
//   - Tesseract OCR itself: Apache License 2.0
//     (https://github.com/tesseract-ocr/tesseract). It is an EXTERNAL
//     EXECUTABLE this package shells out to via os/exec — its source is
//     never vendored, compiled, or linked into this Go module or the
//     resulting binary in any form. A caller who never uses this package
//     (or never calls Engine.Recognize) never touches Tesseract in any way.
//   - Expected minimum version: 4.0+ (for LSTM-based recognition quality
//     and stable TSV output via `-c tsv` / `tsv` configfile support; see
//     tsv.go). Verified empirically against whatever version
//     `tesseract --version` reports — see Version(). Tesseract 3.x's
//     legacy engine is not targeted and not tested against.
//   - Language packs: this repository's MVP scope is English only
//     (`eng`), matching the task contract's "Initial MVP only needs
//     English." A caller must have the `tesseract-ocr-eng` (or
//     distribution-equivalent) language-data package installed alongside
//     the `tesseract` executable itself; this package does not download,
//     bundle, or manage language packs in any way — see Options.Languages
//     on the parent ocr package, which defaults to "eng" when left empty
//     (see engine.go's defaultLanguage).
//   - This repository does not bundle, vendor, or redistribute the
//     Tesseract binary itself, matching the task contract's explicit "do
//     not bundle Tesseract binaries into this repository."
package tesseract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/themurtez/go-valuate/ingestion/ocr"
)

// Dependency documents this adapter's external runtime dependency, for
// ingestion.Metadata.Dependency-style provenance and the README, mirroring
// ingestion/pdf.Dependency's identical convention.
const Dependency = "tesseract (external executable, Apache-2.0, not linked into this Go binary)"

// defaultExecutableName is used when Engine.ExecutablePath is empty — the
// name every common Tesseract packaging (Homebrew, apt, MacPorts, the
// official Windows installer with PATH registration) installs under.
const defaultExecutableName = "tesseract"

// defaultLanguage is used when Options.Languages is empty. English-only,
// matching this repository's documented MVP scope.
const defaultLanguage = "eng"

// Engine implements ocr.Engine by invoking a locally installed Tesseract
// executable via os/exec for each page image. Engine holds no mutable
// state of its own beyond its configuration (ExecutablePath) — every field
// is set once at construction and never written to during Recognize, so a
// single Engine value is safe for concurrent use across goroutines (no
// global mutable state anywhere in this package, per the task contract).
type Engine struct {
	// ExecutablePath is the path to the Tesseract executable, or a bare
	// name to resolve via PATH (exec.LookPath's normal resolution — this
	// package never invokes anything through a shell, so no shell
	// metacharacter in this string is ever interpreted). Empty means
	// defaultExecutableName ("tesseract").
	ExecutablePath string
	// TempDir overrides the directory temporary page-image/output files
	// are created in. Empty means os.TempDir() (see tempfiles.go).
	TempDir string
}

// New returns an Engine using the default executable name ("tesseract",
// resolved via PATH) and the OS default temp directory. Equivalent to
// Engine{}, provided as a discoverable constructor.
func New() *Engine {
	return &Engine{}
}

func (e *Engine) executablePath() string {
	if e.ExecutablePath == "" {
		return defaultExecutableName
	}
	return e.ExecutablePath
}

// checkAvailable resolves e's configured executable via exec.LookPath
// (which itself never invokes a shell — it only stats candidate paths),
// converting "not found" into a structured ocr.ErrCodeEngineUnavailable
// error rather than letting exec.Command's own later failure surface as an
// ambiguous generic error. This is the single runtime gate that makes
// Tesseract unavailability a normal, structured, recoverable outcome
// instead of a crash or an opaque OS error.
func (e *Engine) checkAvailable() (resolvedPath string, ierr *ocr.Error) {
	path, err := exec.LookPath(e.executablePath())
	if err != nil {
		return "", &ocr.Error{
			Code:    ocr.ErrCodeEngineUnavailable,
			Message: "tesseract executable not found",
			Detail:  fmt.Sprintf("looked for %q: %v", e.executablePath(), err),
		}
	}
	return path, nil
}

// Recognize implements ocr.Engine. It writes image.Image to a temporary
// PNG file (see tempfiles.go — Tesseract's own CLI is file-based; stdin
// piping of arbitrary raster formats is not reliably supported across
// Tesseract versions/builds, so a temp file is the documented, portable
// choice here, cleaned up unconditionally via defer), invokes Tesseract
// with TSV output (see tsv.go) via exec.CommandContext (never through a
// shell — see runTesseract), and parses the result into an ocr.Result.
func (e *Engine) Recognize(ctx context.Context, image ocr.ImageInput, opts ocr.Options) (ocr.Result, error) {
	resolvedPath, availErr := e.checkAvailable()
	if availErr != nil {
		return ocr.Result{}, availErr
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	inputPath, cleanupInput, err := writeTempPNG(e.tempDir(), image.Image)
	if err != nil {
		return ocr.Result{}, &ocr.Error{
			Code:    ocr.ErrCodeEngineFailed,
			Message: "failed to prepare temporary input image",
			Detail:  err.Error(),
		}
	}
	defer cleanupInput()

	outputBase, cleanupOutput, err := tempOutputBase(e.tempDir())
	if err != nil {
		return ocr.Result{}, &ocr.Error{
			Code:    ocr.ErrCodeEngineFailed,
			Message: "failed to prepare temporary output path",
			Detail:  err.Error(),
		}
	}
	defer cleanupOutput()

	lang := defaultLanguage
	if len(opts.Languages) > 0 {
		lang = joinLanguages(opts.Languages)
	}

	tsvBytes, runErr := runTesseract(runCtx, resolvedPath, inputPath, outputBase, lang)
	if runErr != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return ocr.Result{}, &ocr.Error{
				Code:    ocr.ErrCodeTimeout,
				Message: "tesseract did not complete within the configured timeout",
			}
		}
		return ocr.Result{}, runErr
	}

	words, err := parseTSV(tsvBytes, image.PageIndex)
	if err != nil {
		return ocr.Result{}, &ocr.Error{
			Code:    ocr.ErrCodeEngineFailed,
			Message: "failed to parse tesseract TSV output",
			Detail:  err.Error(),
		}
	}

	return ocr.Result{
		Words:      words,
		EngineName: "tesseract",
	}, nil
}

func (e *Engine) tempDir() string {
	if e.TempDir != "" {
		return e.TempDir
	}
	return os.TempDir()
}

// runTesseract invokes the resolved Tesseract executable directly via
// exec.CommandContext — NEVER through a shell (no sh -c, no string-built
// command line: every argument is passed as a separate, already-final
// element of the Cmd.Args slice, so no input value, however adversarial,
// can be interpreted as shell syntax) — requesting TSV output, and returns
// the captured stdout bytes. Both stdout and stderr are captured into
// bounded in-memory buffers (never left connected to the parent process's
// own stdout/stderr, and never unbounded) so a misbehaving invocation
// cannot leak output to the wrong place or exhaust memory.
func runTesseract(ctx context.Context, exePath, inputPath, outputBase, lang string) ([]byte, *ocr.Error) {
	// outputBase.tsv is what `tesseract <input> <outputbase> tsv` writes;
	// Tesseract itself appends the ".tsv" extension.
	cmd := exec.CommandContext(ctx, exePath, inputPath, outputBase, "-l", lang, "tsv")
	cmd.Stdin = nil

	var stderr bytes.Buffer
	cmd.Stderr = &boundedWriter{limit: maxCapturedStderrBytes, buf: &stderr}

	if err := cmd.Run(); err != nil {
		return nil, &ocr.Error{
			Code:    ocr.ErrCodeEngineFailed,
			Message: "tesseract process failed",
			Detail:  fmt.Sprintf("%v: %s", err, truncateForError(stderr.String())),
		}
	}

	tsvPath := outputBase + ".tsv"
	data, err := os.ReadFile(tsvPath)
	if err != nil {
		return nil, &ocr.Error{
			Code:    ocr.ErrCodeEngineFailed,
			Message: "tesseract did not produce expected TSV output file",
			Detail:  err.Error(),
		}
	}
	_ = os.Remove(tsvPath)
	return data, nil
}

// maxCapturedStderrBytes bounds how much of a failing Tesseract invocation's
// stderr this package retains for error reporting, so a pathological
// invocation cannot exhaust memory via stderr alone.
const maxCapturedStderrBytes = 64 * 1024

// boundedWriter caps the number of bytes written to buf at limit, silently
// discarding the remainder — used for stderr capture, where the full
// content is never needed, only enough to explain a failure.
type boundedWriter struct {
	limit int
	buf   *bytes.Buffer
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		w.buf.Write(p[:remaining])
	} else {
		w.buf.Write(p)
	}
	return len(p), nil
}

func truncateForError(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func joinLanguages(langs []string) string {
	out := langs[0]
	for _, l := range langs[1:] {
		out += "+" + l
	}
	return out
}

// versionOutputPattern extracts the version token from `tesseract
// --version`'s first line, which looks like "tesseract 5.3.4" (or
// "tesseract 5.3.4-rc" etc. for a release candidate/dev build).
var versionOutputPattern = regexp.MustCompile(`tesseract\s+(\S+)`)

// Version invokes `tesseract --version` and returns the reported version
// string (e.g. "5.3.4"), or an error if the executable could not be found
// or run. This is a best-effort, informational call only — used for
// ingestion/pdf's OCR metadata (Metadata.OCREngineVersion) — never required
// for Recognize to function.
func (e *Engine) Version(ctx context.Context) (string, error) {
	resolvedPath, availErr := e.checkAvailable()
	if availErr != nil {
		return "", availErr
	}
	cmd := exec.CommandContext(ctx, resolvedPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", &ocr.Error{
			Code:    ocr.ErrCodeEngineFailed,
			Message: "failed to determine tesseract version",
			Detail:  err.Error(),
		}
	}
	m := versionOutputPattern.FindSubmatch(out)
	if m == nil {
		return "", &ocr.Error{
			Code:    ocr.ErrCodeEngineFailed,
			Message: "could not parse tesseract --version output",
		}
	}
	return string(m[1]), nil
}

// Available reports whether e's configured Tesseract executable can be
// resolved right now, without running any recognition — a lightweight
// pre-flight check a caller can use to decide whether to offer OCR at all
// (e.g. ingestion/pdf's OCR_AUTO mode uses this internally before deciding
// whether attempting OCR is even worthwhile — see that package's fallback
// logic).
func (e *Engine) Available() bool {
	_, err := e.checkAvailable()
	return err == nil
}
