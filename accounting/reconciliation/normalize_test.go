package reconciliation

import "testing"

func TestNormalize_TrimCaseFold(t *testing.T) {
	got := NormalizeReference("  CHK-1001  ", ReferenceNormalization{Trim: true, CaseFold: true})
	if got != "chk-1001" {
		t.Fatalf("expected 'chk-1001', got %q", got)
	}
}

func TestNormalize_RemovePunctuationAndSpaces(t *testing.T) {
	got := NormalizeReference("CHK-1001 #A", ReferenceNormalization{RemovePunctuation: true, RemoveSpaces: true})
	if got != "CHK1001A" {
		t.Fatalf("expected 'CHK1001A', got %q", got)
	}
}

func TestNormalize_LeadingZerosMeaningfulByDefault(t *testing.T) {
	got := NormalizeReference("00042", ReferenceNormalization{})
	if got != "00042" {
		t.Fatalf("expected leading zeros preserved by default, got %q", got)
	}
}

func TestNormalize_StripLeadingZerosOptIn(t *testing.T) {
	got := NormalizeReference("00042", ReferenceNormalization{StripLeadingZeros: true})
	if got != "42" {
		t.Fatalf("expected '42', got %q", got)
	}
}

func TestNormalize_AllZerosNeverBecomesEmpty(t *testing.T) {
	got := NormalizeReference("000", ReferenceNormalization{StripLeadingZeros: true})
	if got != "0" {
		t.Fatalf("expected '0' (not empty) for an all-zero reference, got %q", got)
	}
}

func TestNormalize_EmptyReferenceNeverParticipates(t *testing.T) {
	_, ok := normalizedReference("", ReferenceNormalization{Trim: true})
	if ok {
		t.Fatalf("expected empty reference to never resolve to a usable normalized value")
	}
}

func TestNormalize_WhitespaceOnlyBecomesUnusable(t *testing.T) {
	_, ok := normalizedReference("   ", ReferenceNormalization{Trim: true})
	if ok {
		t.Fatalf("expected whitespace-only reference to become unusable after trimming")
	}
}
