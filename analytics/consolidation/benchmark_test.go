package consolidation

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// benchmarkEntities builds entityCount entities, each with a 5-year
// dataset of the same shape, for a representative multi-entity
// consolidation workload.
func benchmarkEntities(entityCount int) ([]EntityDataset, []financial.Period) {
	codes := []financial.Code{
		financial.CodeRevProduct, financial.CodeCogsMaterial,
		financial.CodeOpexPayroll, financial.CodeOpexRent,
		financial.CodeBsCash, financial.CodeBsAccountsReceivable,
		financial.CodeBsAccountsPayable, financial.CodeBsLongTermDebt,
	}
	periods := []financial.Period{"2021", "2022", "2023", "2024", "2025"}

	entities := make([]EntityDataset, 0, entityCount)
	for e := 0; e < entityCount; e++ {
		var items []financial.NormalizedItem
		for _, p := range periods {
			for i, code := range codes {
				items = append(items, financial.NormalizedItem{Code: code, Period: p, Amount: float64((i + 1 + e) * 10_000)})
			}
		}
		entities = append(entities, EntityDataset{
			EntityID: fmt.Sprintf("entity-%03d", e),
			Dataset:  financial.FinancialDataset{Currency: "USD", Items: items},
		})
	}
	return entities, periods
}

// BenchmarkCalculate_10EntitiesX5Years exercises a representative
// multi-entity consolidation workload: 10 entities, 5 fiscal years each,
// full consolidation with no eliminations.
func BenchmarkCalculate_10EntitiesX5Years(b *testing.B) {
	entities, periods := benchmarkEntities(10)
	in := Input{Entities: entities, Periods: periods}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in)
	}
}
