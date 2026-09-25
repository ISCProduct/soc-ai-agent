package company

import (
	"strings"
	"testing"

	"Backend/internal/models"
)

// TestBuildCompanyBrief_FieldSelection は brief に何を載せ、何を載せないかを固定する（#1124）。
//
// brief はレビュー用プロンプトへそのまま連結される。空欄のラベルが混ざると
// 「文化: 」のような空の見出しをモデルが埋めにかかるため、値が無い項目は
// 行ごと出さないことを保証する。
func TestBuildCompanyBrief_FieldSelection(t *testing.T) {
	tests := []struct {
		name    string
		company *models.Company
		want    []string
		notWant []string
	}{
		{
			name:    "nil企業は空文字",
			company: nil,
		},
		{
			name:    "名前だけでも成立する",
			company: &models.Company{Name: "株式会社テスト"},
			want:    []string{"企業名: 株式会社テスト"},
			notWant: []string{"業種:", "所在地:", "事業:", "文化:", "働き方:", "技術スタック:", "重視傾向:"},
		},
		{
			name: "空白だけの値は行ごと出さない",
			company: &models.Company{
				Name:      "株式会社テスト",
				Industry:  "   ",
				Location:  "\t\n",
				Culture:   " ",
				WorkStyle: "",
			},
			want:    []string{"企業名: 株式会社テスト"},
			notWant: []string{"業種:", "所在地:", "文化:", "働き方:"},
		},
		{
			name: "事業はMainBusinessを優先する",
			company: &models.Company{
				Name:         "株式会社テスト",
				MainBusiness: "BtoB SaaSの開発",
				Description:  "会社説明のフォールバック",
			},
			want:    []string{"事業: BtoB SaaSの開発"},
			notWant: []string{"会社説明のフォールバック"},
		},
		{
			name: "MainBusinessが空ならDescriptionへ落ちる",
			company: &models.Company{
				Name:        "株式会社テスト",
				Description: "会社説明のフォールバック",
			},
			want: []string{"事業: 会社説明のフォールバック"},
		},
		{
			name: "MainBusinessが空白だけでもDescriptionへ落ちる",
			company: &models.Company{
				Name:         "株式会社テスト",
				MainBusiness: "   ",
				Description:  "会社説明のフォールバック",
			},
			want: []string{"事業: 会社説明のフォールバック"},
		},
		{
			// TechStack は JSON 文字列で保存されており、未取得は "null" / "[]"。
			// そのまま載せると「技術スタック: null」がプロンプトに流れる。
			name:    "技術スタックのnullは載せない",
			company: &models.Company{Name: "株式会社テスト", TechStack: "null"},
			notWant: []string{"技術スタック:"},
		},
		{
			name:    "技術スタックの空配列は載せない",
			company: &models.Company{Name: "株式会社テスト", TechStack: "[]"},
			notWant: []string{"技術スタック:"},
		},
		{
			name:    "技術スタックに中身があれば載せる",
			company: &models.Company{Name: "株式会社テスト", TechStack: `["Go","TypeScript"]`},
			want:    []string{"技術スタック:", "Go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCompanyBrief(tt.company, nil)
			if tt.company == nil {
				if got != "" {
					t.Fatalf("nil企業で空文字を返さない: %q", got)
				}
				return
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("%q が含まれない:\n%s", w, got)
				}
			}
			for _, ng := range tt.notWant {
				if strings.Contains(got, ng) {
					t.Errorf("%q が含まれている:\n%s", ng, got)
				}
			}
			// 値の無いラベルが残ると、モデルがそこを埋めにかかる。
			for _, line := range strings.Split(got, "\n") {
				if strings.HasSuffix(strings.TrimRight(line, " "), ":") {
					t.Errorf("値の無いラベル行が残っている: %q\n%s", line, got)
				}
			}
		})
	}
}

// 長い値を切り詰めること。プロンプトに丸ごと流すとレビュー本体の指示が薄まる。
func TestBuildCompanyBrief_TrimsLongValues(t *testing.T) {
	long := strings.Repeat("あ", 500)
	got := BuildCompanyBrief(&models.Company{
		Name:         "株式会社テスト",
		MainBusiness: long,
		Culture:      long,
		TechStack:    long,
	}, nil)

	if strings.Contains(got, long) {
		t.Error("長い値が切り詰められていない")
	}
	if !strings.Contains(got, "…") {
		t.Errorf("切り詰めの印が無い:\n%s", got)
	}
	// 事業200 + 文化120 + 技術120 + ラベル類。丸ごと3本入ると1500文字を超える。
	if n := len([]rune(got)); n > 600 {
		t.Errorf("brief が長すぎる: %d文字", n)
	}
}

// TestBuildCompanyBrief_TopWeights は重視傾向の選び方を固定する（#1124）。
func TestBuildCompanyBrief_TopWeights(t *testing.T) {
	comp := &models.Company{Name: "株式会社テスト"}

	t.Run("上位3件までしか出さない", func(t *testing.T) {
		got := BuildCompanyBrief(comp, &models.CompanyWeightProfile{
			TechnicalOrientation:  95,
			GrowthOrientation:     90,
			LeadershipOrientation: 85,
			CreativityOrientation: 80,
			TeamworkOrientation:   75,
		})
		line := weightLine(t, got)
		if n := strings.Count(line, "("); n != 3 {
			t.Errorf("重視傾向の件数 = %d, want 3:\n%s", n, line)
		}
		if !strings.Contains(line, "技術志向(95)") || !strings.Contains(line, "成長志向(90)") {
			t.Errorf("上位が入っていない:\n%s", line)
		}
		if strings.Contains(line, "チームワーク(75)") {
			t.Errorf("4件目以降が混ざっている:\n%s", line)
		}
	})

	t.Run("50以下は重視とみなさない", func(t *testing.T) {
		got := BuildCompanyBrief(comp, &models.CompanyWeightProfile{
			TechnicalOrientation: 51,
			GrowthOrientation:    50,
			TeamworkOrientation:  49,
		})
		line := weightLine(t, got)
		if !strings.Contains(line, "技術志向(51)") {
			t.Errorf("51 が落ちている:\n%s", line)
		}
		if strings.Contains(line, "(50)") || strings.Contains(line, "(49)") {
			t.Errorf("50以下が載っている:\n%s", line)
		}
	})

	t.Run("全て50以下なら重視傾向の行ごと出さない", func(t *testing.T) {
		got := BuildCompanyBrief(comp, &models.CompanyWeightProfile{
			TechnicalOrientation: 50,
			GrowthOrientation:    10,
		})
		if strings.Contains(got, "重視傾向") {
			t.Errorf("空の重視傾向行が出ている:\n%s", got)
		}
	})

	t.Run("プロファイル未注入でも落ちない", func(t *testing.T) {
		if got := BuildCompanyBrief(comp, nil); got == "" {
			t.Error("brief が空")
		}
	})
}

func weightLine(t *testing.T, brief string) string {
	t.Helper()
	for _, line := range strings.Split(brief, "\n") {
		if strings.HasPrefix(line, "重視傾向:") {
			return line
		}
	}
	t.Fatalf("重視傾向の行が無い:\n%s", brief)
	return ""
}
