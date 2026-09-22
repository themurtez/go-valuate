// Tesseract TSV output parsing. `tesseract <in> <out> tsv` produces a
// tab-separated file with one header line followed by one row per
// recognized element (page/block/paragraph/line/word/symbol, distinguished
// by the "level" column), documented by Tesseract itself as:
//
//	level page_num block_num par_num line_num word_num left top width
//	height conf text
//
// level: 1=page, 2=block, 3=paragraph, 4=line, 5=word. This package only
// keeps level-5 (word) rows — the levels above it are aggregate/structural
// rows with no useful "text" of their own for this repository's purposes,
// since ingestion/pdf's own row/column reconstruction (layout.go/
// columns.go) already re-derives line/row grouping from word positions
// exactly as it does for embedded PDF text, rather than trusting any
// engine's own line grouping as authoritative.
package tesseract

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/themurtez/go-valuate/ingestion/ocr"
)

const tsvWordLevel = "5"

// tsvColumnCount is the number of tab-separated columns Tesseract's TSV
// format defines. A row with fewer columns than this is malformed and
// skipped (never partially trusted).
const tsvColumnCount = 12

// parseTSV parses raw Tesseract TSV output into ocr.Word values, tagging
// every word with pageIndex (the ImageInput.PageIndex this recognition run
// was for — Tesseract's own "page_num" column always reads 1 for a
// single-image invocation, which is not the same thing as this repository's
// multi-page-PDF page index, so pageIndex is supplied by the caller rather
// than trusted from the TSV itself).
func parseTSV(data []byte, pageIndex int) ([]ocr.Word, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), maxTSVLineBytes)

	var words []ocr.Word
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue // header row
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < tsvColumnCount {
			continue // malformed row: skip rather than guess field meaning
		}
		if fields[0] != tsvWordLevel {
			continue // page/block/paragraph/line aggregate row, not a word
		}

		text := fields[11]
		if strings.TrimSpace(text) == "" {
			continue // Tesseract emits an empty-text placeholder row per line
		}

		blockNum := parseIntOrDefault(fields[2], -1)
		parNum := parseIntOrDefault(fields[3], -1)
		lineNumField := parseIntOrDefault(fields[4], -1)
		left := parseIntOrDefault(fields[6], 0)
		top := parseIntOrDefault(fields[7], 0)
		width := parseIntOrDefault(fields[8], 0)
		height := parseIntOrDefault(fields[9], 0)
		conf := parseFloatOrDefault(fields[10], -1)

		words = append(words, ocr.Word{
			Text:       text,
			Confidence: conf,
			X:          left,
			Y:          top,
			Width:      width,
			Height:     height,
			PageIndex:  pageIndex,
			BlockNum:   blockNum,
			ParNum:     parNum,
			LineNum:    lineNumField,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning tesseract TSV output: %w", err)
	}
	return words, nil
}

// maxTSVLineBytes bounds a single TSV line's length as a defensive limit
// against a pathological/corrupted Tesseract output stream, independent of
// (and much larger than) any realistic single recognized word's TSV row.
const maxTSVLineBytes = 1024 * 1024

func parseIntOrDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func parseFloatOrDefault(s string, def float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return def
	}
	return f
}
