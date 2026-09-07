package resume

import (
	"errors"
	"testing"

	"Backend/internal/config"
	"Backend/internal/models"
)

func intPtr(v int) *int { return &v }

// TestEvaluateResumeStatus は #1030 の受け入れ条件（閾値・境界値・レビュー処理中）を検証する。
func TestEvaluateResumeStatus(t *testing.T) {
	const threshold = 60

	tests := []struct {
		name           string
		hasDocument    bool
		latestScore    *int
		wantAttention  bool
		wantHasDoc     bool
		wantScoreIsNil bool
	}{
		{
			name:           "未提出は要対応",
			hasDocument:    false,
			latestScore:    nil,
			wantAttention:  true,
			wantHasDoc:     false,
			wantScoreIsNil: true,
		},
		{
			// レビューは自動生成されず学生が明示的に実行する操作なので、
			// 未実施は「打つ手がない」ではなく「実行を促すべき」状態。
			name:           "提出済みでレビュー未実施は要対応(未提出とは has_document で区別する)",
			hasDocument:    true,
			latestScore:    nil,
			wantAttention:  true,
			wantHasDoc:     true,
			wantScoreIsNil: true,
		},
		{
			name:          "閾値未満は要対応",
			hasDocument:   true,
			latestScore:   intPtr(59),
			wantAttention: true,
			wantHasDoc:    true,
		},
		{
			name:          "閾値ちょうどは対応不要(境界値)",
			hasDocument:   true,
			latestScore:   intPtr(60),
			wantAttention: false,
			wantHasDoc:    true,
		},
		{
			name:          "閾値超えは対応不要",
			hasDocument:   true,
			latestScore:   intPtr(80),
			wantAttention: false,
			wantHasDoc:    true,
		},
		{
			name:          "スコア0は要対応",
			hasDocument:   true,
			latestScore:   intPtr(0),
			wantAttention: true,
			wantHasDoc:    true,
		},
		{
			name:          "満点は対応不要",
			hasDocument:   true,
			latestScore:   intPtr(100),
			wantAttention: false,
			wantHasDoc:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateResumeStatus(tt.hasDocument, tt.latestScore, threshold)
			if got.NeedsAttention != tt.wantAttention {
				t.Errorf("NeedsAttention = %v, want %v", got.NeedsAttention, tt.wantAttention)
			}
			if got.HasDocument != tt.wantHasDoc {
				t.Errorf("HasDocument = %v, want %v", got.HasDocument, tt.wantHasDoc)
			}
			if tt.wantScoreIsNil && got.LatestScore != nil {
				t.Errorf("LatestScore = %v, want nil", *got.LatestScore)
			}
			if !tt.wantScoreIsNil {
				if got.LatestScore == nil {
					t.Fatal("LatestScore = nil, want 値あり")
				}
				if *got.LatestScore != *tt.latestScore {
					t.Errorf("LatestScore = %d, want %d", *got.LatestScore, *tt.latestScore)
				}
			}
		})
	}
}

// TestResumeCompletenessThreshold は閾値が環境変数で調整可能であることを検証する（PRD 非機能要件）。
func TestResumeCompletenessThreshold(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "未設定なら既定60", env: "", want: 60},
		{name: "正常な値で上書きできる", env: "75", want: 75},
		{name: "数値以外は既定に落ちる", env: "abc", want: 60},
		{name: "0以下は既定に落ちる", env: "-1", want: 60},
		{name: "100超は100へ丸める", env: "200", want: 100},
		{name: "100ちょうどはそのまま", env: "100", want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RESUME_COMPLETENESS_THRESHOLD", tt.env)
			if got := config.ResumeCompletenessThreshold(); got != tt.want {
				t.Errorf("ResumeCompletenessThreshold() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestGetResumeStatus はリポジトリから受け取った内容が状態へ正しく変換されることを検証する。
func TestGetResumeStatus(t *testing.T) {
	t.Run("履歴書なしは未提出として返る", func(t *testing.T) {
		svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), nil)
		got, err := svc.GetResumeStatus(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.HasDocument || !got.NeedsAttention || got.LatestScore != nil {
			t.Errorf("got %+v, want 未提出(has=false, attention=true, score=nil)", got)
		}
	})

	t.Run("レビュー済みならスコアが返る", func(t *testing.T) {
		repo := &resumeRepoStub{
			latestDoc:    &models.ResumeDocument{ID: 10, UserID: 1},
			latestReview: &models.ResumeReview{ID: 5, DocumentID: 10, Score: 42},
		}
		svc := NewResumeService(repo, t.TempDir(), nil)
		got, err := svc.GetResumeStatus(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.HasDocument || !got.NeedsAttention {
			t.Errorf("got %+v, want has=true, attention=true", got)
		}
		if got.LatestScore == nil || *got.LatestScore != 42 {
			t.Errorf("LatestScore = %v, want 42", got.LatestScore)
		}
	})

	t.Run("レビュー未実施はスコアnil・要対応", func(t *testing.T) {
		repo := &resumeRepoStub{latestDoc: &models.ResumeDocument{ID: 10, UserID: 1}}
		svc := NewResumeService(repo, t.TempDir(), nil)
		got, err := svc.GetResumeStatus(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.HasDocument || !got.NeedsAttention || got.LatestScore != nil {
			t.Errorf("got %+v, want has=true, attention=true, score=nil", got)
		}
	})

	// S2: 閾値の環境変数が GetResumeStatus まで実際に配線されていることを検証する。
	// これが無いと config の getter を握り潰してリテラル60を書いてもテストが通ってしまう。
	t.Run("閾値の環境変数がGetResumeStatusまで効いている", func(t *testing.T) {
		t.Setenv("RESUME_COMPLETENESS_THRESHOLD", "40")
		repo := &resumeRepoStub{
			latestDoc:    &models.ResumeDocument{ID: 10, UserID: 1},
			latestReview: &models.ResumeReview{ID: 5, DocumentID: 10, Score: 42},
		}
		svc := NewResumeService(repo, t.TempDir(), nil)
		got, err := svc.GetResumeStatus(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 既定の60なら42は要対応になる。閾値40が効いていれば対応不要。
		if got.NeedsAttention {
			t.Errorf("NeedsAttention = true, want false (閾値40 > スコア42 のはず。環境変数が配線されていない)")
		}
	})

	t.Run("閾値は100を超えない", func(t *testing.T) {
		t.Setenv("RESUME_COMPLETENESS_THRESHOLD", "200")
		repo := &resumeRepoStub{
			latestDoc:    &models.ResumeDocument{ID: 10, UserID: 1},
			latestReview: &models.ResumeReview{ID: 5, DocumentID: 10, Score: 100},
		}
		svc := NewResumeService(repo, t.TempDir(), nil)
		got, err := svc.GetResumeStatus(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.NeedsAttention {
			t.Errorf("満点なのに要対応。閾値が100へ丸められていない")
		}
	})

	t.Run("リポジトリのエラーはそのまま返す", func(t *testing.T) {
		wantErr := errors.New("db down")
		repo := &resumeRepoStub{latestErr: wantErr}
		svc := NewResumeService(repo, t.TempDir(), nil)
		if _, err := svc.GetResumeStatus(1); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}
