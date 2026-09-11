package company

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeCompanyLookup struct {
	exact   map[string]*models.Company
	partial []models.CompanyName
}

func (f *fakeCompanyLookup) FindByName(name string) (*models.Company, error) {
	if c, ok := f.exact[name]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeCompanyLookup) FindAllActiveNames(q string) ([]models.CompanyName, error) {
	qKey := normalizeCompanyKey(q)
	var out []models.CompanyName
	for _, n := range f.partial {
		if qKey == "" || strings.Contains(normalizeCompanyKey(n.Name), qKey) || strings.Contains(n.Name, q) {
			out = append(out, n)
		}
	}
	return out, nil
}

func TestNormalizeCompanyKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"株式会社テスト", "テスト"},
		{"  Test Corp  ", "testcorp"},
		{"（株）サンプル", "サンプル"},
	}
	for _, tc := range cases {
		got := normalizeCompanyKey(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeCompanyKey(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseValidationJSON(t *testing.T) {
	raw := `説明文 {"exists":true,"canonical_name":"株式会社サンプル","evidence_urls":["https://example.com"],"confidence":"high","description":"IT"} 終わり`
	got, err := parseValidationJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Exists || got.CanonicalName != "株式会社サンプル" {
		t.Fatalf("unexpected parse result: %+v", got)
	}
	if len(got.EvidenceURLs) != 1 {
		t.Fatalf("evidence urls: %v", got.EvidenceURLs)
	}
}

func TestCompanyValidationService_DBHitSkipsWebSearch(t *testing.T) {
	repo := &fakeCompanyLookup{
		exact: map[string]*models.Company{
			"株式会社実在": {ID: 10, Name: "株式会社実在", WebsiteURL: "https://real.example", Description: "実在企業"},
		},
	}
	svc := NewCompanyValidationService(repo, nil)
	result, err := svc.Validate(context.Background(), "株式会社実在")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Exists || result.Source != "db" || result.CompanyID == nil || *result.CompanyID != 10 {
		t.Fatalf("unexpected result: %+v", result)
	}

	result2, err := svc.Validate(context.Background(), "株式会社実在")
	if err != nil {
		t.Fatal(err)
	}
	if !result2.FromCache || result2.Source != "cache" {
		t.Fatalf("expected cache hit: %+v", result2)
	}
}

func TestCompanyValidationService_PartialSingleDBHit(t *testing.T) {
	repo := &fakeCompanyLookup{
		exact: map[string]*models.Company{},
		partial: []models.CompanyName{
			{ID: 3, Name: "株式会社ユニーク"},
		},
	}
	svc := NewCompanyValidationService(repo, nil)
	result, err := svc.Validate(context.Background(), "ユニーク")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Exists || result.Source != "db" || result.CanonicalName != "株式会社ユニーク" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCompanyValidationService_RejectsEmpty(t *testing.T) {
	svc := NewCompanyValidationService(nil, nil)
	_, err := svc.Validate(context.Background(), "  ")
	var ve *shared.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestCompanyValidationService_NoOpenAIReturnsNotExists(t *testing.T) {
	svc := NewCompanyValidationService(&fakeCompanyLookup{exact: map[string]*models.Company{}}, nil)
	result, err := svc.Validate(context.Background(), "架空株式会社XYZ999")
	if err != nil {
		t.Fatal(err)
	}
	if result.Exists {
		t.Fatalf("expected not exists without openai: %+v", result)
	}
	if result.Source != "web_search" {
		t.Fatalf("source=%q", result.Source)
	}
}

func TestCompanyValidationService_SearchCandidatesWebFailureReturnsEmpty(t *testing.T) {
	svc := NewCompanyValidationService(&fakeCompanyLookup{exact: map[string]*models.Company{}}, nil)
	results, err := svc.SearchCandidates(context.Background(), "未知企業ABC", true)
	if err != nil {
		t.Fatalf("WEB補完失敗でもエラーにしない: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("候補なしを期待: %+v", results)
	}
}
