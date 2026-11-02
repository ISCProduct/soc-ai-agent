package esreview

import (
	"errors"
	"strings"
	"testing"

	"Backend/internal/models"
)

type repoStub struct {
	created   []*models.ESReview
	previous  *models.ESReview
	createErr error
	nextID    uint
}

func (r *repoStub) Create(v *models.ESReview) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.nextID++
	v.ID = r.nextID
	r.created = append(r.created, v)
	return nil
}

func (r *repoStub) FindPreviousByQuestion(uint, string, uint) (*models.ESReview, error) {
	return r.previous, nil
}

type scoreStub struct{ calls int }

func (s *scoreStub) UpdateScoresFromESReview(uint, *models.ESReview) error { s.calls++; return nil }

func baseInput() Input {
	return Input{
		UserID: 1, OrganizationID: 2,
		Question: "設問", QuestionType: "ガクチカ", ESText: "本文",
		SpecificityScore: 7, StarScore: 6, LengthBalanceScore: 5,
	}
}

// 列長を超える入力で INSERT が落ちると、添削は返るのに履歴だけ黙って残らない。
// 保存側で丸めて、利用者の入力で履歴機能だけが死ぬ経路を作らない。
func TestSave_TruncatesToColumnLimits(t *testing.T) {
	repo := &repoStub{}
	in := baseInput()
	in.Question = strings.Repeat("あ", 600)
	in.QuestionType = strings.Repeat("か", 80)
	in.CompanyName = strings.Repeat("さ", 300)

	if _, _, err := NewService(repo, nil).Save(in); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	got := repo.created[0]
	if n := len([]rune(got.Question)); n != 500 {
		t.Errorf("question = %d文字, want 500", n)
	}
	if n := len([]rune(got.QuestionType)); n != 50 {
		t.Errorf("question_type = %d文字, want 50", n)
	}
	if n := len([]rune(*got.CompanyName)); n != 255 {
		t.Errorf("company_name = %d文字, want 255", n)
	}
	if n := len([]rune(got.QuestionKey)); n != 255 {
		t.Errorf("question_key = %d文字, want 255", n)
	}
}

// 企業名が空なら NULL のまま。空文字を入れると「指定した」と区別できない。
func TestSave_EmptyCompanyStaysNil(t *testing.T) {
	repo := &repoStub{}
	in := baseInput()
	in.CompanyName = "   "
	if _, _, err := NewService(repo, nil).Save(in); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if repo.created[0].CompanyName != nil {
		t.Errorf("company_name は nil のままであるべき: %q", *repo.created[0].CompanyName)
	}
}

// 同じ本文を出し直しただけならスコアは動かさない（#1531）。
// 移動平均なので、同じESを繰り返すだけで対象カテゴリを上限へ寄せられる。
func TestSave_SkipsScoreUpdateWhenTextUnchanged(t *testing.T) {
	scores := &scoreStub{}
	repo := &repoStub{previous: &models.ESReview{ID: 9, ESText: "本文"}}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 0 {
		t.Fatalf("同じ本文なのにスコアを反映している: %d回", scores.calls)
	}
	// 履歴そのものは残す。利用者から見れば添削は行われている。
	if len(repo.created) != 1 {
		t.Fatalf("履歴は残すべき: %d件", len(repo.created))
	}
}

// 本文が変わっていればスコアを反映する。
func TestSave_AppliesScoreWhenTextChanged(t *testing.T) {
	scores := &scoreStub{}
	repo := &repoStub{previous: &models.ESReview{ID: 9, ESText: "前回の本文"}}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 1 {
		t.Fatalf("スコアが反映されていない: %d回", scores.calls)
	}
}

// 保存に失敗したらエラーを返す（呼び出し元が握って添削だけ返す）。
func TestSave_ReturnsCreateError(t *testing.T) {
	repo := &repoStub{createErr: errors.New("db down")}
	if _, _, err := NewService(repo, &scoreStub{}).Save(baseInput()); err == nil {
		t.Fatal("保存エラーを返すべき")
	}
}

// 範囲外の評価は丸めて保存する。RAG 側の異常で 0〜100 を外れた値を
// リポジトリへ渡さないため。
func TestSave_ClampsScores(t *testing.T) {
	repo := &repoStub{}
	in := baseInput()
	in.SpecificityScore = 99
	in.StarScore = -5
	fit := 0
	in.CompanyFitScore = &fit

	if _, _, err := NewService(repo, nil).Save(in); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	got := repo.created[0]
	if got.SpecificityScore != 10 || got.StarScore != 1 {
		t.Errorf("丸めが効いていない: specificity=%d star=%d", got.SpecificityScore, got.StarScore)
	}
	if got.CompanyFitScore == nil || *got.CompanyFitScore != 1 {
		t.Errorf("company_fit_score の丸めが効いていない: %v", got.CompanyFitScore)
	}
}
