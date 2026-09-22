package tabular

import "testing"

func TestClassifyRowKind(t *testing.T) {
	cases := []struct {
		name       string
		label      string
		hasValues  bool
		hasAnyText bool
		wantKind   RowKind
	}{
		{"blank row", "", false, false, RowBlank},
		{"heading no values", "Operating Expenses", false, true, RowHeading},
		{"normal account", "Advertising", true, true, RowNormal},
		{"total prefix", "Total Operating Expenses", true, true, RowSubtotal},
		{"subtotal prefix", "Subtotal COGS", true, true, RowSubtotal},
		{"sub-total hyphen", "Sub-total Revenue", true, true, RowSubtotal},
		{"bare total revenue", "Total Revenue", true, true, RowTotal},
		{"bare total expenses", "Total Expenses", true, true, RowTotal},
		{"net income", "Net Income", true, true, RowTotal},
		{"net loss", "Net Loss", true, true, RowTotal},
		{"gross profit", "Gross Profit", true, true, RowSubtotal},
		{"total assets", "Total Assets", true, true, RowTotal},
		{"case insensitive total", "TOTAL REVENUE", true, true, RowTotal},
		{"unrelated label containing net", "Networking Fees", true, true, RowNormal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, _ := ClassifyRowKind(tc.label, tc.hasValues, tc.hasAnyText)
			if kind != tc.wantKind {
				t.Errorf("ClassifyRowKind(%q, %v, %v) = %q, want %q", tc.label, tc.hasValues, tc.hasAnyText, kind, tc.wantKind)
			}
		})
	}
}

func TestClassifyRowKindUncertain(t *testing.T) {
	kind, uncertain := ClassifyRowKind("", true, true)
	if kind != RowNormal {
		t.Errorf("kind = %q, want RowNormal", kind)
	}
	if !uncertain {
		t.Error("expected uncertain=true for values with no label")
	}
}

func TestDetectIndentLevel(t *testing.T) {
	cases := map[string]int{
		"Advertising":     0,
		"  Advertising":   1,
		"    Advertising": 2,
		"\tAdvertising":   1,
		"":                0,
	}
	for label, want := range cases {
		if got := DetectIndentLevel(label); got != want {
			t.Errorf("DetectIndentLevel(%q) = %d, want %d", label, got, want)
		}
	}
}

func TestParentTracker(t *testing.T) {
	tracker := &ParentTracker{}

	// Heading opens a section.
	p := tracker.Observe(RowHeading, 0, "Operating Expenses")
	if p != "Operating Expenses" {
		t.Errorf("after heading, current = %q, want %q", p, "Operating Expenses")
	}

	// Normal rows under the heading inherit its label.
	p = tracker.Observe(RowNormal, 1, "Advertising")
	if p != "Operating Expenses" {
		t.Errorf("child row parent = %q, want %q", p, "Operating Expenses")
	}
	p = tracker.Observe(RowNormal, 1, "Rent")
	if p != "Operating Expenses" {
		t.Errorf("child row parent = %q, want %q", p, "Operating Expenses")
	}

	// A subtotal at the section's own indent closes the section.
	p = tracker.Observe(RowSubtotal, 0, "Total Operating Expenses")
	if p != "Operating Expenses" {
		t.Errorf("subtotal parent = %q, want %q", p, "Operating Expenses")
	}

	// A row after the section closes has no parent.
	p = tracker.Observe(RowNormal, 0, "Other Income")
	if p != "" {
		t.Errorf("after section close, parent = %q, want empty", p)
	}
}

func TestParentTrackerNestedSections(t *testing.T) {
	tracker := &ParentTracker{}
	tracker.Observe(RowHeading, 0, "Assets")
	tracker.Observe(RowHeading, 1, "Current Assets")
	p := tracker.Observe(RowNormal, 2, "Cash")
	if p != "Current Assets" {
		t.Errorf("nested child parent = %q, want %q", p, "Current Assets")
	}

	// Closing the inner section (subtotal at its indent) returns to the
	// outer section.
	tracker.Observe(RowSubtotal, 1, "Total Current Assets")
	p = tracker.Observe(RowNormal, 1, "Fixed Assets")
	if p != "Assets" {
		t.Errorf("after inner section closes, parent = %q, want %q", p, "Assets")
	}
}
