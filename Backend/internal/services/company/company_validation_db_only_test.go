package company

import (
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// TestValidateFromDB は Web検索に一切落ちないことを固定する（#1124）。
//
// 履歴書レビューはこの経路だけを使う。Web検索は1コールあたり約3万入力トークンかかる一方、
// 返るのは真偽値だけで結果を DB に保存もしないため、その後の企業brief取得にも当たらず
// 「一番高い経路が一番中身の薄いレビューを返す」状態になっていた。
func TestValidateFromDB(t *testing.T) {
	tests := []struct {
		name          string
		repo          *fakeCompanyLookup
		query         string
		wantNil       bool
		wantExists    bool
		wantCanonical string
		wantSource    string
	}{
		{
			name: "完全一致はDBで確定",
			repo: &fakeCompanyLookup{exact: map[string]*models.Company{
				"株式会社実在": {ID: 10, Name: "株式会社実在"},
			}},
			query: "株式会社実在", wantExists: true, wantCanonical: "株式会社実在", wantSource: "db",
		},
		{
			name: "部分一致が1件ならDBで確定",
			repo: &fakeCompanyLookup{
				exact:   map[string]*models.Company{},
				partial: []models.CompanyName{{ID: 3, Name: "株式会社ユニーク"}},
			},
			query: "ユニーク", wantExists: true, wantCanonical: "株式会社ユニーク", wantSource: "db",
		},
		{
			// ここで Web検索に落ちていた。nil を返して呼び出し側に判断させる
			name:    "DBに無ければnilを返す（Web検索しない）",
			repo:    &fakeCompanyLookup{exact: map[string]*models.Company{}},
			query:   "架空株式会社XYZ999",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// openaiClient を nil にしておくことで、万一Web検索へ進んだら
			// Source が "web_search" になり検出できる
			svc := NewCompanyValidationService(tt.repo, nil)
			got, err := svc.ValidateFromDB(tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Fatalf("DBで判定できない場合は nil を返すべき: %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("結果が nil")
			}
			if got.Exists != tt.wantExists || got.CanonicalName != tt.wantCanonical || got.Source != tt.wantSource {
				t.Errorf("= %+v, want exists=%v canonical=%q source=%q",
					got, tt.wantExists, tt.wantCanonical, tt.wantSource)
			}
		})
	}
}

func TestValidateFromDB_RejectsEmpty(t *testing.T) {
	svc := NewCompanyValidationService(nil, nil)
	_, err := svc.ValidateFromDB("  ")
	var ve *shared.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

// TestValidateFromDB_UsesCache は2回目がキャッシュから返ることを確認する。
func TestValidateFromDB_UsesCache(t *testing.T) {
	repo := &fakeCompanyLookup{exact: map[string]*models.Company{
		"株式会社実在": {ID: 10, Name: "株式会社実在"},
	}}
	svc := NewCompanyValidationService(repo, nil)
	if _, err := svc.ValidateFromDB("株式会社実在"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ValidateFromDB("株式会社実在")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.FromCache || got.Source != "cache" {
		t.Fatalf("expected cache hit: %+v", got)
	}
}
