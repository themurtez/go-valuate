package classification

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestBuildAliasIndex_SingleLayer(t *testing.T) {
	layers := []AliasLayer{
		{Name: "global", Aliases: []Alias{
			{Label: "Advertising & Promotion", Code: financial.CodeOpexMarketing},
		}},
	}
	idx := buildAliasIndex(layers)

	got, ok := idx["advertising and promotion"]
	if !ok {
		t.Fatal("expected alias to be indexed")
	}
	if got.code != financial.CodeOpexMarketing {
		t.Errorf("code = %v, want %v", got.code, financial.CodeOpexMarketing)
	}
	if got.layerName != "global" {
		t.Errorf("layerName = %v, want global", got.layerName)
	}
}

func TestBuildAliasIndex_HigherLayerOverridesLower(t *testing.T) {
	layers := []AliasLayer{
		{Name: "global", Aliases: []Alias{
			{Label: "Field Installers", Code: financial.CodeOpexOther},
		}},
		{Name: "account", Aliases: []Alias{
			{Label: "Field Installers", Code: financial.CodeCogsDirectLabor},
		}},
		{Name: "client", Aliases: []Alias{
			{Label: "Field Installers", Code: financial.CodeOpexPayroll},
		}},
		{Name: "valuation", Aliases: []Alias{
			{Label: "Field Installers", Code: financial.CodeOpexOwnerComp},
		}},
	}

	idx := buildAliasIndex(layers)
	got := idx["field installers"]
	if got.code != financial.CodeOpexOwnerComp {
		t.Errorf("code = %v, want %v (valuation should win over client > account > global)", got.code, financial.CodeOpexOwnerComp)
	}
	if got.layerName != "valuation" {
		t.Errorf("layerName = %v, want valuation", got.layerName)
	}
}

func TestBuildAliasIndex_ClientOverridesAccountAndGlobalWhenNoValuation(t *testing.T) {
	layers := []AliasLayer{
		{Name: "global", Aliases: []Alias{{Label: "Shop Supplies", Code: financial.CodeOpexOther}}},
		{Name: "account", Aliases: []Alias{{Label: "Shop Supplies", Code: financial.CodeOpexOffice}}},
		{Name: "client", Aliases: []Alias{{Label: "Shop Supplies", Code: financial.CodeCogsMaterial}}},
		{Name: "valuation", Aliases: nil},
	}

	idx := buildAliasIndex(layers)
	got := idx["shop supplies"]
	if got.code != financial.CodeCogsMaterial {
		t.Errorf("code = %v, want %v (client should win over account and global)", got.code, financial.CodeCogsMaterial)
	}
	if got.layerName != "client" {
		t.Errorf("layerName = %v, want client", got.layerName)
	}
}

func TestBuildAliasIndex_AccountOverridesGlobalWhenNoClientOrValuation(t *testing.T) {
	layers := []AliasLayer{
		{Name: "global", Aliases: []Alias{{Label: "Merchant Fees", Code: financial.CodeOpexOther}}},
		{Name: "account", Aliases: []Alias{{Label: "Merchant Fees", Code: financial.CodeOpexProfessionalFees}}},
		{Name: "client", Aliases: nil},
		{Name: "valuation", Aliases: nil},
	}

	idx := buildAliasIndex(layers)
	got := idx["merchant fees"]
	if got.code != financial.CodeOpexProfessionalFees {
		t.Errorf("code = %v, want %v (account should win over global)", got.code, financial.CodeOpexProfessionalFees)
	}
	if got.layerName != "account" {
		t.Errorf("layerName = %v, want account", got.layerName)
	}
}

func TestBuildAliasIndex_GlobalUsedWhenNoOverride(t *testing.T) {
	layers := []AliasLayer{
		{Name: "global", Aliases: []Alias{{Label: "Bank Charges", Code: financial.CodeOpexProfessionalFees}}},
		{Name: "account", Aliases: nil},
		{Name: "client", Aliases: nil},
		{Name: "valuation", Aliases: nil},
	}

	idx := buildAliasIndex(layers)
	got := idx["bank charges"]
	if got.code != financial.CodeOpexProfessionalFees {
		t.Errorf("code = %v, want %v (global should be used when nothing overrides it)", got.code, financial.CodeOpexProfessionalFees)
	}
	if got.layerName != "global" {
		t.Errorf("layerName = %v, want global", got.layerName)
	}
}

func TestBuildAliasIndex_DifferentLabelsDoNotInterfere(t *testing.T) {
	layers := []AliasLayer{
		{Name: "global", Aliases: []Alias{
			{Label: "Advertising", Code: financial.CodeOpexMarketing},
			{Label: "Bank Charges", Code: financial.CodeOpexProfessionalFees},
		}},
		{Name: "client", Aliases: []Alias{
			{Label: "Bank Charges", Code: financial.CodeOpexOther},
		}},
	}

	idx := buildAliasIndex(layers)
	if idx["advertising"].code != financial.CodeOpexMarketing {
		t.Errorf("advertising should be unaffected by client override of a different label")
	}
	if idx["bank charges"].code != financial.CodeOpexOther {
		t.Errorf("bank charges should reflect the client override")
	}
}
