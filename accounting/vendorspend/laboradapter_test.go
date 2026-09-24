package vendorspend_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestLaborAdapter_MapsContractorIDToSupplierID verifies the explicit
// contractorSupplierID mapping converts a ContractorLaborRecord into a
// SpendRecord under SpendTypeSubcontractor.
func TestLaborAdapter_MapsContractorIDToSupplierID(t *testing.T) {
	records := []labor.ContractorLaborRecord{
		{ID: "CL-1", ContractorID: "CONTRACTOR-A", Period: "2025-03", Date: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			Amount: 8000, Currency: "USD"},
	}
	mapping := map[string]string{"CONTRACTOR-A": "SUP-CONTRACTOR-A"}
	out := vendorspend.SpendRecordsFromContractorLabor(records, mapping)
	if len(out) != 1 {
		t.Fatalf("expected 1 record, got %d", len(out))
	}
	if out[0].SupplierID != "SUP-CONTRACTOR-A" {
		t.Errorf("SupplierID = %q, want SUP-CONTRACTOR-A", out[0].SupplierID)
	}
	if out[0].SpendType != vendorspend.SpendTypeSubcontractor {
		t.Errorf("SpendType = %v, want SpendTypeSubcontractor", out[0].SpendType)
	}
	if out[0].Amount != 8000 {
		t.Errorf("Amount = %v, want 8000", out[0].Amount)
	}
}

// TestLaborAdapter_SkipsUnmappedContractors verifies a ContractorID
// absent from the mapping is skipped, never guessed to equal its own
// ContractorID.
func TestLaborAdapter_SkipsUnmappedContractors(t *testing.T) {
	records := []labor.ContractorLaborRecord{
		{ID: "CL-1", ContractorID: "CONTRACTOR-UNMAPPED", Period: "2025-03", Date: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			Amount: 8000, Currency: "USD"},
	}
	out := vendorspend.SpendRecordsFromContractorLabor(records, map[string]string{})
	if len(out) != 0 {
		t.Errorf("expected 0 records for an unmapped contractor, got %d: %+v", len(out), out)
	}
}
