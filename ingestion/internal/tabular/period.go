package tabular

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PeriodType mirrors ingestion.PeriodType (see the DashTreatment doc
// comment on this package for why: avoiding an import cycle with the
// parent package).
type PeriodType string

const (
	PeriodTypeFiscalYear   PeriodType = "fiscal_year"
	PeriodTypeCalendarYear PeriodType = "calendar_year"
	PeriodTypeQuarter      PeriodType = "quarter"
	PeriodTypeMonth        PeriodType = "month"
	PeriodTypeYTD          PeriodType = "ytd"
	PeriodTypeUnknown      PeriodType = "unknown"
)

// ParsedPeriod is the outcome of attempting to parse a header label into a
// canonical period identifier.
type ParsedPeriod struct {
	// CanonicalID is the deterministic period identifier, always non-empty.
	// Suitable for use directly as a financial.Period.
	CanonicalID string
	Type        PeriodType
	StartDate   string // YYYY-MM-DD, when determinable
	EndDate     string // YYYY-MM-DD, when determinable
	Confidence  float64
	Evidence    string
}

var monthNames = map[string]int{
	"jan": 1, "january": 1,
	"feb": 2, "february": 2,
	"mar": 3, "march": 3,
	"apr": 4, "april": 4,
	"may": 5,
	"jun": 6, "june": 6,
	"jul": 7, "july": 7,
	"aug": 8, "august": 8,
	"sep": 9, "sept": 9, "september": 9,
	"oct": 10, "october": 10,
	"nov": 11, "november": 11,
	"dec": 12, "december": 12,
}

var monthDays = map[int]int{
	1: 31, 2: 28, 3: 31, 4: 30, 5: 31, 6: 30,
	7: 31, 8: 31, 9: 30, 10: 31, 11: 30, 12: 31,
}

func lastDayOfMonth(year, month int) int {
	if month == 2 && isLeapYear(year) {
		return 29
	}
	return monthDays[month]
}

func isLeapYear(y int) bool {
	return (y%4 == 0 && y%100 != 0) || y%400 == 0
}

var (
	reBareYear = regexp.MustCompile(`^(19|20)\d{2}$`)
	reFYYear   = regexp.MustCompile(`(?i)^FY\s*[-']?\s*(\d{2}|\d{4})$`)
	// "FY2025", "FY 2025", "FY'25", "FY-25"
	reQuarter = regexp.MustCompile(`(?i)^(?:Q([1-4])[\s-]*('?\d{2}|\d{4})|('?\d{2}|\d{4})[\s-]*Q([1-4]))$`)
	// "Q1 2025", "Q1'25", "2025 Q1"
	reMonthYearSlash = regexp.MustCompile(`^(\d{1,2})/(\d{4}|\d{2})$`)
	// "03/2025", "3/25"
	reMonthNameYear = regexp.MustCompile(`(?i)^([A-Za-z]+)\.?\s+(\d{4})$`)
	// "March 2025", "Mar 2025", "Mar. 2025"
	reFullDate = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})/(\d{4})$`)
	// "12/31/2024"
	reFullDateDashed = regexp.MustCompile(`(?i)^([A-Za-z]+)\.?\s+(\d{1,2}),?\s+(\d{4})$`)
	// "Dec 31 2024", "December 31, 2024"
	reISODate = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
	// "2024-12-31"
	reYearEnded = regexp.MustCompile(`(?i)^(?:for the )?years?\s+ended?\s+(.+)$`)
	// "Year Ended December 31, 2024", "For the Year Ended Dec 31 2024"
	reMonthRange = regexp.MustCompile(`(?i)^([A-Za-z]+)-([A-Za-z]+)\s+(\d{4})$`)
	// "Jan-Dec 2024"
	reYTD = regexp.MustCompile(`(?i)^YTD\b.*$`)
)

// ParsePeriodLabel attempts to deterministically parse a header cell's text
// into a canonical period. If no rule matches confidently, CanonicalID
// falls back to a normalized (lowercased, whitespace-collapsed) form of the
// original label, Type is PeriodTypeUnknown, and Confidence is 0 — the
// caller is expected to preserve the original label and emit a warning
// (see the ingestion package's WarnPeriodLabelAmbiguous), never fabricate a
// date.
func ParsePeriodLabel(label string) ParsedPeriod {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return unknownPeriod(trimmed)
	}

	if m := reYearEnded.FindStringSubmatch(trimmed); m != nil {
		if p, ok := parseCalendarDateLabel(strings.TrimSpace(m[1])); ok {
			p.Evidence = `matched "Year Ended <date>" pattern`
			p.Confidence = 0.95
			return p
		}
	}

	if reBareYear.MatchString(trimmed) {
		return ParsedPeriod{
			CanonicalID: trimmed,
			Type:        PeriodTypeCalendarYear,
			StartDate:   fmt.Sprintf("%s-01-01", trimmed),
			EndDate:     fmt.Sprintf("%s-12-31", trimmed),
			Confidence:  0.9,
			Evidence:    "matched bare 4-digit year",
		}
	}

	if m := reFYYear.FindStringSubmatch(trimmed); m != nil {
		year := normalizeYear(m[1])
		return ParsedPeriod{
			CanonicalID: fmt.Sprintf("FY%d", year),
			Type:        PeriodTypeFiscalYear,
			Confidence:  0.9,
			Evidence:    "matched FY YYYY pattern",
		}
	}

	if m := reQuarter.FindStringSubmatch(trimmed); m != nil {
		var q, year int
		if m[1] != "" {
			q, _ = strconv.Atoi(m[1])
			year = normalizeYear(m[2])
		} else {
			q, _ = strconv.Atoi(m[4])
			year = normalizeYear(m[3])
		}
		return ParsedPeriod{
			CanonicalID: fmt.Sprintf("%dQ%d", year, q),
			Type:        PeriodTypeQuarter,
			Confidence:  0.9,
			Evidence:    "matched quarter pattern",
		}
	}

	if reYTD.MatchString(trimmed) {
		// Try to find a trailing year to anchor the ID; fall back to the
		// normalized label if none is present.
		if ym := reBareYearAnywhere.FindString(trimmed); ym != "" {
			return ParsedPeriod{
				CanonicalID: fmt.Sprintf("%s-YTD", ym),
				Type:        PeriodTypeYTD,
				Confidence:  0.8,
				Evidence:    "matched YTD pattern with year",
			}
		}
		return ParsedPeriod{
			CanonicalID: normalizeLabelToID(trimmed),
			Type:        PeriodTypeYTD,
			Confidence:  0.6,
			Evidence:    "matched YTD pattern without a discernible year",
		}
	}

	if m := reMonthRange.FindStringSubmatch(trimmed); m != nil {
		startMonth, ok1 := monthNames[strings.ToLower(m[1])]
		endMonth, ok2 := monthNames[strings.ToLower(m[2])]
		year := normalizeYear(m[3])
		if ok1 && ok2 {
			p := ParsedPeriod{
				CanonicalID: strconv.Itoa(year),
				Type:        PeriodTypeCalendarYear,
				StartDate:   fmt.Sprintf("%04d-%02d-01", year, startMonth),
				EndDate:     fmt.Sprintf("%04d-%02d-%02d", year, endMonth, lastDayOfMonth(year, endMonth)),
				Confidence:  0.85,
				Evidence:    "matched month-range-year pattern (e.g. Jan-Dec 2024)",
			}
			if startMonth == 1 && endMonth == 12 {
				return p
			}
			// A partial-year range (e.g. "Jan-Jun 2024") is not a full
			// calendar year; use a distinct ID so it can't collide with
			// the full-year period for the same year.
			p.CanonicalID = fmt.Sprintf("%d-%02dto%02d", year, startMonth, endMonth)
			return p
		}
	}

	if p, ok := parseCalendarDateLabel(trimmed); ok {
		return p
	}

	if m := reMonthYearSlash.FindStringSubmatch(trimmed); m != nil {
		month, _ := strconv.Atoi(m[1])
		year := normalizeYear(m[2])
		if month >= 1 && month <= 12 {
			return ParsedPeriod{
				CanonicalID: fmt.Sprintf("%04d-%02d", year, month),
				Type:        PeriodTypeMonth,
				StartDate:   fmt.Sprintf("%04d-%02d-01", year, month),
				EndDate:     fmt.Sprintf("%04d-%02d-%02d", year, month, lastDayOfMonth(year, month)),
				Confidence:  0.85,
				Evidence:    "matched MM/YYYY pattern",
			}
		}
	}

	if m := reMonthNameYear.FindStringSubmatch(trimmed); m != nil {
		if month, ok := monthNames[strings.ToLower(m[1])]; ok {
			year := normalizeYear(m[2])
			return ParsedPeriod{
				CanonicalID: fmt.Sprintf("%04d-%02d", year, month),
				Type:        PeriodTypeMonth,
				StartDate:   fmt.Sprintf("%04d-%02d-01", year, month),
				EndDate:     fmt.Sprintf("%04d-%02d-%02d", year, month, lastDayOfMonth(year, month)),
				Confidence:  0.85,
				Evidence:    "matched Month YYYY pattern",
			}
		}
	}

	// "Current Year" / "Prior Year" style labels: recognized as periods of
	// unknown-but-plausible type, never fabricated as a specific date.
	lower := strings.ToLower(trimmed)
	if lower == "current year" || lower == "prior year" || lower == "current period" || lower == "prior period" || lower == "ytd" {
		return ParsedPeriod{
			CanonicalID: normalizeLabelToID(trimmed),
			Type:        PeriodTypeUnknown,
			Confidence:  0.3,
			Evidence:    "relative period label (e.g. \"Current Year\"), no absolute date determinable",
		}
	}

	return unknownPeriod(trimmed)
}

var reBareYearAnywhere = regexp.MustCompile(`(19|20)\d{2}`)

// parseCalendarDateLabel attempts to parse a full calendar date label
// ("12/31/2024", "Dec 31 2024", "December 31, 2024", "2024-12-31") into a
// period anchored on that date's year (calendar-year granularity, since a
// single "as of" or period-end date in a statement header conventionally
// represents that fiscal/calendar year).
func parseCalendarDateLabel(s string) (ParsedPeriod, bool) {
	if m := reFullDate.FindStringSubmatch(s); m != nil {
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return ParsedPeriod{
				CanonicalID: strconv.Itoa(year),
				Type:        PeriodTypeCalendarYear,
				StartDate:   fmt.Sprintf("%04d-01-01", year),
				EndDate:     fmt.Sprintf("%04d-%02d-%02d", year, month, day),
				Confidence:  0.85,
				Evidence:    "matched MM/DD/YYYY date pattern",
			}, true
		}
	}
	if m := reISODate.FindStringSubmatch(s); m != nil {
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		return ParsedPeriod{
			CanonicalID: strconv.Itoa(year),
			Type:        PeriodTypeCalendarYear,
			StartDate:   fmt.Sprintf("%04d-01-01", year),
			EndDate:     fmt.Sprintf("%04d-%02d-%02d", year, month, day),
			Confidence:  0.85,
			Evidence:    "matched ISO date (YYYY-MM-DD) pattern",
		}, true
	}
	if m := reFullDateDashed.FindStringSubmatch(s); m != nil {
		if month, ok := monthNames[strings.ToLower(m[1])]; ok {
			day, _ := strconv.Atoi(m[2])
			year, _ := strconv.Atoi(m[3])
			return ParsedPeriod{
				CanonicalID: strconv.Itoa(year),
				Type:        PeriodTypeCalendarYear,
				StartDate:   fmt.Sprintf("%04d-01-01", year),
				EndDate:     fmt.Sprintf("%04d-%02d-%02d", year, month, day),
				Confidence:  0.85,
				Evidence:    "matched \"Month D, YYYY\" date pattern",
			}, true
		}
	}
	return ParsedPeriod{}, false
}

func normalizeYear(s string) int {
	s = strings.TrimPrefix(s, "'")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	if n < 100 {
		if n < 70 {
			return 2000 + n
		}
		return 1900 + n
	}
	return n
}

func unknownPeriod(label string) ParsedPeriod {
	return ParsedPeriod{
		CanonicalID: normalizeLabelToID(label),
		Type:        PeriodTypeUnknown,
		Confidence:  0,
		Evidence:    "no deterministic period pattern matched; original label preserved",
	}
}

var idWhitespace = regexp.MustCompile(`\s+`)

// normalizeLabelToID produces a deterministic, stable fallback period
// identifier from an arbitrary label: lowercased, whitespace-collapsed,
// spaces replaced with underscores. Used only when no structured pattern
// matched, so the resulting financial.Period is at least consistent across
// repeated parses of the same document.
func normalizeLabelToID(label string) string {
	s := strings.ToLower(strings.TrimSpace(label))
	s = idWhitespace.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " ", "_")
	if s == "" {
		return "unknown_period"
	}
	return s
}
