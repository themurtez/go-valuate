package classification

import "github.com/themurtez/go-valuate/financial"

// Classify proposes a classification for a single raw line item, following a
// deterministic precedence pipeline:
//
//  1. structural detection — is this row a heading/subtotal/total rather
//     than an ordinary account? This stage checks raw.Kind first (an
//     upstream adapter's own structural read, e.g. ingestion's
//     label-shape-based ClassifyRowKind) before falling back to its own
//     narrower label-token heuristic when raw.Kind is the zero value — see
//     detectStructuralStatus. If the row is structural, no code is
//     proposed and Status/Kind reflect it: RowStatusIgnored for a heading,
//     RowStatusSubtotal/RowStatusTotal otherwise.
//  2. explicit mapping — an exact financial.RawLineItem.ID match in
//     cfg.Explicits.
//  3. alias — an exact match of the row's normalized label against
//     cfg.AliasLayers, resolved by layer precedence (see AliasLayer).
//  4. rules — cfg.Rules evaluated in order; context-aware rules and
//     phrase/token rules are not distinguished by this package beyond their
//     own declared Confidence, so callers control relative precedence by
//     the order they place rules in cfg.Rules (see DefaultRules for the
//     built-in ordering, which places context-aware rules before
//     phrase-only rules).
//  5. UNKNOWN — no stage above produced a match. Classify never falls back
//     to a generic "other" code; see the package README for why.
//
// The first stage to produce a match wins and becomes the Result. Rules
// that matched but did not win are still recorded, sorted by confidence, as
// Result.Alternatives (capped at cfg.maxAlternatives()).
//
// Classify never mutates raw.
func Classify(raw financial.RawLineItem, cfg Config) Result {
	label := NormalizeLabel(raw.Label)
	parent := NormalizeLabel(raw.ParentLabel)

	if status, ok := detectStructuralStatus(label, raw.Kind); ok {
		reason := "label matches a total/subtotal pattern"
		matchedRule := "structural_detection"
		if raw.Kind != financial.RowKindNormal {
			reason = "upstream structural read: " + string(raw.Kind)
			matchedRule = "structural_detection:kind"
		}
		if raw.Kind == financial.RowKindHeading {
			reason = "row is a section heading, not a financial amount"
		}
		return Result{
			RowID:          raw.ID,
			Label:          raw.Label,
			Status:         status,
			Kind:           raw.Kind,
			Confidence:     ConfidenceStructural,
			Source:         SourceStructural,
			Reason:         reason,
			MatchedRule:    matchedRule,
			ReviewRequired: false,
		}
	}

	if code, ok := lookupExplicit(raw.ID, cfg.Explicits); ok {
		return Result{
			RowID:          raw.ID,
			Label:          raw.Label,
			Code:           code,
			Status:         financial.RowStatusNormal,
			Confidence:     ConfidenceExplicit,
			Source:         SourceExplicit,
			Reason:         "explicit mapping supplied for this row",
			MatchedRule:    "explicit:" + raw.ID,
			ReviewRequired: false,
		}
	}

	aliasIndex := buildAliasIndex(cfg.AliasLayers)
	if match, ok := aliasIndex[label.Comparable]; ok {
		return Result{
			RowID:          raw.ID,
			Label:          raw.Label,
			Code:           match.code,
			Status:         financial.RowStatusNormal,
			Confidence:     ConfidenceAlias,
			Source:         SourceAlias,
			Reason:         `matched alias: ` + match.label,
			MatchedRule:    "alias:" + match.layerName + ":" + match.label,
			ReviewRequired: ConfidenceAlias < cfg.reviewThreshold(),
		}
	}

	in := RuleInput{
		Label:         label,
		ParentLabel:   parent,
		StatementType: raw.StatementType,
		Raw:           raw,
	}

	var matches []struct {
		rule  Rule
		match RuleMatch
	}
	for _, rule := range cfg.Rules {
		if rule == nil {
			continue
		}
		if m, ok := rule.Match(in); ok {
			matches = append(matches, struct {
				rule  Rule
				match RuleMatch
			}{rule, m})
		}
	}

	if len(matches) == 0 {
		return Result{
			RowID:          raw.ID,
			Label:          raw.Label,
			Status:         financial.RowStatusNormal,
			Confidence:     ConfidenceUnknown,
			Source:         SourceUnknown,
			Reason:         "no explicit mapping, alias, or rule matched",
			ReviewRequired: true,
		}
	}

	sortMatchesByConfidenceDesc(matches)

	winner := matches[0]
	winnerSource := SourcePhraseRule
	if winner.match.Confidence >= ConfidenceStrongRule {
		winnerSource = SourceContextRule
	}

	result := Result{
		RowID:          raw.ID,
		Label:          raw.Label,
		Code:           winner.match.Code,
		Status:         financial.RowStatusNormal,
		Confidence:     winner.match.Confidence,
		Source:         winnerSource,
		Reason:         winner.match.Reason,
		MatchedRule:    winner.rule.Name(),
		ReviewRequired: winner.match.Confidence < cfg.reviewThreshold(),
	}

	maxAlt := cfg.maxAlternatives()
	for _, m := range matches[1:] {
		if len(result.Alternatives) >= maxAlt {
			break
		}
		if m.match.Code == winner.match.Code {
			continue
		}
		result.Alternatives = append(result.Alternatives, Candidate{
			Code:       m.match.Code,
			Confidence: m.match.Confidence,
			Source:     SourcePhraseRule,
			Reason:     m.match.Reason,
		})
	}

	return result
}

// ClassifyBatch classifies a slice of raw line items, preserving input
// order: result[i] always corresponds to raw[i]. Every input row produces
// exactly one Result, including rows that end up SourceUnknown.
//
// No concurrency is used: classification is cheap, deterministic, in-memory
// work, and preserving strict input order with zero synchronization
// overhead is simpler and just as fast for realistic statement sizes.
func ClassifyBatch(raws []financial.RawLineItem, cfg Config) []Result {
	results := make([]Result, len(raws))
	for i, raw := range raws {
		results[i] = Classify(raw, cfg)
	}
	return results
}

func lookupExplicit(rowID string, explicits []Explicit) (financial.Code, bool) {
	for _, e := range explicits {
		if e.RowID == rowID {
			return e.Code, true
		}
	}
	return "", false
}

func sortMatchesByConfidenceDesc(matches []struct {
	rule  Rule
	match RuleMatch
}) {
	// Simple stable insertion sort: match counts per row are small (a
	// handful of rules), so this avoids importing sort for a closure over
	// an unexported anonymous struct slice while keeping original rule
	// order as a deterministic tiebreaker.
	for i := 1; i < len(matches); i++ {
		j := i
		for j > 0 && matches[j].match.Confidence > matches[j-1].match.Confidence {
			matches[j], matches[j-1] = matches[j-1], matches[j]
			j--
		}
	}
}
