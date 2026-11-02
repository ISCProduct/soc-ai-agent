package esreview

import (
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
)

type repoStub struct {
	created       []*models.ESReview
	previous      *models.ESReview
	createErr     error
	nextID        uint
	seenHashes    map[string]bool
	sameTextErr   error
	lastScoredAt  *time.Time
	lastScoredErr error
	marked        []uint
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

func (r *repoStub) HasScoredSameText(_ uint, textHash string, _ uint) (bool, error) {
	if r.sameTextErr != nil {
		return false, r.sameTextErr
	}
	return r.seenHashes[textHash], nil
}

func (r *repoStub) LastScoredAt(uint, uint) (*time.Time, error) {
	return r.lastScoredAt, r.lastScoredErr
}

func (r *repoStub) MarkScored(id uint, at time.Time) error {
	r.marked = append(r.marked, id)
	return nil
}

type scoreStub struct {
	calls      int
	notApplied bool
}

func (s *scoreStub) UpdateScoresFromESReview(uint, *models.ESReview) (bool, error) {
	s.calls++
	return !s.notApplied, nil
}

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
//
// 判定は本文のハッシュで行う。設問に紐づけると、本文を変えずに設問のラベルを
// 変えるだけで何度でも加点できる（設問を空にすれば引き当て自体が飛ぶ）。
// 設問や種別は添削結果に影響しない任意入力なので、鍵にしてはいけない。
func TestSave_SkipsScoreWhenSameTextAlreadyScored(t *testing.T) {
	hash := models.HashESText("本文")
	tests := []struct {
		name     string
		question string
		qType    string
	}{
		{"同じ設問", "設問", "ガクチカ"},
		// ここが T3 の見つけた抜け道。設問を変えても本文が同じなら加点しない。
		{"設問だけ変える", "別の設問", "ガクチカ"},
		{"設問を空にする", "", "その他"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scores := &scoreStub{}
			repo := &repoStub{seenHashes: map[string]bool{hash: true}}
			in := baseInput()
			in.Question, in.QuestionType = tt.question, tt.qType

			if _, _, err := NewService(repo, scores).Save(in); err != nil {
				t.Fatalf("Save が失敗: %v", err)
			}
			if scores.calls != 0 {
				t.Fatalf("同じ本文なのにスコアを反映している: %d回", scores.calls)
			}
			// 履歴そのものは残す。利用者から見れば添削は行われている。
			if len(repo.created) != 1 {
				t.Fatalf("履歴は残すべき: %d件", len(repo.created))
			}
		})
	}
}

// 初めて見る本文ならスコアを反映する。
func TestSave_AppliesScoreForNewText(t *testing.T) {
	scores := &scoreStub{}
	repo := &repoStub{seenHashes: map[string]bool{}}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 1 {
		t.Fatalf("スコアが反映されていない: %d回", scores.calls)
	}
}

// 重複判定ができないときは反映しない。二重に積む方が戻せない。
func TestSave_SkipsScoreWhenDuplicateCheckFails(t *testing.T) {
	scores := &scoreStub{}
	repo := &repoStub{sameTextErr: errors.New("db down")}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 0 {
		t.Fatalf("判定できないのに反映している: %d回", scores.calls)
	}
}

// 前後の空白だけの違いは同じ本文とみなす。
func TestHashESText_IgnoresSurroundingSpace(t *testing.T) {
	if models.HashESText("  本文  ") != models.HashESText("本文") {
		t.Error("前後の空白で別の本文になっている")
	}
	if models.HashESText("本文A") == models.HashESText("本文B") {
		t.Error("別の本文が同じハッシュになっている")
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

// 反映の頻度にも上限を置く（#1531）。
//
// 本文の正規化をどれだけ強めても、一語書き換えれば別の本文になる。
// 内容の判定だけでは積み上げが止まらないので、直近の反映から一定時間は動かさない。
func TestSave_SkipsScoreWithinInterval(t *testing.T) {
	recent := time.Now().Add(-1 * time.Hour)
	scores := &scoreStub{}
	repo := &repoStub{seenHashes: map[string]bool{}, lastScoredAt: &recent}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 0 {
		t.Fatalf("直近に反映済みなのに動かしている: %d回", scores.calls)
	}
}

// 間隔が空いていれば、別の本文で反映する。
func TestSave_AppliesScoreAfterInterval(t *testing.T) {
	old := time.Now().Add(-48 * time.Hour)
	scores := &scoreStub{}
	repo := &repoStub{seenHashes: map[string]bool{}, lastScoredAt: &old}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 1 {
		t.Fatalf("反映されていない: %d回", scores.calls)
	}
	if len(repo.marked) != 1 {
		t.Fatalf("反映時刻が記録されていない: %v", repo.marked)
	}
}

// 直近の反映時刻が引けないときは反映しない。
func TestSave_SkipsScoreWhenLastScoredLookupFails(t *testing.T) {
	scores := &scoreStub{}
	repo := &repoStub{seenHashes: map[string]bool{}, lastScoredErr: errors.New("db down")}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 0 {
		t.Fatalf("判定できないのに反映している: %d回", scores.calls)
	}
}

// 見た目が同じ本文は同じ鍵になる。不可視文字・全角半角・合成の違いで
// 別の本文として何度でも積める経路を塞ぐ（いずれも実測で再現した）。
func TestHashESText_FoldsInvisibleAndWidthDifferences(t *testing.T) {
	base := models.HashESText("ゼミ活動でTeam運営を改善した話です。")
	same := []struct {
		name string
		text string
	}{
		{"末尾にZWSP", "ゼミ活動でTeam運営を改善した話です。\u200b"},
		{"BOM", "\ufeffゼミ活動でTeam運営を改善した話です。"},
		{"soft hyphen", "ゼミ活動でTeam\u00ad運営を改善した話です。"},
		{"word joiner", "ゼミ活動でTeam運営を改善した話です。\u2060"},
		{"全角英字", "ゼミ活動でＴｅａｍ運営を改善した話です。"},
		{"英字の大小", "ゼミ活動でTEAM運営を改善した話です。"},
		{"前後と途中の空白", "  ゼミ活動で Team 運営を改善した話です。  "},
		{"改行", "ゼミ活動でTeam運営を\n改善した話です。"},
	}
	for _, tt := range same {
		t.Run(tt.name, func(t *testing.T) {
			if models.HashESText(tt.text) != base {
				t.Errorf("見た目が同じ本文が別の鍵になっている")
			}
		})
	}
	// 中身が変われば別の鍵。畳みすぎていないことを確かめる。
	if models.HashESText("ゼミ活動でTeam運営を改善した話です。") == models.HashESText("サークル活動の話です。") {
		t.Error("別の本文が同じ鍵になっている")
	}
}

// 反映しなかった行を「反映済み」にしない（#1531）。
//
// 診断セッションがまだ無い利用者の添削はスコアへ乗らない。これを計上済みにすると、
// 後から診断セッションができても同じ本文は永久に反映されない（取りこぼし）。
func TestSave_DoesNotMarkScoredWhenNotApplied(t *testing.T) {
	scores := &scoreStub{notApplied: true}
	repo := &repoStub{seenHashes: map[string]bool{}}

	if _, _, err := NewService(repo, scores).Save(baseInput()); err != nil {
		t.Fatalf("Save が失敗: %v", err)
	}
	if scores.calls != 1 {
		t.Fatalf("反映を試みていない: %d回", scores.calls)
	}
	if len(repo.marked) != 0 {
		t.Fatalf("反映していないのに記録している: %v", repo.marked)
	}
}
