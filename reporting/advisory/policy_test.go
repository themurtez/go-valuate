package advisory

import "testing"

func TestPriorityRule_Matches(t *testing.T) {
	cases := []struct {
		name   string
		rule   PriorityRule
		module string
		code   string
		entity string
		want   bool
	}{
		{"empty rule matches anything", PriorityRule{}, "ar", "X", "E1", true},
		{"module match", PriorityRule{MatchSourceModule: "ar"}, "ar", "X", "E1", true},
		{"module mismatch", PriorityRule{MatchSourceModule: "ap"}, "ar", "X", "E1", false},
		{"code match required too", PriorityRule{MatchSourceModule: "ar", MatchSourceCode: "Y"}, "ar", "X", "E1", false},
		{"all three match", PriorityRule{MatchSourceModule: "ar", MatchSourceCode: "X", MatchEntityRef: "E1"}, "ar", "X", "E1", true},
		{"entity mismatch", PriorityRule{MatchEntityRef: "E2"}, "ar", "X", "E1", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.rule.matches(c.module, c.code, c.entity); got != c.want {
				t.Errorf("matches(%q,%q,%q) = %v, want %v", c.module, c.code, c.entity, got, c.want)
			}
		})
	}
}

func TestPriorityRuleKindRank_FixedPrecedence(t *testing.T) {
	// task section 14's fixed precedence: caller override always outranks
	// every computed rule.
	if priorityRuleKindRank(PriorityRuleCallerOverride) >= priorityRuleKindRank(PriorityRuleBlockingCondition) {
		t.Error("CALLER_OVERRIDE must outrank BLOCKING_CONDITION")
	}
	if priorityRuleKindRank(PriorityRuleBlockingCondition) >= priorityRuleKindRank(PriorityRuleLiquidityThreshold) {
		t.Error("BLOCKING_CONDITION must outrank LIQUIDITY_THRESHOLD")
	}
	if priorityRuleKindRank(PriorityRuleInformational) <= priorityRuleKindRank(PriorityRuleWarning) {
		t.Error("INFORMATIONAL must rank after WARNING")
	}
	// Unrecognized kind sorts after every known kind.
	if priorityRuleKindRank(PriorityRuleKind("UNKNOWN")) <= priorityRuleKindRank(PriorityRuleInformational) {
		t.Error("an unrecognized PriorityRuleKind must sort after every known kind")
	}
}

func TestIsMaterial_ZeroThresholdDisablesCheck(t *testing.T) {
	p := MaterialityPolicy{} // every threshold zero
	c := Change{AbsoluteChange: AvailableValue(1000000), PercentChange: AvailableValue(0.99)}
	if p.isMaterial(c) {
		t.Error("a MaterialityPolicy with every threshold at zero must never treat any change as material")
	}
}

func TestIsMaterial_UnavailableChangeNeverMaterial(t *testing.T) {
	p := MaterialityPolicy{AbsoluteThreshold: 1}
	c := Change{} // no Change computed at all
	if p.isMaterial(c) {
		t.Error("an unavailable Change must never be treated as material")
	}
}

func TestIsTerminalStatus(t *testing.T) {
	for _, s := range []ActionStatus{ActionStatusResolved, ActionStatusDeferred, ActionStatusNotRequired} {
		if !isTerminalStatus(s) {
			t.Errorf("isTerminalStatus(%v) = false, want true", s)
		}
	}
	for _, s := range []ActionStatus{ActionStatusOpen, ActionStatusInProgress, ""} {
		if isTerminalStatus(s) {
			t.Errorf("isTerminalStatus(%v) = true, want false", s)
		}
	}
}

func TestUnionStrings(t *testing.T) {
	got := unionStrings([]string{"a", "b"}, []string{"b", "c"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("unionStrings = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unionStrings[%d] = %q, want %q (order must be a's order then b's first-seen order)", i, got[i], want[i])
		}
	}
}

func TestUnionStrings_BothEmpty(t *testing.T) {
	if got := unionStrings(nil, nil); got != nil {
		t.Errorf("unionStrings(nil, nil) = %v, want nil", got)
	}
}
