package teacher

import (
	"testing"

	"Backend/domain/valueobject"
	"Backend/internal/models"
	"Backend/internal/repositories"
)

func TestRankCategories(t *testing.T) {
	tests := []struct {
		name   string
		scores map[string]float64
		limit  int
		want   []string
	}{
		{
			name:   "スコアの高い順に返す",
			scores: map[string]float64{"技術志向": 90, "安定志向": 40, "成長志向": 70},
			limit:  3,
			want:   []string{"技術志向", "成長志向", "安定志向"},
		},
		{
			name:   "limit で打ち切る",
			scores: map[string]float64{"技術志向": 90, "安定志向": 40, "成長志向": 70},
			limit:  2,
			want:   []string{"技術志向", "成長志向"},
		},
		{
			// 同点を map の反復順に任せると同じ生徒でも表示が変わる。
			// 正典の並び順で決めることで安定させる。
			name:   "同点は正典の並び順で決まる",
			scores: map[string]float64{"コミュニケーション力": 80, "技術志向": 80, "チームワーク志向": 80},
			limit:  3,
			want:   []string{"技術志向", "チームワーク志向", "コミュニケーション力"},
		},
		{name: "スコアなしは空", scores: map[string]float64{}, limit: 3, want: nil},
		{name: "limit 0 は空", scores: map[string]float64{"技術志向": 90}, limit: 0, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RankCategories(tt.scores, tt.limit)
			if len(got) != len(tt.want) {
				t.Fatalf("件数 = %d, want %d (%+v)", len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				if got[i].Category != w {
					t.Errorf("[%d] = %q, want %q (全体 %+v)", i, got[i].Category, w, got)
				}
			}
		})
	}
}

// 同じ入力で何度呼んでも同じ順序になること。
func TestRankCategories_IsStable(t *testing.T) {
	scores := map[string]float64{}
	for _, c := range valueobject.AllWeightCategories() {
		scores[string(c)] = 50 // 全部同点にして順序決定ロジックだけを見る
	}
	first := RankCategories(scores, 3)
	for i := 0; i < 30; i++ {
		got := RankCategories(scores, 3)
		for j := range first {
			if got[j].Category != first[j].Category {
				t.Fatalf("順序が変わる: %+v と %+v", first, got)
			}
		}
	}
}

func TestTypeLabelForCategory(t *testing.T) {
	// 正典10種すべてにラベルがあること。抜けると「傾向データ不足」になってしまう。
	for _, c := range valueobject.AllWeightCategories() {
		if got := TypeLabelForCategory(string(c)); got == "傾向データ不足" {
			t.Errorf("%q にタイプ名が定義されていない", c)
		}
	}
	if got := TypeLabelForCategory("未知カテゴリ"); got != "傾向データ不足" {
		t.Errorf("未知カテゴリ = %q, want 傾向データ不足", got)
	}
}

func TestRankIndustries(t *testing.T) {
	industries := []repositories.IndustryOption{
		{ID: 1, Name: "情報通信業"},
		{ID: 2, Name: "金融・保険業"},
	}
	// 業界1は技術重視、業界2は安定重視。
	profiles := map[uint]*models.IndustryWeightProfile{
		1: {IndustryID: 1, TechnicalOrientation: 95, StabilityOrientation: 20,
			TeamworkOrientation: 50, LeadershipOrientation: 50, CreativityOrientation: 50,
			GrowthOrientation: 50, WorkLifeBalance: 50, ChallengeSeeking: 50,
			DetailOrientation: 50, CommunicationSkill: 50},
		2: {IndustryID: 2, TechnicalOrientation: 20, StabilityOrientation: 95,
			TeamworkOrientation: 50, LeadershipOrientation: 50, CreativityOrientation: 50,
			GrowthOrientation: 50, WorkLifeBalance: 50, ChallengeSeeking: 50,
			DetailOrientation: 50, CommunicationSkill: 50},
	}

	t.Run("技術が高い生徒は技術重視の業界が上位", func(t *testing.T) {
		got := RankIndustries(map[string]float64{"技術志向": 95, "安定志向": 20}, industries, profiles, 2)
		if len(got) != 2 {
			t.Fatalf("件数 = %d", len(got))
		}
		if got[0].IndustryID != 1 {
			t.Errorf("1位 = %d(%s), want 1(情報通信業)", got[0].IndustryID, got[0].IndustryName)
		}
	})

	t.Run("安定が高い生徒は安定重視の業界が上位", func(t *testing.T) {
		got := RankIndustries(map[string]float64{"技術志向": 20, "安定志向": 95}, industries, profiles, 2)
		if got[0].IndustryID != 2 {
			t.Errorf("1位 = %d(%s), want 2(金融・保険業)", got[0].IndustryID, got[0].IndustryName)
		}
	})

	// PRD 非機能要件: 未設定業界は中立50にフォールバックしエラーにならない。
	t.Run("プロファイル未設定でも中立値で計算されエラーにならない", func(t *testing.T) {
		got := RankIndustries(map[string]float64{"技術志向": 95}, industries, map[uint]*models.IndustryWeightProfile{}, 2)
		if len(got) != 2 {
			t.Fatalf("件数 = %d, want 2", len(got))
		}
		// 全業界が同じ中立プロファイルなのでスコアは同値。IDの昇順で安定すること。
		if got[0].IndustryID != 1 || got[1].IndustryID != 2 {
			t.Errorf("同点時の順序が不安定: %+v", got)
		}
	})

	t.Run("limit で打ち切る", func(t *testing.T) {
		if got := RankIndustries(map[string]float64{"技術志向": 95}, industries, profiles, 1); len(got) != 1 {
			t.Errorf("件数 = %d, want 1", len(got))
		}
	})
}

func TestBuildTendency(t *testing.T) {
	industries := []repositories.IndustryOption{{ID: 1, Name: "情報通信業"}}
	profiles := map[uint]*models.IndustryWeightProfile{}

	// PRD 境界値: スコア未計測の生徒は「分析データ不足」になりエラーにならない。
	t.Run("スコアなしはデータ不足として返る", func(t *testing.T) {
		got := BuildTendency(1, "山田太郎", "a@example.com", map[string]float64{}, industries, profiles)
		if got.DataAvailable {
			t.Error("DataAvailable = true, want false")
		}
		if got.TypeLabel != "分析データ不足" {
			t.Errorf("TypeLabel = %q", got.TypeLabel)
		}
		if len(got.TopCategories) != 0 || len(got.SuitedIndustries) != 0 {
			t.Error("データ不足なのにタイプや業界が入っている")
		}
	})

	// 実データに「全カテゴリ score=0」の生徒が存在する。
	// そのまま通すと確信ありげなタイプ名が教員に出る（PRD が避けたい断定）。
	t.Run("全カテゴリ0点はデータ不足として返る", func(t *testing.T) {
		got := BuildTendency(1, "山田太郎", "a@example.com",
			map[string]float64{"技術志向": 0, "チームワーク志向": 0, "リーダーシップ志向": 0}, industries, profiles)
		if got.DataAvailable {
			t.Error("全カテゴリ0点なのに DataAvailable = true")
		}
		if got.TypeLabel != "分析データ不足" {
			t.Errorf("TypeLabel = %q", got.TypeLabel)
		}
	})

	// 計測カテゴリが少なすぎると、残りを中立50で埋めた業界TOP3が
	// 表示のほとんどを占めてノイズになる。
	t.Run("計測カテゴリが少なすぎるとデータ不足", func(t *testing.T) {
		got := BuildTendency(1, "山田太郎", "a@example.com",
			map[string]float64{"技術志向": 90, "成長志向": 70}, industries, profiles)
		if got.DataAvailable {
			t.Error("2カテゴリしか無いのに DataAvailable = true")
		}
	})

	t.Run("スコアありはタイプと業界が入る", func(t *testing.T) {
		got := BuildTendency(1, "山田太郎", "a@example.com",
			map[string]float64{"技術志向": 90, "成長志向": 70, "チームワーク志向": 60}, industries, profiles)
		if !got.DataAvailable {
			t.Fatal("DataAvailable = false")
		}
		if got.TypeLabel != "技術探究タイプ" {
			t.Errorf("TypeLabel = %q, want 技術探究タイプ", got.TypeLabel)
		}
		if len(got.SuitedIndustries) == 0 {
			t.Error("業界が空")
		}
	})
}

// S5: 同点時のタイブレークを実効的に検証する。
// 入力を ID の降順にしておかないと、sort.SliceStable が入力順を保つため
// タイブレークを消しても気づけない。
func TestRankIndustries_TieBreakByIndustryID(t *testing.T) {
	// 入力はわざと ID 降順。プロファイル未設定なので全業界が同点になる。
	industries := []repositories.IndustryOption{
		{ID: 3, Name: "C"}, {ID: 1, Name: "A"}, {ID: 2, Name: "B"},
	}
	got := RankIndustries(map[string]float64{"技術志向": 80}, industries,
		map[uint]*models.IndustryWeightProfile{}, 3)

	want := []uint{1, 2, 3}
	for i, w := range want {
		if got[i].IndustryID != w {
			t.Fatalf("同点時がID昇順で安定していない: %+v", got)
		}
	}
}

// S6: 未評価カテゴリを中立50で埋めていることを検証する。
//
// これは「企業マッチングと同じ結果になること」というPRD非機能要件の要。
// 0 で埋めると企業側 scoredMatch と食い違い、業界適性だけがずれる。
func TestRankIndustries_MissingCategoryUsesNeutral(t *testing.T) {
	industries := []repositories.IndustryOption{{ID: 1, Name: "X"}}
	// 全軸50の業界に対し、生徒のスコアが1つも無い場合。
	// 中立50で埋めていれば全カテゴリが完全一致してスコアは最大に近づく。
	neutral := RankIndustries(map[string]float64{}, industries,
		map[uint]*models.IndustryWeightProfile{}, 1)
	// 明示的に全カテゴリ50を与えた場合と一致するはず。
	explicit := map[string]float64{}
	for _, c := range valueobject.AllWeightCategories() {
		explicit[string(c)] = 50
	}
	withAll := RankIndustries(explicit, industries,
		map[uint]*models.IndustryWeightProfile{}, 1)

	if neutral[0].Score != withAll[0].Score {
		t.Errorf("未評価カテゴリが中立50として扱われていない: %.4f != %.4f",
			neutral[0].Score, withAll[0].Score)
	}
}
