// Package csv parses delimited financial statement exports into
// ingestion.Result, using only the Go standard library's encoding/csv (no
// external dependency).
//
// It supports comma, semicolon, and tab delimiters (auto-detected, or
// forced via ingestion.Options.Delimiter), UTF-8 byte-order marks,
// CRLF/LF line endings, quoted cells with embedded delimiters, and blank
// rows. Statement-type/period/structural interpretation is delegated
// entirely to the internal tabular package, shared with ingestion/xlsx —
// see that package's doc comment.
package csv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Parse reads r as delimited text and produces an ingestion.Result. On
// fatal failure it returns a nil *ingestion.Result and a non-nil
// *ingestion.Error (see ingestion.ErrCode* for the possible codes); on
// success it returns a populated Result and a nil error, with any
// non-fatal issues recorded in Result.Warnings.
func Parse(r io.Reader, opts ingestion.Options) (*ingestion.Result, *ingestion.Error) {
	opts = ingestion.ResolveOptions(opts)
	limits := opts.Limits

	limited := io.LimitReader(r, limits.MaxFileSizeBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeInvalidFile,
			Message: "failed to read input",
			Detail:  err.Error(),
		}
	}
	if int64(len(data)) > limits.MaxFileSizeBytes {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeLimitExceeded,
			Message: "input exceeds maximum file size",
			Detail:  fmt.Sprintf("limit is %d bytes", limits.MaxFileSizeBytes),
		}
	}

	data = bytes.TrimPrefix(data, utf8BOM)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "input is empty",
		}
	}

	delim := opts.Delimiter
	if delim == 0 {
		delim = detectDelimiter(data)
	}

	records, warnings, perr := readRecords(data, delim, limits)
	if perr != nil {
		return nil, perr
	}
	if len(records) == 0 {
		return nil, &ingestion.Error{
			Code:    ingestion.ErrCodeNoTabularData,
			Message: "no rows found in input",
		}
	}

	grid := make(tabular.Grid, len(records))
	for i, rec := range records {
		grid[i] = rec
	}

	result, moreWarnings := ingestion.BuildResult(ingestion.BuildInput{
		Format:     ingestion.FormatCSV,
		SheetIndex: 0,
		SheetName:  "",
		Grid:       grid,
		Opts:       opts,
		Formula:    nil,
	})
	result.Metadata.Delimiter = string(delim)
	result.Metadata.Sheets = []ingestion.SheetInfo{{
		Index:    0,
		RowCount: len(records),
		Selected: true,
	}}
	result.Warnings = append(warnings, append(result.Warnings, moreWarnings...)...)

	return &result, nil
}

// readRecords reads every record from data using encoding/csv with the
// given delimiter, enforcing limits.MaxRows/MaxColumns and truncating
// over-long cell text (WarnCellTextTruncated), and reporting malformed
// rows as warnings (WarnMalformedRowSkipped) rather than failing the whole
// parse — encoding/csv's default field-count mismatch behavior is a fatal
// error per-record, so this reads records one at a time to isolate
// failures to the offending row.
func readRecords(data []byte, delim rune, limits ingestion.Limits) ([][]string, []ingestion.Warning, *ingestion.Error) {
	reader := csv.NewReader(bytes.NewReader(normalizeLineEndings(data)))
	reader.Comma = delim
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // allow ragged rows; we validate width ourselves

	var records [][]string
	var warnings []ingestion.Warning
	rowIndex := 0
	for {
		if rowIndex >= limits.MaxRows {
			return nil, nil, &ingestion.Error{
				Code:    ingestion.ErrCodeLimitExceeded,
				Message: "input exceeds maximum row count",
				Detail:  fmt.Sprintf("limit is %d rows", limits.MaxRows),
			}
		}

		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var parseErr *csv.ParseError
			if errors.As(err, &parseErr) {
				warnings = append(warnings, ingestion.Warning{
					Code:     ingestion.WarnMalformedRowSkipped,
					Message:  fmt.Sprintf("row %d could not be parsed: %s", rowIndex, err.Error()),
					RowIndex: rowIndex,
				})
				rowIndex++
				continue
			}
			return nil, nil, &ingestion.Error{
				Code:    ingestion.ErrCodeInvalidFile,
				Message: "failed to parse CSV",
				Detail:  err.Error(),
			}
		}

		if len(record) > limits.MaxColumns {
			warnings = append(warnings, ingestion.Warning{
				Code:     ingestion.WarnMalformedRowSkipped,
				Message:  fmt.Sprintf("row %d has %d columns, exceeding the limit of %d; extra columns dropped", rowIndex, len(record), limits.MaxColumns),
				RowIndex: rowIndex,
			})
			record = record[:limits.MaxColumns]
		}

		for i, cell := range record {
			if len(cell) > limits.MaxCellTextLength {
				record[i] = truncateRunes(cell, limits.MaxCellTextLength)
				warnings = append(warnings, ingestion.Warning{
					Code:        ingestion.WarnCellTextTruncated,
					Message:     fmt.Sprintf("cell at row %d, column %d exceeded max text length and was truncated", rowIndex, i),
					RowIndex:    rowIndex,
					ColumnIndex: i,
				})
			}
		}

		records = append(records, record)
		rowIndex++
	}

	return records, warnings, nil
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// normalizeLineEndings converts CRLF to LF so encoding/csv (which already
// handles bare \r\n within its own scanning) behaves consistently
// regardless of how the input mixes line-ending conventions; this mainly
// guards against a stray lone \r appearing outside encoding/csv's own
// CRLF handling.
func normalizeLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// detectDelimiter picks the most likely field delimiter among comma,
// semicolon, and tab by counting occurrences on the first non-empty line
// (outside quoted spans is not attempted — a simple count is sufficient
// for the vast majority of real exports and this is only used when the
// caller has not forced a delimiter via Options.Delimiter).
func detectDelimiter(data []byte) rune {
	firstLine := data
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		firstLine = data[:idx]
	}
	line := string(firstLine)

	counts := map[rune]int{
		',':  strings.Count(line, ","),
		';':  strings.Count(line, ";"),
		'\t': strings.Count(line, "\t"),
	}

	best := ','
	bestCount := -1
	for _, d := range []rune{',', ';', '\t'} {
		if counts[d] > bestCount {
			bestCount = counts[d]
			best = d
		}
	}
	if bestCount <= 0 {
		return ','
	}
	return best
}
