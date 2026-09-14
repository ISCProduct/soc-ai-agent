package matching

import (
	"strings"
	"testing"

	"Backend/domain/entity"
)

// TestBuildMatchReasonScoreWording は、マッチ度に応じて文面が切り替わることを固定する（#1124）。
//
// 飽和していた頃はスコアが91%未満に落ちなかったため「高い水準で一致しています」
// 「特におすすめです」の固定文で破綻しなかった。線形化でスコアが下がるように
// なったため、境界値をここで押さえる。
func TestBuildMatchReasonScoreWording(t *testing.T) {
	tests := []struct {
		name      string
		score     float64
		wantLevel string
		wantClose string
		notWant   []string
	}{
		{
			name:      "80以上は高い水準",
			score:     80,
			wantLevel: "高い水準で一致しています",
			wantClose: "特におすすめです",
		},
		{
			name:      "80直下は高い水準にしない",
			score:     79,
			wantLevel: "おおむね一致しています",
			wantClose: "強みを活かせる場面がある候補です",
			notWant:   []string{"高い水準", "特におすすめ"},
		},
		{
			name:      "60ちょうどはおおむね",
			score:     60,
			wantLevel: "おおむね一致しています",
			wantClose: "強みを活かせる場面がある候補です",
			notWant:   []string{"高い水準"},
		},
		{
			name:      "60直下は部分的",
			score:     59,
			wantLevel: "部分的に一致しています",
			wantClose: "志向の重なりが部分的です",
			notWant:   []string{"高い水準", "おおむね", "特におすすめ"},
		},
		{
			name:      "40ちょうどは部分的",
			score:     40,
			wantLevel: "部分的に一致しています",
			wantClose: "志向の重なりが部分的です",
		},
		{
			name:      "40直下はあまり一致していない",
			score:     39,
			wantLevel: "あまり一致していません",
			wantClose: "志向の重なりが小さい候補です",
			notWant:   []string{"高い水準", "おおむね", "部分的に一致", "特におすすめ"},
		},
		{
			name:      "低スコアでもおすすめ文言を出さない",
			score:     12,
			wantLevel: "あまり一致していません",
			wantClose: "志向の重なりが小さい候補です",
			notWant:   []string{"特におすすめ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &entity.UserCompanyMatch{
				CompanyID:      1,
				Company:        &entity.Company{ID: 1, Name: "テスト株式会社", Industry: "IT"},
				MatchScore:     tt.score,
				TechnicalMatch: 70,
			}

			got := BuildMatchReason(match, nil)

			if !strings.Contains(got, tt.wantLevel) {
				t.Errorf("マッチ度%.0f%%の文面に %q が含まれていない:\n%s", tt.score, tt.wantLevel, got)
			}
			if !strings.Contains(got, tt.wantClose) {
				t.Errorf("マッチ度%.0f%%の締めに %q が含まれていない:\n%s", tt.score, tt.wantClose, got)
			}
			for _, ng := range tt.notWant {
				if strings.Contains(got, ng) {
					t.Errorf("マッチ度%.0f%%の文面に %q が混ざっている:\n%s", tt.score, ng, got)
				}
			}
			// 企業名は締めに必ず入る。差し込みが壊れたら気付けるようにする。
			if !strings.Contains(got, "テスト株式会社") {
				t.Errorf("企業名が文面に入っていない:\n%s", got)
			}
		})
	}
}

// TestBuildMatchReasonExcludesUnmeasuredAxes は、未計測（0%）の軸を
// 「一致度が高い軸」として挙げないことを固定する（#1124）。
func TestBuildMatchReasonExcludesUnmeasuredAxes(t *testing.T) {
	match := &entity.UserCompanyMatch{
		CompanyID:  1,
		Company:    &entity.Company{ID: 1, Name: "テスト株式会社", Industry: "IT"},
		MatchScore: 55,
		// 測れたのは2軸だけ。残り8軸は未計測で0のまま保存されている。
		TechnicalMatch: 80,
		TeamworkMatch:  30,
	}

	got := BuildMatchReason(match, nil)

	for _, label := range []string{
		"リーダーシップ", "創造性", "安定志向", "成長志向",
		"ワークライフバランス", "チャレンジ志向", "細部志向", "コミュニケーション力",
	} {
		if strings.Contains(got, label+"(0%)") {
			t.Errorf("未計測の軸 %s(0%%) が挙がっている:\n%s", label, got)
		}
	}
	if strings.Contains(got, "(0%)") {
		t.Errorf("0%%の軸が文面に混ざっている:\n%s", got)
	}
	if !strings.Contains(got, "技術志向(80%)") {
		t.Errorf("計測済みの軸が挙がっていない:\n%s", got)
	}
	if !strings.Contains(got, "チームワーク(30%)") {
		t.Errorf("計測済みの軸が挙がっていない:\n%s", got)
	}
}

// TestBuildMatchReasonAllAxesUnmeasured は、全軸が未計測のときに
// 個別軸を挙げずフォールバックすることを確認する。
func TestBuildMatchReasonAllAxesUnmeasured(t *testing.T) {
	match := &entity.UserCompanyMatch{
		CompanyID:  1,
		Company:    &entity.Company{ID: 1, Name: "テスト株式会社", Industry: "IT"},
		MatchScore: 0,
	}

	got := BuildMatchReason(match, nil)

	if !strings.Contains(got, "複数の評価軸") {
		t.Errorf("全軸未計測時のフォールバックが出ていない:\n%s", got)
	}
	if strings.Contains(got, "(0%)") {
		t.Errorf("0%%の軸が文面に混ざっている:\n%s", got)
	}
}
