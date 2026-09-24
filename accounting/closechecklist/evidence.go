package closechecklist

// EvidenceRef is a generic, opaque evidence pointer attached to one
// TaskState. This package performs no file storage and no content
// inspection — Reference/SourceRef are caller-defined opaque strings.
type EvidenceRef struct {
	EvidenceID  string `json:"evidence_id"`
	Type        string `json:"type,omitempty"`
	Reference   string `json:"reference,omitempty"`
	Description string `json:"description,omitempty"`
	SourceRef   string `json:"source_ref,omitempty"`
}

// EvidencePolicyType is the fixed set of evidence requirement kinds a
// task may declare.
type EvidencePolicyType string

const (
	EvidenceNone          EvidencePolicyType = "NONE"
	EvidenceAtLeastOne    EvidencePolicyType = "AT_LEAST_ONE"
	EvidenceMinCount      EvidencePolicyType = "MIN_COUNT"
	EvidenceSpecificTypes EvidencePolicyType = "SPECIFIC_TYPES"
)

func isRecognizedEvidencePolicyType(t EvidencePolicyType) bool {
	switch t {
	case "", EvidenceNone, EvidenceAtLeastOne, EvidenceMinCount, EvidenceSpecificTypes:
		return true
	default:
		return false
	}
}

// EvidencePolicy declares one task's evidence requirement. The zero
// value (empty Type) is treated as NONE.
type EvidencePolicy struct {
	Type          EvidencePolicyType `json:"type,omitempty"`
	MinCount      int                `json:"min_count,omitempty"`
	RequiredTypes []string           `json:"required_types,omitempty"`
}

// requiredEvidenceCount reports how many EvidenceRef entries are
// structurally required by p (used for coverage counting — section 20).
// SPECIFIC_TYPES requires one per declared type at minimum.
func (p EvidencePolicy) requiredCount() int {
	switch p.Type {
	case EvidenceAtLeastOne:
		return 1
	case EvidenceMinCount:
		if p.MinCount > 0 {
			return p.MinCount
		}
		return 1
	case EvidenceSpecificTypes:
		if len(p.RequiredTypes) > 0 {
			return len(p.RequiredTypes)
		}
		return 1
	default:
		return 0
	}
}

// evidenceSatisfied reports whether the supplied evidence refs satisfy
// p, plus the count of "present" evidence toward the requirement
// (capped at requiredCount for coverage purposes).
func evidenceSatisfied(p EvidencePolicy, refs []EvidenceRef) bool {
	switch p.Type {
	case "", EvidenceNone:
		return true
	case EvidenceAtLeastOne:
		return len(refs) >= 1
	case EvidenceMinCount:
		min := p.MinCount
		if min <= 0 {
			min = 1
		}
		return len(refs) >= min
	case EvidenceSpecificTypes:
		if len(p.RequiredTypes) == 0 {
			return len(refs) >= 1
		}
		have := make(map[string]bool, len(refs))
		for _, r := range refs {
			have[r.Type] = true
		}
		for _, want := range p.RequiredTypes {
			if !have[want] {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// missingEvidenceTypes returns which of p's RequiredTypes are absent
// from refs — used for Blocker.MissingEvidenceType (section 21). Only
// meaningful for EvidenceSpecificTypes; returns nil otherwise.
func missingEvidenceTypes(p EvidencePolicy, refs []EvidenceRef) []string {
	if p.Type != EvidenceSpecificTypes {
		return nil
	}
	have := make(map[string]bool, len(refs))
	for _, r := range refs {
		have[r.Type] = true
	}
	var missing []string
	for _, want := range p.RequiredTypes {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	return missing
}
