package tabular

import "testing"

func TestParseNumeric(t *testing.T) {
	cases := []struct {
		name          string
		raw           string
		dashTreatment DashTreatment
		wantParsed    bool
		wantValue     float64
		wantBlank     bool
		wantDash      bool
		wantFailed    bool
	}{
		{"plain integer", "1234", DashAsBlank, true, 1234, false, false, false},
		{"thousands separator", "1,234.56", DashAsBlank, true, 1234.56, false, false, false},
		{"dollar prefix", "$1,234.56", DashAsBlank, true, 1234.56, false, false, false},
		{"dollar suffix", "1234.56$", DashAsBlank, true, 1234.56, false, false, false},
		{"parentheses negative", "(1,234.56)", DashAsBlank, true, -1234.56, false, false, false},
		{"leading minus", "-1,234.56", DashAsBlank, true, -1234.56, false, false, false},
		{"leading plus", "+1234.56", DashAsBlank, true, 1234.56, false, false, false},
		{"dollar and minus", "-$1,234.56", DashAsBlank, true, -1234.56, false, false, false},
		{"dollar then minus", "$-1,234.56", DashAsBlank, true, -1234.56, false, false, false},
		{"zero", "0", DashAsBlank, true, 0, false, false, false},
		{"blank", "", DashAsBlank, false, 0, true, false, false},
		{"whitespace only", "   ", DashAsBlank, false, 0, true, false, false},
		{"dash as blank", "-", DashAsBlank, false, 0, false, true, false},
		{"em dash as blank", "—", DashAsBlank, false, 0, false, true, false},
		{"double dash as blank", "--", DashAsBlank, false, 0, false, true, false},
		{"dash as zero", "-", DashAsZero, true, 0, false, true, false},
		{"em dash as zero", "—", DashAsZero, true, 0, false, true, false},
		{"percentage rejected", "12%", DashAsBlank, false, 0, false, false, true},
		{"percentage with decimal rejected", "12.5%", DashAsBlank, false, 0, false, false, true},
		{"malformed NA", "N/A", DashAsBlank, false, 0, false, false, true},
		{"malformed text", "TBD", DashAsBlank, false, 0, false, false, true},
		{"malformed double negative parens", "(-1,234.56)", DashAsBlank, false, 0, false, false, true},
		{"malformed trailing currency word", "1,234.56 USD", DashAsBlank, false, 0, false, false, true},
		{"malformed double decimal", "1.234.56", DashAsBlank, false, 0, false, false, true},
		{"malformed bare currency", "$", DashAsBlank, false, 0, false, false, true},
		{"malformed bare minus", "-$", DashAsBlank, false, 0, false, false, true},
		{"negative zero parens", "(0)", DashAsBlank, true, 0, false, false, false},
		{"large number", "1,000,000.00", DashAsBlank, true, 1000000, false, false, false},
		{"decimal only", ".5", DashAsBlank, true, 0.5, false, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseNumeric(tc.raw, tc.dashTreatment)
			if got.Parsed != tc.wantParsed {
				t.Errorf("Parsed = %v, want %v (result: %+v)", got.Parsed, tc.wantParsed, got)
			}
			if tc.wantParsed && got.Value != tc.wantValue {
				t.Errorf("Value = %v, want %v", got.Value, tc.wantValue)
			}
			if got.IsBlank != tc.wantBlank {
				t.Errorf("IsBlank = %v, want %v", got.IsBlank, tc.wantBlank)
			}
			if got.IsDash != tc.wantDash {
				t.Errorf("IsDash = %v, want %v", got.IsDash, tc.wantDash)
			}
			if got.Failed != tc.wantFailed {
				t.Errorf("Failed = %v, want %v", got.Failed, tc.wantFailed)
			}
		})
	}
}

func TestParseNumericDeterministic(t *testing.T) {
	inputs := []string{"1,234.56", "(500)", "-", "$99.99", "N/A", "12%"}
	for _, in := range inputs {
		first := ParseNumeric(in, DashAsBlank)
		for i := 0; i < 10; i++ {
			got := ParseNumeric(in, DashAsBlank)
			if got != first {
				t.Fatalf("ParseNumeric(%q) not deterministic: %+v vs %+v", in, first, got)
			}
		}
	}
}
