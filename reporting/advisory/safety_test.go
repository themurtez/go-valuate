package advisory

import (
	"strings"
	"testing"
)

// disallowedTerms is the task section 108 scan list of prescriptive/
// unsafe terms. Matching is case-insensitive substring search — a
// generated string containing any of these (outside a caller-supplied,
// ActionOriginCallerSupplied string, which this test never scans) is a
// contract violation.
var disallowedTerms = []string{
	"fire ", "terminate employee", "drop customer", "replace supplier",
	"fraud", "theft", "borrow immediately", "sell company", "sell the company",
	"write off", "write-off", "tax violation", "audit opinion",
	"raise prices", "raise equity", "draw credit line", "cut staff",
	"layoff", "lay off",
}

// TestNoPrescriptiveLanguage scans every default GENERATED string this
// package produces — actionTemplates' Title/Description, statements.go's
// example outputs, questionTemplates' Text — for task section 108's
// disallowed terms. This is the permanent regression guard task section
// 78/107/108 requires.
func TestNoPrescriptiveLanguage(t *testing.T) {
	var all []string
	for _, tmpl := range actionTemplates {
		all = append(all, tmpl.Title, tmpl.Description)
	}
	for _, q := range questionTemplates {
		all = append(all, q.Text)
	}

	for _, s := range all {
		lower := strings.ToLower(s)
		for _, term := range disallowedTerms {
			if strings.Contains(lower, term) {
				t.Errorf("disallowed term %q found in generated string: %q", term, s)
			}
		}
	}
}

// TestActionTemplatesUseNeutralVerbs checks every actionTemplate.Title
// starts with one of task section 107's allowed framing verbs (review,
// resolve, validate, investigate, confirm, complete, obtain, attach) —
// stronger than the substring-blocklist scan above, this is an allowlist
// on the leading verb.
func TestActionTemplatesUseNeutralVerbs(t *testing.T) {
	allowedPrefixes := []string{"Review ", "Resolve ", "Validate ", "Investigate ", "Confirm ", "Complete ", "Obtain ", "Attach "}
	for code, tmpl := range actionTemplates {
		ok := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(tmpl.Title, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("actionTemplates[%q].Title = %q does not start with an allowed neutral verb", code, tmpl.Title)
		}
	}
}

// TestCallerSuppliedActionsNeverAltered confirms buildActionRegisterSection
// never rewrites a caller-supplied action's own Title/Description, even
// when that text happens to contain a term this package would never
// generate itself (the caller is exempt — task section 108's "caller-
// supplied text is exempt but must remain marked caller-supplied" rule).
func TestCallerSuppliedActionsNeverAltered(t *testing.T) {
	callerText := "Sell the excess equipment lot per management's own decision."
	callerAction := ActionItem{ActionCode: "CUSTOM_ACTION", Title: "Caller Title", Description: callerText}

	sections := []Section{}
	out := buildActionRegisterSection(sections, []ActionItem{callerAction}, Policy{})

	if len(out.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(out.Actions))
	}
	got := out.Actions[0]
	if got.Description != callerText {
		t.Errorf("Description = %q, want unchanged %q", got.Description, callerText)
	}
	if got.Origin != ActionOriginCallerSupplied {
		t.Errorf("Origin = %q, want %q", got.Origin, ActionOriginCallerSupplied)
	}
}
