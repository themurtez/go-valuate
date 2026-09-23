package consolidation

import (
	"math"
	"sort"
	"strconv"

	"github.com/themurtez/go-valuate/financial"
)

// buildEntityContributions computes one EntityContribution per entity in
// selected, restricted to periods, under mode, after applying
// elimByEntity and converting to targetCurrency via rates. It returns the
// contributions (one per entity that had at least one matching item,
// excluding any entity that contributed nothing), the distinct currency
// conversions actually applied, and every issue raised along the way.
func buildEntityContributions(selected []EntityDataset, periods []financial.Period, mode Mode, elimByEntity map[elimKey]float64, targetCurrency string, rates []CurrencyRate) ([]EntityContribution, []CurrencyConversion, []Issue) {
	rateIndex, rateIssues := buildRateIndex(rates)
	periodSet := make(map[financial.Period]struct{}, len(periods))
	for _, p := range periods {
		periodSet[p] = struct{}{}
	}

	var contributions []EntityContribution
	var conversions []CurrencyConversion
	issues := append([]Issue{}, rateIssues...)

	for _, e := range selected {
		if e.Dataset.Currency == "" {
			issues = append(issues, Issue{
				Code:     IssueEntityCurrencyEmpty,
				Severity: SeverityWarning,
				EntityID: e.EntityID,
				Message:  "entity \"" + e.EntityID + "\" has an empty Dataset.Currency and was excluded from consolidation",
			})
			continue
		}

		weight, weightOK, weightIssue := resolveOwnershipWeight(e, mode)
		if weightIssue != nil {
			issues = append(issues, *weightIssue)
		}
		if !weightOK {
			continue
		}

		var items []EntityCodeContribution
		var missingRatePeriods map[financial.Period]struct{}
		var overflowedCount int
		ratesUsed := make(map[financial.Period]float64)

		for _, item := range e.Dataset.Items {
			if _, inScope := periodSet[item.Period]; !inScope {
				continue
			}

			raw := item.Amount
			if delta, ok := elimByEntity[elimKey{entityID: e.EntityID, code: item.Code, period: item.Period}]; ok {
				raw -= delta
			}

			converted := raw
			if e.Dataset.Currency != targetCurrency {
				rate, ok := rateIndex[rateKey{from: e.Dataset.Currency, to: targetCurrency, period: item.Period}]
				if !ok {
					if missingRatePeriods == nil {
						missingRatePeriods = make(map[financial.Period]struct{})
					}
					missingRatePeriods[item.Period] = struct{}{}
					continue
				}
				converted = raw * rate
				if math.IsInf(converted, 0) {
					overflowedCount++
					continue
				}
				ratesUsed[item.Period] = rate
			}

			items = append(items, EntityCodeContribution{
				Code:            item.Code,
				Period:          item.Period,
				RawAmount:       raw,
				ConvertedAmount: converted,
				WeightedAmount:  converted * weight,
			})
		}

		for p := range missingRatePeriods {
			issues = append(issues, Issue{
				Code:     IssueMissingCurrencyRate,
				Severity: SeverityWarning,
				EntityID: e.EntityID,
				Period:   p,
				Message:  "no currency rate " + e.Dataset.Currency + "->" + targetCurrency + " for period \"" + string(p) + "\"; entity \"" + e.EntityID + "\"'s items for that period were excluded",
			})
		}
		if overflowedCount > 0 {
			issues = append(issues, Issue{
				Code:     IssueCurrencyConversionOverflow,
				Severity: SeverityWarning,
				EntityID: e.EntityID,
				Message:  "entity \"" + e.EntityID + "\": " + strconv.Itoa(overflowedCount) + " item(s) overflowed float64's range when converted to " + targetCurrency + " and were excluded",
			})
		}
		for p, rate := range ratesUsed {
			conversions = append(conversions, CurrencyConversion{
				EntityID:     e.EntityID,
				FromCurrency: e.Dataset.Currency,
				ToCurrency:   targetCurrency,
				Period:       p,
				Rate:         rate,
			})
		}

		if len(items) == 0 {
			continue
		}

		sort.Slice(items, func(i, j int) bool {
			if items[i].Code != items[j].Code {
				return items[i].Code < items[j].Code
			}
			return items[i].Period < items[j].Period
		})

		var total float64
		for _, it := range items {
			total += it.WeightedAmount
		}

		contributions = append(contributions, EntityContribution{
			EntityID:                  e.EntityID,
			EntityLabel:               e.EntityLabel,
			Currency:                  e.Dataset.Currency,
			Mode:                      mode,
			OwnershipPercent:          e.OwnershipPercent,
			Items:                     items,
			TotalWeightedContribution: total,
		})
	}

	sort.Slice(conversions, func(i, j int) bool {
		if conversions[i].EntityID != conversions[j].EntityID {
			return conversions[i].EntityID < conversions[j].EntityID
		}
		return conversions[i].Period < conversions[j].Period
	})

	return contributions, conversions, issues
}

// resolveOwnershipWeight determines the multiplier applied to every
// converted amount for e under mode: always 1.0 under
// ModeFullConsolidation, or e.OwnershipPercent under ModeOwnershipWeighted
// (which must be non-nil, finite, and within [0, 1]). The second return
// value is false if e should contribute nothing at all (an invalid/missing
// ownership percent under ModeOwnershipWeighted), in which case the third
// return value carries the Issue explaining why.
func resolveOwnershipWeight(e EntityDataset, mode Mode) (float64, bool, *Issue) {
	if mode != ModeOwnershipWeighted {
		return 1.0, true, nil
	}
	if e.OwnershipPercent == nil {
		return 0, false, &Issue{
			Code:     IssueMissingOwnershipPercent,
			Severity: SeverityWarning,
			EntityID: e.EntityID,
			Message:  "ModeOwnershipWeighted requires OwnershipPercent for entity \"" + e.EntityID + "\", which was nil; it was excluded from consolidation",
		}
	}
	v := *e.OwnershipPercent
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
		return 0, false, &Issue{
			Code:     IssueInvalidOwnershipPercent,
			Severity: SeverityWarning,
			EntityID: e.EntityID,
			Message:  "entity \"" + e.EntityID + "\"'s OwnershipPercent is not a finite number in [0, 1]; it was excluded from consolidation",
		}
	}
	return v, true, nil
}

// buildConsolidatedDataset sums every contribution's WeightedAmount per
// (Code, Period) into a single financial.FinancialDataset, with Sources
// carrying one financial.SourceRef per contributing entity — see
// Result.Consolidated's doc comment.
func buildConsolidatedDataset(contributions []EntityContribution, currency string) financial.FinancialDataset {
	type cell struct {
		code   financial.Code
		period financial.Period
	}
	totals := make(map[cell]float64)
	sources := make(map[cell][]financial.SourceRef)

	for _, c := range contributions {
		for _, it := range c.Items {
			k := cell{code: it.Code, period: it.Period}
			totals[k] += it.WeightedAmount
			sources[k] = append(sources[k], financial.SourceRef{
				RowID:  c.EntityID,
				Label:  c.EntityLabel,
				Period: it.Period,
				Amount: it.WeightedAmount,
			})
		}
	}

	items := make([]financial.NormalizedItem, 0, len(totals))
	for k, amount := range totals {
		refs := sources[k]
		sort.Slice(refs, func(i, j int) bool { return refs[i].RowID < refs[j].RowID })
		items = append(items, financial.NormalizedItem{
			Code:    k.code,
			Period:  k.period,
			Amount:  amount,
			Sources: refs,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Code != items[j].Code {
			return items[i].Code < items[j].Code
		}
		return items[i].Period < items[j].Period
	})

	return financial.FinancialDataset{Currency: currency, Items: items}
}
