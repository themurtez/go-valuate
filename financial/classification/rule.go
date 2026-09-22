package classification

import (
	"strings"

	"github.com/themurtez/go-valuate/financial"
)

// RuleInput is the context a Rule may inspect when deciding whether it
// matches a row. It exposes both the normalized and original label so rules
// can choose whichever is appropriate, plus the row's parent label and
// statement type for context-aware matching.
type RuleInput struct {
	// Label is the row's normalized label.
	Label NormalizedLabel
	// ParentLabel is the row's normalized parent label/section, if any. Its
	// Comparable field is empty when the row has no parent.
	ParentLabel NormalizedLabel
	// StatementType is the row's statement type (income statement, balance
	// sheet, cash flow).
	StatementType financial.StatementType
	// Raw is the original, unmodified row, available for rules that need
	// something not otherwise surfaced above.
	Raw financial.RawLineItem
}

// RuleMatch is what a Rule returns when it matches a RuleInput.
type RuleMatch struct {
	Code       financial.Code
	Confidence Confidence
	Reason     string
}

// Rule is a single deterministic classification rule. Rules are evaluated in
// the order they appear in Config.Rules; the first match at the
// highest-precedence stage wins (see the package README for the full
// pipeline order). A Rule that does not match returns ok == false.
//
// Rules must be pure functions of their input: no I/O, no randomness, no
// hidden state, so that classification stays deterministic and testable in
// isolation.
type Rule interface {
	// Name identifies the rule for Result.MatchedRule and debugging.
	Name() string
	// Match evaluates the rule against in and returns a RuleMatch plus true
	// if it applies, or a zero RuleMatch and false otherwise.
	Match(in RuleInput) (RuleMatch, bool)
}

// RuleFunc adapts a plain function to the Rule interface.
type RuleFunc struct {
	RuleName string
	MatchFn  func(in RuleInput) (RuleMatch, bool)
}

func (f RuleFunc) Name() string { return f.RuleName }

func (f RuleFunc) Match(in RuleInput) (RuleMatch, bool) { return f.MatchFn(in) }

// containsToken reports whether token appears in s as a whole word, i.e.
// bounded by whitespace or string edges, never as a bare substring. This is
// the primitive every built-in phrase/token rule uses so that, for example,
// a rule keyed on the token "ad" never matches inside "advertising" or
// "adjustment": both contain "ad" as a substring but neither contains it as
// a standalone word.
func containsToken(s, token string) bool {
	if token == "" {
		return false
	}
	idx := 0
	for {
		pos := strings.Index(s[idx:], token)
		if pos == -1 {
			return false
		}
		start := idx + pos
		end := start + len(token)

		leftOK := start == 0 || s[start-1] == ' '
		rightOK := end == len(s) || s[end] == ' '
		if leftOK && rightOK {
			return true
		}
		idx = start + 1
		if idx >= len(s) {
			return false
		}
	}
}

// containsAnyToken reports whether s contains any of tokens as a whole word.
func containsAnyToken(s string, tokens ...string) (string, bool) {
	for _, t := range tokens {
		if containsToken(s, t) {
			return t, true
		}
	}
	return "", false
}

// containsPhrase reports whether s contains phrase as a contiguous
// whitespace-bounded sequence of words (phrase itself may be multi-word,
// e.g. "cost of sales"). It reuses the same whole-word boundary rule as
// containsToken, applied to the full phrase, so a phrase never matches
// inside a longer unrelated word.
func containsPhrase(s, phrase string) bool {
	return containsToken(s, phrase)
}
