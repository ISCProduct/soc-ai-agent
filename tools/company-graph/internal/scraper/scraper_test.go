package scraper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFallbackNodePreservesRelationFields(t *testing.T) {
	raw := &RawCompany{
		RawName:              "テスト株式会社",
		SourceURL:            "https://example.com",
		BusinessDescription:  "ITサービス事業",
		RelatedCompaniesText: "親会社A、子会社B",
		BusinessPartnersText: "取引先X、取引先Y",
	}

	node := fallbackNode(raw)

	if node.BusinessDescription != raw.BusinessDescription {
		t.Errorf("BusinessDescription: got %q, want %q", node.BusinessDescription, raw.BusinessDescription)
	}
	if node.RawRelatedCompaniesText != raw.RelatedCompaniesText {
		t.Errorf("RawRelatedCompaniesText: got %q, want %q", node.RawRelatedCompaniesText, raw.RelatedCompaniesText)
	}
	if node.RawBusinessPartnersText != raw.BusinessPartnersText {
		t.Errorf("RawBusinessPartnersText: got %q, want %q", node.RawBusinessPartnersText, raw.BusinessPartnersText)
	}
	if len(node.RelatedCompanies) != 0 {
		t.Errorf("fallback node should have no resolved RelatedCompanies, got %d", len(node.RelatedCompanies))
	}
}

func TestResolveCompanyRefsParseAndMatch(t *testing.T) {
	// モックgBizINFOサーバー
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		var records []GBizRecord
		switch name {
		case "株式会社A":
			records = []GBizRecord{{CorporateNumber: "1234567890123", Name: "株式会社A"}}
		case "株式会社B":
			records = []GBizRecord{{CorporateNumber: "9876543210987", Name: "株式会社B"}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"hojin-infos": records})
	}))
	defer srv.Close()

	client := NewGBizClient(srv.URL, "")

	refs := client.ResolveCompanyRefs(context.Background(), "株式会社A、株式会社B、")

	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Name != "株式会社A" {
		t.Errorf("refs[0].Name: got %q, want %q", refs[0].Name, "株式会社A")
	}
	if refs[0].CorporateNumber != "1234567890123" {
		t.Errorf("refs[0].CorporateNumber: got %q, want %q", refs[0].CorporateNumber, "1234567890123")
	}
	if refs[1].CorporateNumber != "9876543210987" {
		t.Errorf("refs[1].CorporateNumber: got %q, want %q", refs[1].CorporateNumber, "9876543210987")
	}
}

func TestResolveCompanyRefsEmptyText(t *testing.T) {
	client := NewGBizClient("", "")
	refs := client.ResolveCompanyRefs(context.Background(), "")
	if refs != nil {
		t.Errorf("expected nil for empty text, got %v", refs)
	}
}

func TestMergeRelations(t *testing.T) {
	dst := &CompanyNode{
		BusinessDescription: "",
		RelatedCompanies:    nil,
		BusinessPartners:    nil,
	}
	src := &CompanyNode{
		BusinessDescription: "事業内容テスト",
		RelatedCompanies:    []CompanyRef{{Name: "関連会社A", CorporateNumber: "111"}},
		BusinessPartners:    []CompanyRef{{Name: "取引先X", CorporateNumber: "222"}},
	}

	mergeRelations(dst, src)

	if dst.BusinessDescription != "事業内容テスト" {
		t.Errorf("BusinessDescription not merged: %q", dst.BusinessDescription)
	}
	if len(dst.RelatedCompanies) != 1 || dst.RelatedCompanies[0].CorporateNumber != "111" {
		t.Errorf("RelatedCompanies not merged correctly: %v", dst.RelatedCompanies)
	}
	if len(dst.BusinessPartners) != 1 || dst.BusinessPartners[0].CorporateNumber != "222" {
		t.Errorf("BusinessPartners not merged correctly: %v", dst.BusinessPartners)
	}

	// 既存データがある場合は上書きしない
	mergeRelations(dst, &CompanyNode{
		BusinessDescription: "別の事業内容",
		RelatedCompanies:    []CompanyRef{{Name: "別の関連会社"}},
	})
	if dst.BusinessDescription != "事業内容テスト" {
		t.Errorf("mergeRelations should not overwrite existing data: %q", dst.BusinessDescription)
	}
}

func TestGBizMatchSetsRelationFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records := []GBizRecord{{
			CorporateNumber: "1234567890123",
			Name:            "テスト株式会社",
		}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"hojin-infos": records})
	}))
	defer srv.Close()

	client := NewGBizClient(srv.URL, "")
	raw := &RawCompany{
		RawName:              "テスト株式会社",
		SourceURL:            "https://example.com",
		BusinessDescription:  "IT事業",
		RelatedCompaniesText: "テスト株式会社",
		BusinessPartnersText: "テスト株式会社",
	}

	node, err := client.Match(context.Background(), raw, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if node.BusinessDescription != "IT事業" {
		t.Errorf("BusinessDescription: got %q", node.BusinessDescription)
	}
	if node.RawRelatedCompaniesText != "テスト株式会社" {
		t.Errorf("RawRelatedCompaniesText: got %q", node.RawRelatedCompaniesText)
	}
	if len(node.RelatedCompanies) == 0 {
		t.Error("expected RelatedCompanies to be resolved")
	}
}
