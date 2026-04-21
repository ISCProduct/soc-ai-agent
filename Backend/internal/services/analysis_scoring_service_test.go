package services

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestRuleBasedFutureAnalyzerScore(t *testing.T) {
	analyzer := NewRuleBasedFutureAnalyzer()

	tests := []struct {
		name         string
		messages     []models.ChatMessage
		wantScore    float64
		wantKeywords []string
	}{
		{
			name:      "空メッセージは0点",
			messages:  nil,
			wantScore: 0,
		},
		{
			name: "user以外は無視",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: "成長したいですか"},
			},
			wantScore: 0,
		},
		{
			name: "キーワードを重複なく検出してスコア化",
			messages: []models.ChatMessage{
				{Role: "user", Content: "将来は成長して、海外にも挑戦したいです"},
				{Role: "user", Content: "さらに成長したい"},
			},
			wantScore:    0.8, // 4 / threshold(5)
			wantKeywords: []string{"将来", "成長", "海外", "挑戦"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotScore, gotKeywords := analyzer.Score(tt.messages)
			if math.Abs(gotScore-tt.wantScore) > 1e-9 {
				t.Fatalf("score mismatch: got=%v want=%v", gotScore, tt.wantScore)
			}
			for _, keyword := range tt.wantKeywords {
				if !slices.Contains(gotKeywords, keyword) {
					t.Fatalf("expected keyword %q in %v", keyword, gotKeywords)
				}
			}
		})
	}
}

func TestBuildScoreComment(t *testing.T) {
	tests := []struct {
		name     string
		scores   AnalysisScores
		contains []string
	}{
		{
			name:   "未評価コメント",
			scores: AnalysisScores{},
			contains: []string{
				"チャット診断を完了させることで",
			},
		},
		{
			name: "高評価コメント",
			scores: AnalysisScores{
				JobScore:      0.85,
				InterestScore: 0.82,
				AptitudeScore: 0.81,
				FutureScore:   0.80,
				FinalScore:    0.83,
			},
			contains: []string{
				"志望職種への適性が高い",
				"企業への関心・意欲が非常に高い",
				"総合的に非常に優れたプロフィールです",
			},
		},
		{
			name: "中位評価コメント",
			scores: AnalysisScores{
				JobScore:      0.55,
				InterestScore: 0.52,
				AptitudeScore: 0.51,
				FutureScore:   0.50,
				FinalScore:    0.61,
			},
			contains: []string{
				"一定水準ある",
				"示されている",
				"総合的にバランスの取れたプロフィールです",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildScoreComment(tt.scores)
			for _, part := range tt.contains {
				if !strings.Contains(got, part) {
					t.Fatalf("expected %q in %q", part, got)
				}
			}
		})
	}
}

func TestBuildJobSuitabilityComment(t *testing.T) {
	tests := []struct {
		name               string
		scores             []entity.UserWeightScore
		wantCommentContain string
		wantMinRoles       int
		wantRoleContain    string
	}{
		{
			name:         "空スコア",
			scores:       nil,
			wantMinRoles: 0,
		},
		{
			name: "主要スコアがある場合にコメントとロールを返す",
			scores: []entity.UserWeightScore{
				{WeightCategory: "リーダーシップ志向", Score: 90},
				{WeightCategory: "成長志向", Score: 85},
				{WeightCategory: "チャレンジ志向", Score: 80},
				{WeightCategory: "技術志向", Score: 78},
			},
			wantCommentContain: "強みがあります",
			wantMinRoles:       1,
			wantRoleContain:    "プロジェクトマネージャー",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comment, roles := buildJobSuitabilityComment(tt.scores)
			if tt.wantCommentContain != "" && !strings.Contains(comment, tt.wantCommentContain) {
				t.Fatalf("expected comment to contain %q, got %q", tt.wantCommentContain, comment)
			}
			if len(roles) < tt.wantMinRoles {
				t.Fatalf("roles length mismatch: got=%d want>=%d", len(roles), tt.wantMinRoles)
			}
			if tt.wantRoleContain != "" {
				found := false
				for _, role := range roles {
					if strings.Contains(role.Title, tt.wantRoleContain) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected a role title containing %q, got=%v", tt.wantRoleContain, roles)
				}
			}
		})
	}
}

func TestParseEmbedding(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      []float64
		wantError bool
	}{
		{
			name: "空文字はnil",
			raw:  "   ",
			want: nil,
		},
		{
			name: "有効なJSON配列",
			raw:  "[0.1, 0.2, 0.3]",
			want: []float64{0.1, 0.2, 0.3},
		},
		{
			name:      "不正なJSON",
			raw:       "{bad json}",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEmbedding(tt.raw)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("embedding mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{
			name: "同一ベクトルは1",
			a:    []float64{1, 2, 3},
			b:    []float64{1, 2, 3},
			want: 1,
		},
		{
			name: "長さ不一致は0",
			a:    []float64{1, 2},
			b:    []float64{1, 2, 3},
			want: 0,
		},
		{
			name: "ゼロノルムは0",
			a:    []float64{0, 0},
			b:    []float64{1, 2},
			want: 0,
		},
		{
			name: "負値方向はclampされ0",
			a:    []float64{1, 0},
			b:    []float64{-1, 0},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("cosine mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestClamp01(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{name: "下限以下", value: -0.1, want: 0},
		{name: "範囲内", value: 0.25, want: 0.25},
		{name: "上限以上", value: 1.2, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp01(tt.value)
			if got != tt.want {
				t.Fatalf("clamp mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestAverageCategoryScore(t *testing.T) {
	tests := []struct {
		name       string
		scoreMap   map[string]float64
		categories []string
		want       float64
	}{
		{
			name:       "カテゴリ空",
			scoreMap:   map[string]float64{"技術志向": 80},
			categories: nil,
			want:       0,
		},
		{
			name: "一部カテゴリのみ存在",
			scoreMap: map[string]float64{
				"技術志向": 80,
			},
			categories: []string{"技術志向", "成長志向"},
			want:       0.8,
		},
		{
			name: "100超えスコアはclamp",
			scoreMap: map[string]float64{
				"技術志向": 120,
				"成長志向": 100,
			},
			categories: []string{"技術志向", "成長志向"},
			want:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := averageCategoryScore(tt.scoreMap, tt.categories)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("average mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}
