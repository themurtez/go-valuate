package tabular

import "testing"

func TestParsePeriodLabel(t *testing.T) {
	cases := []struct {
		label       string
		wantID      string
		wantType    PeriodType
		wantHasConf bool
	}{
		{"2024", "2024", PeriodTypeCalendarYear, true},
		{"2025", "2025", PeriodTypeCalendarYear, true},
		{"FY2025", "FY2025", PeriodTypeFiscalYear, true},
		{"FY 2025", "FY2025", PeriodTypeFiscalYear, true},
		{"FY'25", "FY2025", PeriodTypeFiscalYear, true},
		{"FY-25", "FY2025", PeriodTypeFiscalYear, true},
		{"Q1 2025", "2025Q1", PeriodTypeQuarter, true},
		{"Q1'25", "2025Q1", PeriodTypeQuarter, true},
		{"2025 Q1", "2025Q1", PeriodTypeQuarter, true},
		{"12/31/2024", "2024", PeriodTypeCalendarYear, true},
		{"Dec 31 2024", "2024", PeriodTypeCalendarYear, true},
		{"December 31, 2024", "2024", PeriodTypeCalendarYear, true},
		{"2024-12-31", "2024", PeriodTypeCalendarYear, true},
		{"March 2025", "2025-03", PeriodTypeMonth, true},
		{"Mar 2025", "2025-03", PeriodTypeMonth, true},
		{"03/2025", "2025-03", PeriodTypeMonth, true},
		{"3/25", "2025-03", PeriodTypeMonth, true},
		{"Jan-Dec 2024", "2024", PeriodTypeCalendarYear, true},
		{"Year Ended December 31, 2024", "2024", PeriodTypeCalendarYear, true},
		{"For the Year Ended Dec 31 2024", "2024", PeriodTypeCalendarYear, true},
		{"YTD", "ytd", PeriodTypeYTD, true},
		{"YTD 2024", "2024-YTD", PeriodTypeYTD, true},
		{"Current Year", "current_year", PeriodTypeUnknown, true},
		{"Prior Year", "prior_year", PeriodTypeUnknown, true},
		{"Some Random Label", "some_random_label", PeriodTypeUnknown, false},
		{"", "unknown_period", PeriodTypeUnknown, false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := ParsePeriodLabel(tc.label)
			if got.CanonicalID != tc.wantID {
				t.Errorf("CanonicalID = %q, want %q", got.CanonicalID, tc.wantID)
			}
			if got.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tc.wantType)
			}
			hasConf := got.Confidence > 0
			if hasConf != tc.wantHasConf {
				t.Errorf("Confidence>0 = %v, want %v (confidence=%v)", hasConf, tc.wantHasConf, got.Confidence)
			}
		})
	}
}

func TestParsePeriodLabelNeverFabricatesDate(t *testing.T) {
	// A label with no recognizable pattern must never produce a StartDate
	// or EndDate — the original label is preserved and Type is Unknown
	// instead.
	got := ParsePeriodLabel("Current Year")
	if got.StartDate != "" || got.EndDate != "" {
		t.Errorf("expected no fabricated dates for ambiguous label, got StartDate=%q EndDate=%q", got.StartDate, got.EndDate)
	}
	if got.Type != PeriodTypeUnknown {
		t.Errorf("expected PeriodTypeUnknown, got %q", got.Type)
	}
}

func TestParsePeriodLabelDeterministic(t *testing.T) {
	labels := []string{"2024", "FY2025", "Q1 2025", "March 2025", "garbage label"}
	for _, l := range labels {
		first := ParsePeriodLabel(l)
		for i := 0; i < 5; i++ {
			got := ParsePeriodLabel(l)
			if got != first {
				t.Fatalf("ParsePeriodLabel(%q) not deterministic: %+v vs %+v", l, first, got)
			}
		}
	}
}

func TestNormalizeYear(t *testing.T) {
	cases := map[string]int{
		"25":   2025,
		"'25":  2025,
		"99":   1999,
		"69":   2069,
		"70":   1970,
		"2024": 2024,
	}
	for in, want := range cases {
		if got := normalizeYear(in); got != want {
			t.Errorf("normalizeYear(%q) = %d, want %d", in, got, want)
		}
	}
}
