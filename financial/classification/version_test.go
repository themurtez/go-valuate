package classification

import "testing"

func TestDefaultRulesVersion_NotEmpty(t *testing.T) {
	if DefaultRulesVersion == "" {
		t.Error("DefaultRulesVersion constant must not be empty")
	}
	if len(DefaultRules()) == 0 {
		t.Error("DefaultRules() must not be empty for DefaultRulesVersion to describe anything")
	}
}
