package company

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/houjinbangou"
)

func TestNormalizeReading(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"全角カタカナはそのまま", "トヨタジドウシャ", "トヨタジドウシャ", true},
		{"前後の空白を除く", "  ソニーグループ  ", "ソニーグループ", true},
		{"半角カタカナは全角にする", "ﾄﾖﾀｼﾞﾄﾞｳｼｬ", "トヨタジドウシャ", true},
		{"ひらがなはカタカナにする", "とよたじどうしゃ", "トヨタジドウシャ", true},
		{"長音・中黒・空白は許す", "ヤフー・ジャパン コーポレーション", "ヤフー・ジャパン コーポレーション", true},
		{"空は不可", "", "", false},
		{"漢字を含むものは不可（読みではない）", "トヨタ自動車", "", false},
		{"英字を含むものは不可", "freeeカブシキガイシャ", "", false},
		{"記号だけは不可", "・ー", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeReading(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

type fakeFinder struct {
	byNumber    map[string]*houjinbangou.Corporation
	byName      *houjinbangou.Corporation
	numberErr   error
	resolveErr  error
	numberCalls int
	nameCalls   int
}

func (f *fakeFinder) FindByNumber(_ context.Context, n string) (*houjinbangou.Corporation, error) {
	f.numberCalls++
	return f.byNumber[n], f.numberErr
}
func (f *fakeFinder) ResolveCorporateNumber(context.Context, string, string) (*houjinbangou.Corporation, error) {
	f.nameCalls++
	return f.byName, f.resolveErr
}

func TestResolveNameReading(t *testing.T) {
	toyota := &houjinbangou.Corporation{CorporateNumber: "1180301018771", Name: "トヨタ自動車株式会社", Furigana: "トヨタジドウシャ"}
	tests := []struct {
		name       string
		company    models.Company
		finder     *fakeFinder
		wantMethod string
		wantRead   string
		wantWhy    string
	}{
		{"法人番号があれば、番号で引く", models.Company{Name: "トヨタ自動車株式会社", CorporateNumber: "1180301018771"},
			&fakeFinder{byNumber: map[string]*houjinbangou.Corporation{"1180301018771": toyota}}, MethodCorporateNumber, "トヨタジドウシャ", ""},
		{"法人番号が無ければ、商号と所在地で特定する", models.Company{Name: "トヨタ自動車", Location: "愛知県豊田市"},
			&fakeFinder{byName: toyota}, MethodNameMatch, "トヨタジドウシャ", ""},
		{"商号と一致しない法人番号は採用しない（番号の入力誤り）", models.Company{Name: "ソニーグループ株式会社", CorporateNumber: "1180301018771"},
			&fakeFinder{byNumber: map[string]*houjinbangou.Corporation{"1180301018771": toyota}}, "", "", ReasonNameMismatch},
		{"法人番号が見つからない", models.Company{Name: "トヨタ自動車株式会社", CorporateNumber: "9999999999999"},
			&fakeFinder{}, "", "", ReasonNotFound},
		{"閉鎖された法人は採用しない", models.Company{Name: "トヨタ自動車株式会社", CorporateNumber: "1180301018771"},
			&fakeFinder{byNumber: map[string]*houjinbangou.Corporation{"1180301018771": {Name: "トヨタ自動車株式会社", Furigana: "トヨタジドウシャ", CloseDate: "2020-01-01"}}}, "", "", ReasonClosed},
		{"一意に特定できなければ、空のまま", models.Company{Name: "サンプル株式会社"}, &fakeFinder{}, "", "", ReasonNotResolved},
		{"フリガナが読みとして使えない形なら採用しない", models.Company{Name: "トヨタ自動車株式会社", CorporateNumber: "1180301018771"},
			&fakeFinder{byNumber: map[string]*houjinbangou.Corporation{"1180301018771": {Name: "トヨタ自動車株式会社", Furigana: ""}}}, "", "", ReasonNoReading},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveNameReading(context.Background(), tt.finder, &tt.company)
			if err != nil {
				t.Fatal(err)
			}
			if got.Method != tt.wantMethod || got.Reading != tt.wantRead || got.Reason != tt.wantWhy {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestResolveNameReading_ErrorsArePropagated(t *testing.T) {
	boom := errors.New("api down")
	if _, err := ResolveNameReading(context.Background(), &fakeFinder{numberErr: boom}, &models.Company{Name: "A", CorporateNumber: "1"}); !errors.Is(err, boom) {
		t.Fatalf("法人番号の取得失敗は、「見つからない」と区別して返す: %v", err)
	}
	if _, err := ResolveNameReading(context.Background(), &fakeFinder{resolveErr: boom}, &models.Company{Name: "A"}); !errors.Is(err, boom) {
		t.Fatalf("商号での特定の失敗は、「特定できない」と区別して返す: %v", err)
	}
}
