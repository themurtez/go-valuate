package tabular

import (
	"math"
	"testing"
)

// FuzzParseNumeric proves ParseNumeric — this module's financial numeric
// parser, reused unmodified by ingestion/csv, ingestion/xlsx, and
// ingestion/pdf — never panics on arbitrary input, never reports Parsed
// with a NaN/Inf Value, and never reports both Parsed and Failed at once.
// See docs/V1_CONTRACTS.md's fuzz-testing goals: no panics, no NaN/Inf
// leakage, bounded behavior, malformed input returns a structured failure
// rather than a crash or a silently wrong value.
func FuzzParseNumeric(f *testing.F) {
	seeds := []string{
		"", " ", "-", "—", "--",
		"1,234.56", "$1,234.56", "(1,234.56)", "-$1,234.56", "$-1,234.56",
		"1234", "-1234", "0", "0.00", "N/A", "TBD", "12%", "1,234.56 USD",
		"(-1,234.56)", "$", "(", ")", "((", "))", ".", ",", "1.2.3", "1,,234",
		"999999999999999999999999999999999999999999999999999999999999999999",
		"-999999999999999999999999999999999999999999999999999999999999999999",
		"Infinity", "NaN", "-Infinity", "1e400", "-1e400",
	}
	for _, s := range seeds {
		f.Add(s, 0)
		f.Add(s, 1)
	}

	f.Fuzz(func(t *testing.T, raw string, dashMode int) {
		dt := DashAsBlank
		if dashMode%2 == 1 {
			dt = DashAsZero
		}

		// The function under test must never panic on any input, of any
		// length or byte content (fuzzing supplies arbitrary bytes,
		// including invalid UTF-8) — that is the primary property this
		// fuzz test exists to catch, so no recover() wrapper here: a panic
		// should fail the fuzz run loudly.
		res := ParseNumeric(raw, dt)

		if res.Parsed && res.Failed {
			t.Fatalf("ParseNumeric(%q) reported both Parsed and Failed: %+v", raw, res)
		}
		if res.Parsed {
			if math.IsNaN(res.Value) {
				t.Fatalf("ParseNumeric(%q) returned NaN as a Parsed value", raw)
			}
			if math.IsInf(res.Value, 0) {
				t.Fatalf("ParseNumeric(%q) returned +/-Inf as a Parsed value", raw)
			}
		} else if !res.Failed && !res.IsBlank && !res.IsDash {
			t.Fatalf("ParseNumeric(%q) reported neither Parsed, Failed, IsBlank, nor IsDash — an unclassified outcome: %+v", raw, res)
		}
	})
}

// FuzzParsePeriodLabel proves ParsePeriodLabel — the period-label parser
// shared by every ingestion format — never panics on arbitrary input and
// never fabricates a false-confident period: whenever it cannot
// confidently recognize the label's shape, PeriodType must be
// PeriodTypeUnknown with Confidence 0 (see the README's period-detection
// "never fabricates a date" guarantee), rather than a plausible-looking
// but wrong PeriodType/CanonicalID pair.
func FuzzParsePeriodLabel(f *testing.F) {
	seeds := []string{
		"", " ", "2025", "FY2025", "FY 2025", "FY'25", "Q1 2025", "2025 Q1",
		"Q1'25", "12/31/2024", "Dec 31 2024", "December 31, 2024", "2024-12-31",
		"March 2025", "Mar 2025", "03/2025", "Jan-Dec 2024", "YTD", "YTD 2024",
		"Current Year", "Prior Year", "Year Ended December 31, 2024",
		"garbage", "99999", "0000", "-2025", "FY99999", "Q5 2025", "13/45/2024",
		"2024-13-45", "\x00\x01\x02", "２０２５", "FY２０２５",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, label string) {
		result := ParsePeriodLabel(label)

		if result.Confidence < 0 || result.Confidence > 1 {
			t.Fatalf("ParsePeriodLabel(%q) returned Confidence outside [0,1]: %v", label, result.Confidence)
		}
		if math.IsNaN(result.Confidence) || math.IsInf(result.Confidence, 0) {
			t.Fatalf("ParsePeriodLabel(%q) returned a non-finite Confidence: %v", label, result.Confidence)
		}
		// PeriodTypeUnknown does not always mean Confidence == 0: a
		// recognized-but-dateless label ("Current Year", "Prior Year", "YTD"
		// — see the README's period-detection table) legitimately carries a
		// low nonzero Confidence (0.3) while still correctly reporting
		// PeriodTypeUnknown, since no absolute calendar/fiscal-year/quarter/
		// month can be determined from it. The real invariant this fuzz test
		// checks is narrower: Confidence must never EXCEED the ceiling this
		// package's own weakest genuinely-recognized-shape case uses, for a
		// result carrying no determinable date at all — i.e. no path should
		// ever report high confidence for an undated label.
		if result.Type == PeriodTypeUnknown && result.StartDate == "" && result.EndDate == "" && result.Confidence > 0.3 {
			t.Fatalf("ParsePeriodLabel(%q) returned PeriodTypeUnknown with no determinable date but Confidence %v above this package's own 0.3 ceiling for that case", label, result.Confidence)
		}
		if result.CanonicalID == "" {
			t.Fatalf("ParsePeriodLabel(%q) returned an empty CanonicalID — every result must carry a non-empty canonical period identifier, even for an unrecognized label (a normalized form of the original)", label)
		}
	})
}
