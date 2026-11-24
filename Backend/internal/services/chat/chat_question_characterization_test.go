package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	internalOpenAI "Backend/internal/openai"
)

// 特性テスト（#2569）: generateStrategicQuestion / processAnswerAndNextQuestion の
// 今の出力（AI へ送るプロンプトの順序と本文、戻り値、保存内容）を golden ファイルに固定する。
// 分割リファクタで挙動が変わっていないことを確かめるためのもの。データはすべて合成。
// 意図して挙動を変えたときだけ UPDATE_CHAT_GOLDEN=1 で golden を作り直す。

// untrustedToken は会話履歴を囲む区切りの乱数部分。呼ぶたびに変わるので固定値に置き換えて比べる。
var untrustedToken = regexp.MustCompile(`(UNTRUSTED_[^_]+_)[A-Z0-9]+(_(START|END))`)

// scriptedAIServer は、決めた順に応答を返し、受け取ったプロンプトを記録する偽の AI。
type scriptedAIServer struct {
	mu      sync.Mutex
	outputs []string
	prompts []string
}

func newScriptedAIServer(t *testing.T, outputs ...string) (*scriptedAIServer, *internalOpenAI.Client) {
	t.Helper()
	s := &scriptedAIServer{outputs: outputs}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Input json.RawMessage `json:"input"`
		}
		_ = json.Unmarshal(body, &req)
		s.mu.Lock()
		s.prompts = append(s.prompts, untrustedToken.ReplaceAllString(string(req.Input), "${1}TOKEN${2}"))
		out := "<<台本切れ>>"
		if len(s.outputs) > 0 {
			out = s.outputs[0]
			s.outputs = s.outputs[1:]
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": out,
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 1},
		})
	}))
	t.Cleanup(server.Close)
	return s, internalOpenAI.NewWithBaseURL(server.URL, "gpt-4o-mini")
}

type charAIQuestionRepo struct {
	repository.AIGeneratedQuestionRepository
	existing  []models.AIGeneratedQuestion
	createErr error
	created   []models.AIGeneratedQuestion
}

func (r *charAIQuestionRepo) Create(q *models.AIGeneratedQuestion) error {
	if r.createErr != nil {
		return r.createErr
	}
	q.ID = uint(100 + len(r.created))
	r.created = append(r.created, *q)
	return nil
}

func (r *charAIQuestionRepo) FindByUserAndSession(uint, string) ([]models.AIGeneratedQuestion, error) {
	return r.existing, nil
}

type charScoreRepo struct {
	repository.UserWeightScoreRepository
	scores []models.UserWeightScore
}

func (r *charScoreRepo) FindByUserAndSession(uint, string) ([]models.UserWeightScore, error) {
	return r.scores, nil
}

type charProgressRepo struct {
	repository.UserAnalysisProgressRepository
	progresses []models.UserAnalysisProgress
	updated    []models.UserAnalysisProgress
}

func (r *charProgressRepo) FindByUserAndSession(uint, string) ([]models.UserAnalysisProgress, error) {
	return r.progresses, nil
}

func (r *charProgressRepo) Update(p *models.UserAnalysisProgress) error {
	cp := *p
	cp.CompletedAt = nil // 時刻はテストごとに変わるので記録しない
	r.updated = append(r.updated, cp)
	return nil
}

type charPhaseRepo struct {
	repository.AnalysisPhaseRepository
	phases []models.AnalysisPhase
}

func (r *charPhaseRepo) FindAll() ([]models.AnalysisPhase, error) { return r.phases, nil }

type charUserRepo struct {
	repository.UserRepository
	user *models.User
}

func (r *charUserRepo) GetUserByID(uint) (*models.User, error) { return r.user, nil }

type charJobCategoryRepo struct {
	repository.JobCategoryRepository
	category *models.JobCategory
}

func (r *charJobCategoryRepo) FindByID(context.Context, uint) (*models.JobCategory, error) {
	return r.category, nil
}

type charConversationRepo struct {
	repository.ConversationContextRepository
	intent string
}

func (r *charConversationRepo) GetConsultIntent(string) (string, error) { return r.intent, nil }

func charPhase(id uint, name string, minQ, maxQ int) *models.AnalysisPhase {
	return &models.AnalysisPhase{
		ID: id, PhaseName: name, DisplayName: name + "（表示名）",
		Description: name + " の合成説明", MinQuestions: minQ, MaxQuestions: maxQ,
	}
}

func charJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("<<marshal error: %v>>", err)
	}
	return string(b)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "chat_question_golden", name+".golden")
	if os.Getenv("UPDATE_CHAT_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden を読めない（UPDATE_CHAT_GOLDEN=1 で作成）: %v", err)
	}
	if string(want) != got {
		t.Fatalf("%s の出力が golden と違う\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func writePrompts(b *strings.Builder, ai *scriptedAIServer) {
	fmt.Fprintf(b, "== AI呼び出し回数: %d\n", len(ai.prompts))
	for i, p := range ai.prompts {
		fmt.Fprintf(b, "== プロンプト %d\n%s\n", i+1, p)
	}
}

func TestGenerateStrategicQuestion_Characterization(t *testing.T) {
	t.Setenv("PROMPT_VERSION", "")
	t.Setenv("PROMPT_VERSION_QUESTION_GENERATION", "char-v1")

	cases := []struct {
		name          string
		outputs       []string
		history       []models.ChatMessage
		scoreMap      map[string]float64
		allCategories []string
		askedTexts    map[string]bool
		jobCategoryID uint
		jobCategory   *models.JobCategory
		targetLevel   string
		phase         *models.UserAnalysisProgress
		asked         map[string]int
		intent        string
	}{
		{
			// 選択肢必須フェーズで、選択肢の無い質問が返り、再生成で選択肢つきになる
			name: "choice_regenerated",
			outputs: []string{
				"最近どんなことに挑戦しましたか？教えてください。",
				"まだ自由記述の質問です。教えてください。",
				"興味がある分野はどれですか？\nA) 開発\nB) 企画\nC) その他（自由記述）",
			},
			history: []models.ChatMessage{
				{Role: "assistant", Content: "どんな経験がありますか？具体的に教えてください。"},
				{Role: "user", Content: "合成の回答です"},
			},
			scoreMap:      map[string]float64{"技術志向": 40},
			allCategories: []string{"技術志向", "成長志向"},
			askedTexts:    map[string]bool{"どんな経験がありますか？": true},
			phase:         &models.UserAnalysisProgress{QuestionsAsked: 1, Phase: charPhase(1, "job_analysis", 3, 5)},
			intent:        ConsultIntentFindJob,
		},
		{
			// 選択肢の再生成が2回とも選択肢なしで、決まった選択肢で補う
			name: "choice_fallback_built",
			outputs: []string{
				"休日はどのように過ごしていますか？教えてください。",
				"自由記述のままです。教えてください。",
				"まだ自由記述です。教えてください。",
			},
			history: []models.ChatMessage{
				{Role: "assistant", Content: "どんな経験がありますか？具体的に教えてください。"},
				{Role: "user", Content: "合成の回答です"},
			},
			scoreMap:      map[string]float64{},
			allCategories: []string{"技術志向"},
			askedTexts:    map[string]bool{},
			phase:         &models.UserAnalysisProgress{QuestionsAsked: 1, Phase: charPhase(2, "interest_analysis", 3, 4)},
			asked:         map[string]int{"技術志向": 2},
		},
		{
			// 中途・フェーズ定義外・最大質問数なし。重複が続き再生成を使い切る
			name: "duplicate_retry_exhausted_mid",
			outputs: []string{
				"これまでの仕事で工夫したことは？",
				"これまでの仕事で工夫したことは？",
				"これまでの仕事で工夫したことは何ですか？",
				"これまでの仕事で工夫したことは？",
			},
			history:       []models.ChatMessage{{Role: "user", Content: "合成の回答です"}},
			scoreMap:      map[string]float64{"安定志向": 10},
			allCategories: []string{"安定志向", "ワークライフバランス"},
			askedTexts:    map[string]bool{"これまでの仕事で工夫したことは？": true},
			jobCategoryID: 7,
			jobCategory:   &models.JobCategory{Name: "合成職種", Code: "ENG01"},
			targetLevel:   "中途",
			phase:         &models.UserAnalysisProgress{QuestionsAsked: 0, Phase: charPhase(3, "custom_phase", 2, 0)},
		},
		{
			// フェーズ最初の質問は自由記述を強制する
			name:          "force_text_question",
			outputs:       []string{"チームで取り組んだことを教えてください。"},
			history:       nil,
			scoreMap:      map[string]float64{"コミュニケーション力": 0},
			allCategories: []string{"コミュニケーション力"},
			askedTexts:    map[string]bool{},
			phase:         &models.UserAnalysisProgress{QuestionsAsked: 0, Phase: charPhase(4, "aptitude_analysis", 3, 4)},
			asked:         map[string]int{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ai, client := newScriptedAIServer(t, tc.outputs...)
			questionRepo := &charAIQuestionRepo{}
			s := &ChatService{
				aiClient:                client,
				aiGeneratedQuestionRepo: questionRepo,
				jobCategoryRepo:         &charJobCategoryRepo{category: tc.jobCategory},
				conversationContextRepo: &charConversationRepo{intent: tc.intent},
			}

			q, id, cat, err := s.generateStrategicQuestion(context.Background(), tc.history, 42, "char-session",
				tc.scoreMap, tc.allCategories, tc.askedTexts, 3, tc.jobCategoryID, tc.targetLevel, tc.phase, tc.asked)

			var b strings.Builder
			fmt.Fprintf(&b, "== 戻り値\nquestion=%q\nid=%d\ncategory=%q\nerr=%v\n", q, id, cat, err)
			fmt.Fprintf(&b, "== 保存した質問\n%s\n", charJSON(questionRepo.created))
			writePrompts(&b, ai)
			assertGolden(t, "strategic_"+tc.name, b.String())
		})
	}
}

func TestProcessAnswerAndNextQuestion_Characterization(t *testing.T) {
	t.Setenv("PROMPT_VERSION", "")
	t.Setenv("PROMPT_VERSION_QUESTION_GENERATION", "char-v1")

	jobSelection := "まず、どの職種に興味がありますか？以下から選んでください：\n\n1. エンジニア（プログラミング、開発）\n2. 営業（顧客対応、提案）\n3. その他・まだ決めていない\n\n番号で答えても、職種名で答えても構いません。"
	phases := []models.AnalysisPhase{*charPhase(1, "job_analysis", 3, 5), *charPhase(2, "interest_analysis", 3, 0)}

	cases := []struct {
		name            string
		outputs         []string
		message         string
		history         []models.ChatMessage
		jobCategoryID   uint
		jobCategory     *models.JobCategory
		jobJustResolved bool
		phase           *models.UserAnalysisProgress
		completed       []models.UserAnalysisProgress
		extraRound      bool
		user            *models.User
		scores          []models.UserWeightScore
		existingAIQs    []models.AIGeneratedQuestion
		predefined      []*models.PredefinedQuestion
		createErr       error
	}{
		{
			// 職種未定・AI 生成。フェーズ最初の自由記述
			name:    "ai_generated_new_grad",
			outputs: []string{"あなたが最近夢中になったことについて教えてください。"},
			message: "合成の回答です。授業で発表をしました。",
			history: []models.ChatMessage{{Role: "user", Content: "合成の回答です。授業で発表をしました。"}},
			phase:   &models.UserAnalysisProgress{ID: 11, PhaseID: 1, QuestionsAsked: 1, ValidAnswers: 1, Phase: charPhase(1, "job_analysis", 3, 5)},
			completed: []models.UserAnalysisProgress{
				{PhaseID: 1, ValidAnswers: 5},
				{PhaseID: 2, ValidAnswers: 1, Phase: charPhase(2, "interest_analysis", 3, 0)},
			},
			user:         &models.User{TargetLevel: ""},
			scores:       []models.UserWeightScore{{WeightCategory: "技術志向", Score: 30}, {WeightCategory: "成長志向", Score: 0}},
			existingAIQs: []models.AIGeneratedQuestion{{QuestionText: "過去の合成質問はありますか？"}},
		},
		{
			// 職種が決まった直後・事前定義質問。新卒向けに冗長な質問を言い換える
			name:            "predefined_job_just_resolved",
			outputs:         []string{"開発の経験を教えてください。"},
			message:         "エンジニア",
			history:         []models.ChatMessage{{Role: "user", Content: "エンジニア"}},
			jobCategoryID:   5,
			jobCategory:     &models.JobCategory{Name: "合成エンジニア", Code: "ENG01"},
			jobJustResolved: true,
			user:            &models.User{TargetLevel: "新卒"},
			predefined: []*models.PredefinedQuestion{{
				ID: 9, Category: "技術志向", Priority: 1,
				QuestionText: "これまでに取り組んだ開発経験について教えてください。（授業や趣味でも構いません）",
			}},
		},
		{
			// 事前定義質問が自由記述で、選択肢フェーズなので選択肢で補う
			name:    "predefined_choice_fallback",
			message: "エンジニアに興味があります",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "どんな経験がありますか？具体的に教えてください。"},
				{Role: "user", Content: "合成の回答です"},
				{Role: "assistant", Content: jobSelection},
				{Role: "user", Content: "エンジニアに興味があります"},
			},
			jobCategoryID: 5,
			jobCategory:   &models.JobCategory{Name: "合成エンジニア", Code: "ENG01"},
			phase:         &models.UserAnalysisProgress{ID: 12, PhaseID: 2, QuestionsAsked: 1, Phase: charPhase(2, "interest_analysis", 3, 0)},
			user:          &models.User{TargetLevel: "新卒"},
			predefined: []*models.PredefinedQuestion{{
				ID: 21, Category: "技術志向", Priority: 1, QuestionText: "好きな技術について教えてください。",
			}},
		},
		{
			// 中途・延長ラウンド。AI 質問の保存に失敗し、フォールバック質問になる
			name:       "ai_save_error_fallback_mid_extra_round",
			outputs:    []string{"チームでの役割を教えてください。"},
			message:    "合成の回答です",
			history:    []models.ChatMessage{{Role: "user", Content: "合成の回答です"}},
			phase:      &models.UserAnalysisProgress{ID: 13, PhaseID: 2, QuestionsAsked: 3, ValidAnswers: 3, Phase: charPhase(2, "interest_analysis", 3, 0)},
			extraRound: true,
			user:       &models.User{TargetLevel: "中途"},
			scores:     []models.UserWeightScore{{WeightCategory: "技術志向", Score: 50}},
			createErr:  errors.New("合成の保存エラー"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ai, client := newScriptedAIServer(t, tc.outputs...)
			questionRepo := &charAIQuestionRepo{existing: tc.existingAIQs, createErr: tc.createErr}
			chatRepo := &fakeChatRepo{}
			progressRepo := &charProgressRepo{progresses: tc.completed}
			s := &ChatService{
				aiClient:                client,
				aiGeneratedQuestionRepo: questionRepo,
				chatMessageRepo:         chatRepo,
				userWeightScoreRepo:     &charScoreRepo{scores: tc.scores},
				progressRepo:            progressRepo,
				phaseRepo:               &charPhaseRepo{phases: phases},
				userRepo:                &charUserRepo{user: tc.user},
				jobCategoryRepo:         &charJobCategoryRepo{category: tc.jobCategory},
				predefinedQuestionRepo:  &stubPredefinedRepo{questions: tc.predefined},
			}
			phaseByID := map[uint]*models.AnalysisPhase{1: &phases[0], 2: &phases[1]}

			resp, err := s.processAnswerAndNextQuestion(context.Background(), processQuestionInput{
				req:                 ChatRequest{UserID: 42, SessionID: "char-session", Message: tc.message, IndustryID: 3},
				history:             tc.history,
				jobCategoryID:       tc.jobCategoryID,
				jobJustResolved:     tc.jobJustResolved,
				currentPhase:        tc.phase,
				allPhases:           phases,
				completedProgresses: tc.completed,
				phaseByID:           phaseByID,
				extraRound:          tc.extraRound,
			})

			var b strings.Builder
			fmt.Fprintf(&b, "== err=%v\n== 応答\n%s\n", err, charJSON(resp))
			saved := make([]models.ChatMessage, 0, len(chatRepo.created))
			for _, m := range chatRepo.created {
				saved = append(saved, *m)
			}
			fmt.Fprintf(&b, "== 保存したメッセージ\n%s\n", charJSON(saved))
			fmt.Fprintf(&b, "== 保存した AI 質問\n%s\n", charJSON(questionRepo.created))
			fmt.Fprintf(&b, "== 更新した進捗\n%s\n", charJSON(progressRepo.updated))
			writePrompts(&b, ai)
			assertGolden(t, "process_"+tc.name, b.String())
		})
	}
}
