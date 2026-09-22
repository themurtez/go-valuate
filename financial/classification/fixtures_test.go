package classification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// businessFixtures mirrors the shape of
// fixtures/raw_line_items_by_business_type.json.
type businessFixtures struct {
	HVAC     []financial.RawLineItem `json:"hvac_service_business"`
	Agency   []financial.RawLineItem `json:"agency_professional_services"`
	Manufact []financial.RawLineItem `json:"manufacturer"`
	SaaS     []financial.RawLineItem `json:"growth_software_company"`
}

func loadBusinessFixtures(t *testing.T) businessFixtures {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "raw_line_items_by_business_type.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	// The fixture has a "_comment" field alongside the real keys; decoding
	// into a struct without a matching field simply ignores it.
	var fixtures businessFixtures
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("unmarshaling fixture: %v", err)
	}
	return fixtures
}

// fixtureAliases is a representative global alias layer covering labels
// used across the business-type fixtures that are best resolved by alias
// rather than by a phrase rule (e.g. because the label alone is not a safe
// generic pattern to hardcode into DefaultRules).
func fixtureAliases() []AliasLayer {
	return []AliasLayer{
		{Name: "global", Aliases: []Alias{
			{Label: "Truck & Auto", Code: financial.CodeOpexVehicle},
			{Label: "Bank Charges", Code: financial.CodeOpexProfessionalFees},
			{Label: "Owner Wages", Code: financial.CodeOpexOwnerComp},
			{Label: "Merchant Processing Fees", Code: financial.CodeOpexProfessionalFees},
			{Label: "Subcontract Installers", Code: financial.CodeCogsDirectLabor},
			{Label: "Officer Compensation", Code: financial.CodeOpexOwnerComp},
			{Label: "Direct Materials", Code: financial.CodeCogsMaterial},
			{Label: "Production Labor", Code: financial.CodeCogsDirectLabor},
			{Label: "Payroll - Salaried Staff", Code: financial.CodeOpexPayroll},
			{Label: "Office Rent", Code: financial.CodeOpexRent},
			{Label: "Professional Fees - Legal & Accounting", Code: financial.CodeOpexProfessionalFees},
			{Label: "Amortization of Capitalized Software", Code: financial.CodeAmortization},
		}},
	}
}

func fixtureConfig() Config {
	return Config{
		AliasLayers: fixtureAliases(),
		Rules:       DefaultRules(),
	}
}

// TestFixtures_BusinessRowsClassifyAsExpected exercises Classify/ClassifyBatch
// against realistic rows from four different business archetypes, asserting
// which rows resolve to a specific code, which are recognized as
// structural (total/subtotal), and which are deliberately left UNKNOWN
// because the label alone is genuinely ambiguous (e.g. "Misc", "General",
// "Adjustment", "Other").
func TestFixtures_BusinessRowsClassifyAsExpected(t *testing.T) {
	fixtures := loadBusinessFixtures(t)
	cfg := fixtureConfig()

	type expectation struct {
		rowID       string
		wantCode    financial.Code
		wantStatus  financial.RowStatus
		wantUnknown bool
	}

	all := map[string][]financial.RawLineItem{
		"hvac":   fixtures.HVAC,
		"agency": fixtures.Agency,
		"mfg":    fixtures.Manufact,
		"saas":   fixtures.SaaS,
	}
	for name, rows := range all {
		if len(rows) == 0 {
			t.Fatalf("fixture group %q is empty; fixture file may not have decoded correctly", name)
		}
	}

	expectations := []expectation{
		// HVAC
		{rowID: "hvac-1", wantCode: financial.CodeOpexMarketing, wantStatus: financial.RowStatusNormal},
		{rowID: "hvac-2", wantCode: financial.CodeOpexVehicle, wantStatus: financial.RowStatusNormal},
		{rowID: "hvac-3", wantCode: financial.CodeOpexOwnerComp, wantStatus: financial.RowStatusNormal},
		{rowID: "hvac-4", wantCode: financial.CodeOpexProfessionalFees, wantStatus: financial.RowStatusNormal},
		{rowID: "hvac-5", wantCode: financial.CodeCogsDirectLabor, wantStatus: financial.RowStatusNormal},
		{rowID: "hvac-7", wantCode: financial.CodeCogsDirectLabor, wantStatus: financial.RowStatusNormal},
		{rowID: "hvac-8", wantCode: financial.CodeOpexRepairs, wantStatus: financial.RowStatusNormal},
		{rowID: "hvac-10", wantStatus: financial.RowStatusSubtotal},
		{rowID: "hvac-11", wantStatus: financial.RowStatusSubtotal},

		// Agency
		{rowID: "agency-1", wantCode: financial.CodeOpexSoftware, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-2", wantCode: financial.CodeOpexProfessionalFees, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-3", wantCode: financial.CodeOpexOwnerComp, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-4", wantCode: financial.CodeOpexMarketing, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-5", wantCode: financial.CodeOpexPayroll, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-6", wantCode: financial.CodeOpexTravel, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-7", wantCode: financial.CodeOpexRent, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-8", wantCode: financial.CodeOpexProfessionalFees, wantStatus: financial.RowStatusNormal},
		{rowID: "agency-9", wantUnknown: true},
		{rowID: "agency-10", wantStatus: financial.RowStatusSubtotal},

		// Manufacturer
		{rowID: "mfg-1", wantCode: financial.CodeCogsFreight, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-2", wantCode: financial.CodeCogsFreight, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-3", wantCode: financial.CodeCogsMaterial, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-4", wantCode: financial.CodeCogsDirectLabor, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-5", wantCode: financial.CodeDepreciation, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-6", wantCode: financial.CodeInterestExpense, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-7", wantCode: financial.CodeOpexRepairs, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-8", wantUnknown: true},
		{rowID: "mfg-9", wantCode: financial.CodeBsAccountsReceivable, wantStatus: financial.RowStatusNormal},
		{rowID: "mfg-11", wantStatus: financial.RowStatusSubtotal},

		// SaaS
		{rowID: "saas-1", wantCode: financial.CodeOpexSoftware, wantStatus: financial.RowStatusNormal},
		{rowID: "saas-2", wantCode: financial.CodeOpexPayroll, wantStatus: financial.RowStatusNormal},
		{rowID: "saas-3", wantCode: financial.CodeOpexMarketing, wantStatus: financial.RowStatusNormal},
		{rowID: "saas-4", wantCode: financial.CodeOpexProfessionalFees, wantStatus: financial.RowStatusNormal},
		{rowID: "saas-5", wantCode: financial.CodeAmortization, wantStatus: financial.RowStatusNormal},
		{rowID: "saas-6", wantCode: financial.CodeInterestIncome, wantStatus: financial.RowStatusNormal},
		{rowID: "saas-7", wantCode: financial.CodeOpexOffice, wantStatus: financial.RowStatusNormal},
		{rowID: "saas-8", wantUnknown: true},
		{rowID: "saas-9", wantUnknown: true},
		{rowID: "saas-10", wantStatus: financial.RowStatusTotal},
	}

	byID := make(map[string]financial.RawLineItem)
	for _, rows := range all {
		for _, r := range rows {
			byID[r.ID] = r
		}
	}

	for _, exp := range expectations {
		row, ok := byID[exp.rowID]
		if !ok {
			t.Fatalf("fixture row %q not found", exp.rowID)
		}
		result := Classify(row, cfg)

		if exp.wantUnknown {
			if !result.IsUnknown() {
				t.Errorf("row %s (%q): expected UNKNOWN, got code=%v source=%v", exp.rowID, row.Label, result.Code, result.Source)
			}
			continue
		}

		if exp.wantStatus != "" && result.Status != exp.wantStatus {
			t.Errorf("row %s (%q): Status = %v, want %v", exp.rowID, row.Label, result.Status, exp.wantStatus)
		}
		if exp.wantCode != "" && result.Code != exp.wantCode {
			t.Errorf("row %s (%q): Code = %v, want %v (source=%v reason=%q)", exp.rowID, row.Label, result.Code, exp.wantCode, result.Source, result.Reason)
		}
	}
}

// TestFixtures_BusinessRowsBatchCoversEveryRow verifies ClassifyBatch
// produces exactly one Result per input row, in order, across every
// business-type fixture group combined.
func TestFixtures_BusinessRowsBatchCoversEveryRow(t *testing.T) {
	fixtures := loadBusinessFixtures(t)
	cfg := fixtureConfig()

	var all []financial.RawLineItem
	all = append(all, fixtures.HVAC...)
	all = append(all, fixtures.Agency...)
	all = append(all, fixtures.Manufact...)
	all = append(all, fixtures.SaaS...)

	results := ClassifyBatch(all, cfg)
	if len(results) != len(all) {
		t.Fatalf("got %d results, want %d", len(results), len(all))
	}
	for i, r := range results {
		if r.RowID != all[i].ID {
			t.Errorf("results[%d].RowID = %q, want %q", i, r.RowID, all[i].ID)
		}
	}
}

// TestFixtures_RawLineItemsDecode is a basic sanity check that the fixture
// file is well-formed and every row has the minimum required fields, since
// it otherwise would only be exercised indirectly.
func TestFixtures_RawLineItemsDecode(t *testing.T) {
	fixtures := loadBusinessFixtures(t)
	groups := [][]financial.RawLineItem{fixtures.HVAC, fixtures.Agency, fixtures.Manufact, fixtures.SaaS}
	total := 0
	for _, group := range groups {
		for _, item := range group {
			total++
			if item.ID == "" {
				t.Errorf("row missing id: %+v", item)
			}
			if item.Label == "" {
				t.Errorf("row %s missing label", item.ID)
			}
			if item.StatementType == "" {
				t.Errorf("row %s missing statement_type", item.ID)
			}
		}
	}
	if total == 0 {
		t.Fatal("expected at least one row across all business-type fixtures")
	}
}
