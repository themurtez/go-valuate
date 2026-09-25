package advisory

// Snapshot is a compact, typed headline-figure view suitable for a future
// dashboard/report renderer — task section 86. Every field is a Metric
// (not a bare Value), so SourceModule/SourceCode/Period are always
// available alongside the figure — task section 87's "no untraceable
// headline number" requirement; a snapshot field is simply
// Metric.Value.Available == false when its underlying source was absent,
// never omitted or forced to a zero-value Metric with fabricated
// provenance.
type Snapshot struct {
	Revenue        Metric `json:"revenue"`
	EBITDA         Metric `json:"ebitda"`
	GrossMargin    Metric `json:"gross_margin"`
	EndingCash     Metric `json:"ending_cash"`
	MinimumCash    Metric `json:"minimum_13_week_cash"`
	AR             Metric `json:"ar"`
	AP             Metric `json:"ap"`
	Inventory      Metric `json:"inventory"`
	DSO            Metric `json:"dso"`
	DPO            Metric `json:"dpo"`
	DIO            Metric `json:"dio"`
	Debt           Metric `json:"debt"`
	CloseReadiness Metric `json:"close_readiness"`
	Valuation      Metric `json:"valuation"`
}

// buildSnapshot pulls one representative Metric per Snapshot field from
// the already-built sections, by fixed Metric.Code lookup — never a
// second read of any source Result (task section 123's "adapt each
// source once" rule; Snapshot is purely a re-projection of Metrics
// section builders already produced). A field stays its zero Metric
// (Value.Available == false, empty provenance) when no section produced a
// Metric with the expected code.
func buildSnapshot(sections []Section) Snapshot {
	find := func(code string) Metric {
		for _, s := range sections {
			for _, m := range s.Metrics {
				if m.Code == code {
					return m
				}
			}
		}
		return Metric{}
	}

	return Snapshot{
		Revenue:        find(metricCodeRevenue),
		EBITDA:         find(metricCodeEBITDA),
		GrossMargin:    find(metricCodeGrossMargin),
		EndingCash:     find(metricCodeEndingCash),
		MinimumCash:    find(metricCodeMinimumCash),
		AR:             find(metricCodeAR),
		AP:             find(metricCodeAP),
		Inventory:      find(metricCodeInventoryValue),
		DSO:            find(metricCodeDSO),
		DPO:            find(metricCodeDPO),
		DIO:            find(metricCodeDIO),
		Debt:           find(metricCodeTotalDebt),
		CloseReadiness: find(metricCodeCloseReadiness),
		Valuation:      find(metricCodeIndicatedValue),
	}
}
