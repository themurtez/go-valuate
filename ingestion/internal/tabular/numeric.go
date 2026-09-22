package tabular

import (
	"strconv"
	"strings"
)

// DashTreatment mirrors ingestion.DashTreatment without importing the
// parent package (which would create an import cycle, since ingestion
// imports this package's exported helpers indirectly through csv/xlsx).
// csv/xlsx translate ingestion.DashTreatment into this type at their call
// sites.
type DashTreatment int

const (
	DashAsBlank DashTreatment = iota
	DashAsZero
)

// NumericResult is the outcome of attempting to parse a single cell's text
// as a monetary amount.
type NumericResult struct {
	// Value is the parsed amount. Valid only when Parsed is true.
	Value float64
	// Parsed is true when Raw was successfully interpreted as a numeric
	// amount (including the dash-as-zero/blank cases, per dashTreatment).
	Parsed bool
	// IsBlank is true when the cell was empty/whitespace-only (not a parse
	// failure — simply no value present).
	IsBlank bool
	// IsDash is true when the cell was a bare dash/em-dash token, before
	// dashTreatment was applied.
	IsDash bool
	// Failed is true when the cell had non-empty text that could not be
	// confidently parsed as a monetary amount (ambiguous or malformed).
	// Mutually exclusive with Parsed.
	Failed bool
	// LooksLikePercentage is true when the cell text ends in "%": such
	// cells are never treated as monetary values (see ParseNumeric), to
	// avoid a percentage like "12%" silently becoming the monetary value
	// 12 or 0.12.
	LooksLikePercentage bool
}

var (
	dashTokens = map[string]bool{
		"-": true, "–": true, "—": true, "--": true, "‐": true,
	}
)

// ParseNumeric interprets raw cell text as a monetary amount under North
// American financial formatting conventions:
//
//   - "1,234.56"    -> 1234.56  (thousands separators)
//   - "$1,234.56"   -> 1234.56  (currency prefix, also suffix "1,234.56$")
//   - "(1,234.56)"  -> -1234.56 (parentheses = negative)
//   - "-1,234.56"   -> -1234.56 (leading minus)
//   - ""             -> IsBlank
//   - "-", "—", "--" -> IsDash, then Value/Parsed per dashTreatment
//   - "0"            -> 0, Parsed
//   - "12%"          -> LooksLikePercentage, Failed (never coerced to a
//     monetary value — see the ingestion contract's numeric parsing
//     section)
//
// Anything else that doesn't cleanly reduce to a signed decimal number
// after stripping the above is reported as Failed rather than silently
// becoming zero, so a caller can emit UNPARSEABLE_NUMERIC_CELL instead of
// fabricating data.
func ParseNumeric(raw string, dashTreatment DashTreatment) NumericResult {
	s := strings.TrimSpace(raw)
	if s == "" {
		return NumericResult{IsBlank: true}
	}

	if dashTokens[s] {
		res := NumericResult{IsDash: true}
		if dashTreatment == DashAsZero {
			res.Value = 0
			res.Parsed = true
		}
		return res
	}

	if strings.HasSuffix(s, "%") {
		return NumericResult{Failed: true, LooksLikePercentage: true}
	}

	negative := false
	work := s

	// Parentheses = negative, e.g. "(1,234.56)".
	if strings.HasPrefix(work, "(") && strings.HasSuffix(work, ")") {
		negative = true
		work = strings.TrimSuffix(strings.TrimPrefix(work, "("), ")")
		work = strings.TrimSpace(work)
	}

	// Strip a currency symbol prefix or suffix, and a sign, in either
	// order ("-$1,234.56" and "$-1,234.56" both occur in real exports), by
	// alternating strip passes until neither makes further progress.
	for {
		before := work
		work = strings.TrimSpace(work)
		work = strings.TrimPrefix(work, "$")
		work = strings.TrimSuffix(work, "$")
		work = strings.TrimSpace(work)

		if strings.HasPrefix(work, "-") {
			if negative {
				// "(-1,234.56)" or "--1,234.56" is ambiguous (double
				// negative signal) — treat as malformed rather than
				// guessing a sign.
				return NumericResult{Failed: true}
			}
			negative = true
			work = strings.TrimPrefix(work, "-")
		} else if strings.HasPrefix(work, "+") {
			work = strings.TrimPrefix(work, "+")
		}

		if work == before {
			break
		}
	}

	if work == "" || dashTokens[work] {
		// e.g. raw was "-" already handled above, but "$-" or "-$" reduces
		// to empty here.
		return NumericResult{Failed: true}
	}

	// Reject anything containing a letter or other unexpected symbol
	// before attempting numeric parse, so e.g. "N/A", "TBD", "1,234.56 USD"
	// are reported as Failed rather than silently truncated by ParseFloat.
	if !isPlausibleNumericToken(work) {
		return NumericResult{Failed: true}
	}

	cleaned := strings.ReplaceAll(work, ",", "")
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return NumericResult{Failed: true}
	}
	if negative {
		value = -value
	}
	return NumericResult{Value: value, Parsed: true}
}

// isPlausibleNumericToken reports whether s contains only digits, commas,
// and at most one decimal point — the only characters a well-formed
// North-American-locale monetary number (after sign/currency/parenthesis
// stripping) should contain.
func isPlausibleNumericToken(s string) bool {
	dots := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == ',':
		case r == '.':
			dots++
			if dots > 1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
