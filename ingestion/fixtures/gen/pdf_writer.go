// A minimal, hand-rolled, deterministic PDF writer, used ONLY to generate
// this repository's synthetic PDF test fixtures (ingestion/pdf's test
// corpus) — never imported by ingestion/pdf itself or any other
// non-test/non-tool code.
//
// This exists instead of adding a second PDF-writing dependency (alongside
// github.com/ledongthuc/pdf, which only reads PDFs) because: (1) a
// deterministic financial-statement-shaped test fixture needs nothing a
// full PDF-writing library provides beyond positioned Helvetica text runs
// across one or more pages — no images, no complex typography, no
// embedded fonts; (2) hand-constructing the small, well-documented subset
// of PDF syntax this needs keeps the fixture corpus's own dependency
// footprint at zero beyond the standard library, matching this
// repository's general "don't add a dependency for what a hundred lines
// of deterministic code already does correctly" discipline (see
// ingestion/csv's identical zero-dependency stance); (3) hand construction
// makes it trivial to also emit the "no text layer" fixture (a PDF with a
// page containing only a drawn rectangle, no text objects at all) for
// OCR-required-detection testing, which most higher-level PDF-writing
// libraries do not make easy to construct on purpose.
//
// The PDF subset implemented: one Catalog/Pages/Page-per-page object
// graph, one shared Helvetica font resource (with an explicit /Widths
// array — required for github.com/ledongthuc/pdf to report non-zero glyph
// advances/positions correctly; see the ingestion/pdf README section on
// this), positioned text via Tf/Td/Tj content-stream operators, and a
// plain (non-cross-reference-stream) xref table + trailer, which is the
// simplest PDF structure ledongthuc/pdf's reader supports.
package main

import (
	"bytes"
	"fmt"
	"strings"
)

// helveticaWidths are the standard Adobe Font Metrics advance widths (in
// 1000-unit em space) for Helvetica, ASCII 32-126 — the same well-known
// values every PDF-authoring tool assumes for this base-14 font. An
// explicit /Widths array is required here because
// github.com/ledongthuc/pdf's Font.Width() reads only /Widths and does not
// fall back to built-in AFM metrics for an unembedded base-14 font (this
// was confirmed by spiking against a hand-built fixture without /Widths
// before choosing to always include one — see the package doc comment).
var helveticaWidths = map[rune]int{
	' ': 278, '!': 278, '"': 355, '#': 556, '$': 556, '%': 889, '&': 667,
	'\'': 191, '(': 333, ')': 333, '*': 389, '+': 584, ',': 278, '-': 333,
	'.': 278, '/': 278,
	'0': 556, '1': 556, '2': 556, '3': 556, '4': 556, '5': 556, '6': 556,
	'7': 556, '8': 556, '9': 556,
	':': 278, ';': 278, '<': 584, '=': 584, '>': 584, '?': 556, '@': 1015,
	'A': 667, 'B': 667, 'C': 722, 'D': 722, 'E': 667, 'F': 611, 'G': 778,
	'H': 722, 'I': 278, 'J': 500, 'K': 667, 'L': 556, 'M': 833, 'N': 722,
	'O': 778, 'P': 667, 'Q': 778, 'R': 722, 'S': 667, 'T': 611, 'U': 722,
	'V': 667, 'W': 944, 'X': 667, 'Y': 667, 'Z': 611,
	'[': 278, '\\': 278, ']': 278, '^': 469, '_': 556, '`': 333,
	'a': 556, 'b': 556, 'c': 500, 'd': 556, 'e': 556, 'f': 278, 'g': 556,
	'h': 556, 'i': 222, 'j': 222, 'k': 500, 'l': 222, 'm': 833, 'n': 556,
	'o': 556, 'p': 556, 'q': 556, 'r': 333, 's': 500, 't': 278, 'u': 556,
	'v': 500, 'w': 722, 'x': 500, 'y': 500, 'z': 500,
	'{': 334, '|': 260, '}': 334, '~': 584,
}

// pdfTextRun is one positioned line of text to draw on a page: font size
// fixed at 10pt for body text or 12pt for a title, by convention of the
// higher-level fixture builders in generate_pdf.go (this writer itself
// takes an explicit size per run for flexibility).
type pdfTextRun struct {
	x, y float64
	size float64
	text string
}

// pdfPage is one page's content: its text runs and (for the image-only
// fixture) any drawn rectangles with no text at all.
type pdfPage struct {
	runs  []pdfTextRun
	rects [][4]float64 // x, y, w, h
}

// pdfDoc accumulates pages for writePDF.
type pdfDoc struct {
	pages []pdfPage
}

func (d *pdfDoc) addPage(p pdfPage) { d.pages = append(d.pages, p) }

// newLine appends a body-text run (10pt) at the given position.
func (p *pdfPage) newLine(x, y float64, text string) {
	p.runs = append(p.runs, pdfTextRun{x: x, y: y, size: 10, text: text})
}

// newTitle appends a title-text run (14pt, larger than body text — see
// ingestion/pdf's title/boundary-signal detection, which favors short
// lines but does not itself require a larger font; the larger size here
// is purely for fixture realism, matching how real statements visually
// distinguish a title).
func (p *pdfPage) newTitle(x, y float64, text string) {
	p.runs = append(p.runs, pdfTextRun{x: x, y: y, size: 14, text: text})
}

// writePDF renders doc into a complete, valid, minimal PDF byte stream.
// Deterministic: the same doc always produces byte-identical output (no
// timestamps, no random IDs), which downstream ingestion/pdf tests rely on
// for their own "repeated parsing is deterministic" assertions to be
// meaningful all the way back to fixture generation.
func writePDF(doc *pdfDoc) []byte {
	var buf bytes.Buffer
	var offsets []int

	write := func(s string) { buf.WriteString(s) }
	obj := func(n int, body string) {
		offsets = append(offsets, buf.Len())
		write(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", n, body))
	}

	write("%PDF-1.4\n")

	numPages := len(doc.pages)
	if numPages == 0 {
		numPages = 1
		doc.pages = []pdfPage{{}}
	}

	// Object numbering: 1=Catalog, 2=Pages, 3=Font, then 2 objects per
	// page (Page dict, Content stream), in page order.
	const catalogObj = 1
	const pagesObj = 2
	const fontObj = 3
	firstPageObj := fontObj + 1

	pageObjNums := make([]int, numPages)
	contentObjNums := make([]int, numPages)
	for i := 0; i < numPages; i++ {
		pageObjNums[i] = firstPageObj + i*2
		contentObjNums[i] = pageObjNums[i] + 1
	}

	kids := make([]string, numPages)
	for i, n := range pageObjNums {
		kids[i] = fmt.Sprintf("%d 0 R", n)
	}

	widthsBuf := make([]string, 0, 95)
	for c := 32; c <= 126; c++ {
		w := helveticaWidths[rune(c)]
		if w == 0 {
			w = 556
		}
		widthsBuf = append(widthsBuf, fmt.Sprintf("%d", w))
	}

	// obj() must be called in strictly increasing object-number order
	// since it appends to buf sequentially and offsets are recorded by
	// append position — enforced here by construction (catalog, pages,
	// font, then each page's dict+stream in order), not by a general
	// out-of-order writer.
	pendingCatalog := fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesObj)
	pendingPages := fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), numPages)
	pendingFont := fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /FirstChar 32 /LastChar 126 /Widths [%s] >>", strings.Join(widthsBuf, " "))

	obj(catalogObj, pendingCatalog)
	obj(pagesObj, pendingPages)
	obj(fontObj, pendingFont)

	for i, page := range doc.pages {
		content := renderPageContent(page)
		pageDict := fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>",
			pagesObj, fontObj, contentObjNums[i],
		)
		obj(pageObjNums[i], pageDict)
		obj(contentObjNums[i], fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content))
	}

	xrefStart := buf.Len()
	write(fmt.Sprintf("xref\n0 %d\n", len(offsets)+1))
	write("0000000000 65535 f \n")
	for _, off := range offsets {
		write(fmt.Sprintf("%010d 00000 n \n", off))
	}
	write(fmt.Sprintf("trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF", len(offsets)+1, catalogObj, xrefStart))

	return buf.Bytes()
}

func renderPageContent(page pdfPage) string {
	var b strings.Builder
	for _, run := range page.runs {
		fmt.Fprintf(&b, "BT /F1 %g Tf %g %g Td %s Tj ET\n", run.size, run.x, run.y, pdfStringLiteral(run.text))
	}
	for _, r := range page.rects {
		fmt.Fprintf(&b, "%g %g %g %g re f\n", r[0], r[1], r[2], r[3])
	}
	return b.String()
}

// pdfStringLiteral escapes text for use inside a PDF "(...)" string
// literal: backslash, and both parentheses, must be backslash-escaped, per
// the PDF spec's literal-string syntax.
func pdfStringLiteral(text string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(text)
	return "(" + escaped + ")"
}
