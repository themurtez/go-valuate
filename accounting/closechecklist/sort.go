package closechecklist

import "sort"

// sortStrings returns a sorted copy of ss (nil in, nil out) — never
// mutates the input slice, and never leaves output ordering to Go map
// iteration (section 39/40).
func sortStrings(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}

// sortBlockers orders Blockers by TaskCode, then ReasonCode — section 39.
func sortBlockers(bs []Blocker) []Blocker {
	out := make([]Blocker, len(bs))
	copy(out, bs)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TaskCode != out[j].TaskCode {
			return out[i].TaskCode < out[j].TaskCode
		}
		return out[i].ReasonCode < out[j].ReasonCode
	})
	return out
}

// sortFindings orders Findings by severity rank, then FindingCode
// declaration order, then TaskCode — section 39.
func sortFindings(fs []Finding) []Finding {
	out := make([]Finding, len(fs))
	copy(out, fs)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := findingSeverityRank[out[i].Severity], findingSeverityRank[out[j].Severity]
		if si != sj {
			return si < sj
		}
		ri, rj := findingRank[out[i].Code], findingRank[out[j].Code]
		if ri != rj {
			return ri < rj
		}
		return out[i].TaskCode < out[j].TaskCode
	})
	return out
}

// sortIssues orders Issues by Code, then TaskCode — section 39.
func sortIssues(is []Issue) []Issue {
	out := make([]Issue, len(is))
	copy(out, is)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].TaskCode < out[j].TaskCode
	})
	return out
}

// sortSignOffs orders SignOffs by Role, then ActorRef, then SignOffID —
// section 39.
func sortSignOffs(ss []SignOff) []SignOff {
	out := make([]SignOff, len(ss))
	copy(out, ss)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		if out[i].ActorRef != out[j].ActorRef {
			return out[i].ActorRef < out[j].ActorRef
		}
		return out[i].SignOffID < out[j].SignOffID
	})
	return out
}
