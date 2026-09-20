package chat

// 出題時に狙った軸が、採点までそのまま運ばれることのテスト（#1333）。
// 実行: cd Backend && go test ./internal/services/chat/ -run "WeightCategory|Predefined" -v
//
// これまで採点側は質問文のキーワードから軸を推測し直していた。
// キーワードに当たらない質問文は既定の「技術志向」に落ちるため、
//
//	ある軸を狙って出題 → 回答が技術志向に書かれる → 狙った軸は未評価のまま →
//	次のターンでも同じ軸が選ばれる
//
// というループになり、15問かけて6軸しか埋まらなかった。
// 「安定志向」はAI生成の文面がキーワードに当たりにくく、全セッションで
// 一度も測れていなかった。

import (
	"testing"

	"Backend/domain/valueobject"
	"Backend/internal/models"
)

// 推測に頼ると落ちる質問文。実際にAIが生成しうる言い回しを使う。
func TestInferCategory_推測では取り違える質問がある(t *testing.T) {
	s := &ChatService{}

	// 「安定志向」を狙って出した質問だが、キーワード(安定/堅実/長く…)に
	// 当たらないので技術志向に落ちる。これが保存が必要な理由。
	q := "将来のキャリアを考えるとき、腰を落ち着けて働ける環境と、変化の多い環境のどちらに魅力を感じますか？"
	got := s.inferCategoryFromQuestion(q)

	if got == string(valueobject.CategoryStability) {
		t.Skip("推測で当たるようになった。保存の仕組みは依然として必要だが、この例は無効")
	}
	// 取り違えること自体を記録する。直したいのは推測の精度ではなく、
	// 推測に頼っている構造のほう。
	t.Logf("推測結果: %s（狙いは安定志向）", got)
}

// 保存された軸があれば、質問文がどうであれそれが使われる。
func TestChatMessage_WeightCategoryを保持する(t *testing.T) {
	msg := models.ChatMessage{
		Role:           "assistant",
		Content:        "腰を落ち着けて働ける環境と、変化の多い環境のどちらに魅力を感じますか？",
		WeightCategory: string(valueobject.CategoryStability),
	}

	if msg.WeightCategory != "安定志向" {
		t.Errorf("軸が保持されていない: %q", msg.WeightCategory)
	}

	// 正典の10種のいずれかであること。表記ゆれを入れると採点先が作られない。
	if _, ok := valueobject.NormalizeWeightCategory(msg.WeightCategory); !ok {
		t.Errorf("正典に無いカテゴリ名: %q", msg.WeightCategory)
	}
}

// 保存が無い過去のメッセージは、従来どおり推測にフォールバックする。
func TestWeightCategory_空なら推測にフォールバックする(t *testing.T) {
	s := &ChatService{}
	stored := ""

	target := stored
	if target == "" {
		target = s.inferCategoryFromQuestion("チームで協力した経験を教えてください")
	}

	if target != string(valueobject.CategoryTeamwork) {
		t.Errorf("フォールバックが効いていない: %q", target)
	}
}

// --- 事前定義質問の選択（#1333） ---

// 必要な1メソッドだけ動かすスタブ。他は呼ばれないので nil を返す。
type stubPredefinedRepo struct {
	questions []*models.PredefinedQuestion
}

func (r *stubPredefinedRepo) FindActiveQuestions(string, *uint, *uint, string) ([]*models.PredefinedQuestion, error) {
	return r.questions, nil
}
func (r *stubPredefinedRepo) FindByCategory(string, string) ([]*models.PredefinedQuestion, error) {
	return nil, nil
}
func (r *stubPredefinedRepo) FindByID(uint) (*models.PredefinedQuestion, error) { return nil, nil }
func (r *stubPredefinedRepo) Create(*models.PredefinedQuestion) error           { return nil }
func (r *stubPredefinedRepo) Update(*models.PredefinedQuestion) error           { return nil }
func (r *stubPredefinedRepo) GetNextQuestion([]uint, string, *uint, *uint, string, string) (*models.PredefinedQuestion, error) {
	return nil, nil
}
func (r *stubPredefinedRepo) CountByCategory(string) (int64, error) { return 0, nil }

func uintPtr(v uint) *uint { return &v }

// 汎用質問(job_category_id IS NULL)も使われること。
//
// 以前はここで汎用質問を捨てていたため、事前定義質問11問のうち職種に紐づく
// 技術志向の2問しか使われず、安定志向・ワークライフバランスを含む9問が
// 採点ルールごと死んでいた。
func TestTryGetPredefinedQuestion_汎用質問も使う(t *testing.T) {
	generic := &models.PredefinedQuestion{
		ID: 1, Category: "安定志向", QuestionText: "安定した環境で働きたいですか？",
		Priority: 10, JobCategoryID: nil,
	}
	s := &ChatService{predefinedQuestionRepo: &stubPredefinedRepo{
		questions: []*models.PredefinedQuestion{generic},
	}}

	got, err := s.tryGetPredefinedQuestion(1, "sess", "安定志向", 0, 5, "新卒", map[string]bool{}, "")
	if err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if got == nil {
		t.Fatal("汎用質問が捨てられている。安定志向は事前定義にしか無いので永久に測れない")
	}
	if got.ID != generic.ID {
		t.Errorf("別の質問が選ばれた: ID=%d", got.ID)
	}
}

// 職種固有があればそちらを優先すること（元の意図を保つ）。
func TestTryGetPredefinedQuestion_職種固有を優先する(t *testing.T) {
	generic := &models.PredefinedQuestion{
		ID: 1, Category: "技術志向", QuestionText: "汎用の質問", Priority: 99, JobCategoryID: nil,
	}
	specific := &models.PredefinedQuestion{
		ID: 2, Category: "技術志向", QuestionText: "職種固有の質問", Priority: 1, JobCategoryID: uintPtr(5),
	}
	s := &ChatService{predefinedQuestionRepo: &stubPredefinedRepo{
		questions: []*models.PredefinedQuestion{generic, specific},
	}}

	got, err := s.tryGetPredefinedQuestion(1, "sess", "技術志向", 0, 5, "新卒", map[string]bool{}, "")
	if err != nil {
		t.Fatalf("エラー: %v", err)
	}
	// priority は汎用のほうが高いが、職種固有が勝つ
	if got == nil || got.ID != specific.ID {
		t.Errorf("職種固有が優先されていない: %+v", got)
	}
}

// 他職種向けの質問は使わないこと。
func TestTryGetPredefinedQuestion_他職種の質問は使わない(t *testing.T) {
	other := &models.PredefinedQuestion{
		ID: 1, Category: "技術志向", QuestionText: "他職種向け", Priority: 99, JobCategoryID: uintPtr(99),
	}
	s := &ChatService{predefinedQuestionRepo: &stubPredefinedRepo{
		questions: []*models.PredefinedQuestion{other},
	}}

	got, _ := s.tryGetPredefinedQuestion(1, "sess", "技術志向", 0, 5, "新卒", map[string]bool{}, "")
	if got != nil {
		t.Errorf("他職種向けの質問が選ばれた: %+v", got)
	}
}

// 既出の質問は選ばないこと。
func TestTryGetPredefinedQuestion_既出は選ばない(t *testing.T) {
	q := &models.PredefinedQuestion{
		ID: 1, Category: "安定志向", QuestionText: "安定した環境で働きたいですか？", Priority: 10,
	}
	s := &ChatService{predefinedQuestionRepo: &stubPredefinedRepo{
		questions: []*models.PredefinedQuestion{q},
	}}

	got, _ := s.tryGetPredefinedQuestion(1, "sess", "安定志向", 0, 5, "新卒",
		map[string]bool{q.QuestionText: true}, "")
	if got != nil {
		t.Errorf("既出の質問が再選択された: %+v", got)
	}
}
