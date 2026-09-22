package pdf

import (
	"testing"

	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

func TestParseOCRNumeric_AlreadyValidNeverTouched(t *testing.T) {
	res := parseOCRNumeric("850,000", tabular.DashAsBlank)
	if !res.Parsed || res.Value != 850000 {
		t.Fatalf("res = %+v, want Parsed=true Value=850000", res)
	}
	if res.CorrectedText != "" {
		t.Errorf("CorrectedText = %q, want empty (no correction needed)", res.CorrectedText)
	}
}

func TestParseOCRNumeric_LetterOForZeroCorrected(t *testing.T) {
	res := parseOCRNumeric("1O,OOO", tabular.DashAsBlank)
	if !res.Parsed {
		t.Fatalf("res = %+v, want Parsed=true", res)
	}
	if res.Value != 10000 {
		t.Errorf("Value = %v, want 10000", res.Value)
	}
	if res.CorrectedText != "10,000" {
		t.Errorf("CorrectedText = %q, want %q", res.CorrectedText, "10,000")
	}
}

func TestParseOCRNumeric_LetterIForOneCorrected(t *testing.T) {
	res := parseOCRNumeric("l,234", tabular.DashAsBlank)
	if !res.Parsed || res.Value != 1234 {
		t.Fatalf("res = %+v, want Parsed=true Value=1234", res)
	}
}

func TestParseOCRNumeric_LetterSForFiveCorrected(t *testing.T) {
	res := parseOCRNumeric("S00", tabular.DashAsBlank)
	if !res.Parsed || res.Value != 500 {
		t.Fatalf("res = %+v, want Parsed=true Value=500", res)
	}
}

func TestParseOCRNumeric_ParenthesesNegativeWithLetterConfusion(t *testing.T) {
	res := parseOCRNumeric("(l2,4OO)", tabular.DashAsBlank)
	if !res.Parsed {
		t.Fatalf("res = %+v, want Parsed=true", res)
	}
	if res.Value != -12400 {
		t.Errorf("Value = %v, want -12400", res.Value)
	}
}

func TestParseOCRNumeric_GenuinelyAmbiguousRejected(t *testing.T) {
	// Mixed letters/digits with no clean correction path (still contains
	// non-numeric-shaped structure after substitution) must be rejected,
	// not guessed.
	res := parseOCRNumeric("Revenue Total", tabular.DashAsBlank)
	if res.Parsed {
		t.Fatalf("res = %+v, want Parsed=false (not numeric-shaped at all)", res)
	}
	if res.Ambiguous {
		t.Error("Ambiguous = true, want false (this is plain text, not an OCR numeric ambiguity)")
	}
}

func TestParseOCRNumeric_CorrectionStillFailsIsAmbiguous(t *testing.T) {
	// Numeric-shaped-with-letters but the correction doesn't resolve to a
	// valid amount (e.g. a genuinely malformed shape with multiple
	// decimal-like separators after substitution).
	res := parseOCRNumeric("1O.O.OO", tabular.DashAsBlank)
	if res.Parsed {
		t.Fatalf("res = %+v, want Parsed=false", res)
	}
	if !res.Ambiguous {
		t.Error("Ambiguous = false, want true (numeric-shaped with letters, but correction still fails)")
	}
}

func TestParseOCRNumeric_BlankCellNeverTouched(t *testing.T) {
	res := parseOCRNumeric("   ", tabular.DashAsBlank)
	if !res.IsBlank {
		t.Errorf("res = %+v, want IsBlank=true", res)
	}
	if res.Ambiguous {
		t.Error("Ambiguous = true for a blank cell, want false")
	}
}

func TestParseOCRNumeric_DashNeverTouched(t *testing.T) {
	res := parseOCRNumeric("-", tabular.DashAsBlank)
	if !res.IsDash {
		t.Errorf("res = %+v, want IsDash=true", res)
	}
}

func TestParseOCRNumeric_PercentageNeverCorrected(t *testing.T) {
	res := parseOCRNumeric("l2%", tabular.DashAsBlank)
	if !res.LooksLikePercentage {
		t.Errorf("res = %+v, want LooksLikePercentage=true", res)
	}
	if res.CorrectedText != "" {
		t.Error("percentage values must never be routed through numeric correction")
	}
}

func TestParseOCRNumeric_CleanLabelTextNeverFlaggedAmbiguous(t *testing.T) {
	// A label cell (e.g. "Cost of Goods Sold") happens to fail numeric
	// parsing but must never be treated as an OCR numeric ambiguity — it's
	// not shaped like a number at all.
	res := parseOCRNumeric("Cost of Goods Sold", tabular.DashAsBlank)
	if res.Ambiguous {
		t.Error("Ambiguous = true for ordinary label text, want false")
	}
}

func TestParseOCRNumeric_DollarPrefixWithConfusion(t *testing.T) {
	res := parseOCRNumeric("$l,234,5OO.OO", tabular.DashAsBlank)
	if !res.Parsed {
		t.Fatalf("res = %+v, want Parsed=true", res)
	}
	if res.Value != 1234500.00 {
		t.Errorf("Value = %v, want 1234500", res.Value)
	}
}
