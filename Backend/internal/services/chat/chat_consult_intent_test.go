package chat

import (
	"context"
	"strings"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	internalOpenAI "Backend/internal/openai"
)

func TestValidateConsultIntent(t *testing.T) {
	long := strings.Repeat("あ", maxConsultFreeTextRunes+1)
	tests := []struct {
		name    string
		intent  string
		message string
		wantErr bool
	}{
		{"送っていない（既存のクライアント）は、そのまま通す", "", "開発系エンジニア", false},
		{"4つの意図", ConsultIntentFindJob, "やりたい仕事を一緒に考えたい", false},
		{"自由記述", ConsultIntentFree, "情報系の仕事に就きたい", false},
		{"知らない値は拒否", "unknown", "x", true},
		{"自由記述が空白だけなら拒否", ConsultIntentFree, "   ", true},
		{"自由記述が上限ちょうどは通す", ConsultIntentFree, strings.Repeat("あ", maxConsultFreeTextRunes), false},
		{"自由記述が上限超えなら拒否", ConsultIntentFree, long, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConsultIntent(tt.intent, tt.message)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

// --- 最初の回答が相談の意図のときの扱い（職種の判定を飛ばし、意図を保存し、職種を聞く） ---

type intentCtxRepo struct {
	repository.ConversationContextRepository
	intent    string
	jobSet    bool
	intentSet []string
}

func (r *intentCtxRepo) GetJobCategoryID(string) (uint, error) { return 0, nil }
func (r *intentCtxRepo) SetJobCategoryID(uint, string, uint) error {
	r.jobSet = true
	return nil
}
func (r *intentCtxRepo) GetConsultIntent(string) (string, error) { return r.intent, nil }
func (r *intentCtxRepo) SetConsultIntent(_ uint, _ string, v string) error {
	r.intentSet = append(r.intentSet, v)
	r.intent = v
	return nil
}

type intentMsgRepo struct {
	repository.ChatMessageRepository
	created []*models.ChatMessage
}

func (r *intentMsgRepo) Create(m *models.ChatMessage) error {
	r.created = append(r.created, m)
	return nil
}

type intentJobRepo struct {
	repository.JobCategoryRepository
}

func (intentJobRepo) GetTopCategories() ([]models.JobCategory, error) {
	return []models.JobCategory{{Name: "エンジニア"}, {Name: "営業"}}, nil
}

func newIntentService(ctxRepo *intentCtxRepo, msgs *intentMsgRepo) *ChatService {
	// jobValidator の AI は nil。意図の回答で職種の判定（AI 呼び出し）に入ると、ここで落ちる。
	return &ChatService{
		conversationContextRepo: ctxRepo,
		chatMessageRepo:         msgs,
		jobValidator:            &JobCategoryValidator{jobCategoryRepo: intentJobRepo{}},
	}
}

func TestResolveJobCategory_FirstAnswerIsConsultIntent(t *testing.T) {
	for _, intent := range []string{ConsultIntentFindJob, ConsultIntentLearnIndustry, ConsultIntentHowToProceed, ConsultIntentOrganizeStrengths, ConsultIntentFree} {
		t.Run(intent, func(t *testing.T) {
			ctxRepo, msgs := &intentCtxRepo{}, &intentMsgRepo{}
			s := newIntentService(ctxRepo, msgs)
			req := ChatRequest{UserID: 1, SessionID: "s1", Message: "就活の進め方がわからない", ConsultIntent: intent}
			history := []models.ChatMessage{{Role: "user", Content: req.Message}}

			got, err := s.resolveJobCategoryForChat(context.Background(), req, history)
			if err != nil {
				t.Fatal(err)
			}
			if got.earlyResponse == nil {
				t.Fatal("職種を聞く質問を、そのターンの応答として返すはず")
			}
			if len(ctxRepo.intentSet) != 1 || ctxRepo.intentSet[0] != intent {
				t.Fatalf("意図を1回だけ保存するはず: %v", ctxRepo.intentSet)
			}
			if ctxRepo.jobSet {
				t.Fatal("意図の回答を、職種として保存してはいけない")
			}
			resp := got.earlyResponse
			if !strings.Contains(resp.Response, "エンジニア") || !strings.Contains(resp.Response, "営業") {
				t.Fatalf("職種の一覧が出るはず: %q", resp.Response)
			}
			if resp.AnsweredQuestions != 0 || resp.IsComplete {
				t.Fatalf("意図の回答は質問数に数えない: %+v", resp)
			}
			if resp.JudgmentPercent != 0 {
				t.Fatalf("判定の進み具合は0%%のまま: %d", resp.JudgmentPercent)
			}
			if len(msgs.created) != 1 || msgs.created[0].Role != "assistant" || msgs.created[0].Content != resp.Response {
				t.Fatalf("職種を聞く質問を、会話に1件保存するはず: %+v", msgs.created)
			}
		})
	}
}

func TestResolveJobCategory_ConsultIntentIgnoredAfterFirstTurn(t *testing.T) {
	// すでに質問が出ている（診断の出題。評価軸あり）あとに意図が付いて届いても、保存せず、職種の判定にも入らない
	ctxRepo, msgs := &intentCtxRepo{}, &intentMsgRepo{}
	s := newIntentService(ctxRepo, msgs)
	req := ChatRequest{UserID: 1, SessionID: "s1", Message: "回答", ConsultIntent: ConsultIntentFindJob, JobCategoryID: 5}
	history := []models.ChatMessage{
		{Role: "assistant", Content: "質問です。", WeightCategory: "logic"},
		{Role: "user", Content: "回答"},
	}
	got, err := s.resolveJobCategoryForChat(context.Background(), req, history)
	if err != nil {
		t.Fatal(err)
	}
	if got.earlyResponse != nil || len(ctxRepo.intentSet) != 0 || len(msgs.created) != 0 {
		t.Fatalf("2回目以降は意図を扱わない: early=%v saved=%v msgs=%d", got.earlyResponse, ctxRepo.intentSet, len(msgs.created))
	}
}

func TestResolveJobCategory_ConsultIntentNotOverwritten(t *testing.T) {
	// 最初の選択を正とする。保存済みなら、同じセッションで別の意図が来ても上書きしない（職種は決まっている）
	ctxRepo, msgs := &intentCtxRepo{intent: ConsultIntentFindJob}, &intentMsgRepo{}
	s := newIntentService(ctxRepo, msgs)
	req := ChatRequest{UserID: 1, SessionID: "s1", Message: "回答", ConsultIntent: ConsultIntentFree, JobCategoryID: 5}
	history := []models.ChatMessage{{Role: "assistant", Content: "q", WeightCategory: "logic"}, {Role: "user", Content: "回答"}}
	if _, err := s.resolveJobCategoryForChat(context.Background(), req, history); err != nil {
		t.Fatal(err)
	}
	if len(ctxRepo.intentSet) != 0 {
		t.Fatalf("保存済みの意図を上書きしてはいけない: %v", ctxRepo.intentSet)
	}
}

// --- 有効な回答数の数え方（判定の床に使う） ---

func TestValidAnswersFromProgress(t *testing.T) {
	phases := map[uint]*models.AnalysisPhase{
		1: {PhaseName: "job_analysis"},
		2: {PhaseName: "interest_analysis"},
		3: {PhaseName: "aptitude_analysis"},
	}
	tests := []struct {
		name string
		in   []models.UserAnalysisProgress
		want int
	}{
		{"何も無い", nil, 0},
		{"職種の回答1回で、職種のフェーズが4問分に引き上げられても、1回と数える", []models.UserAnalysisProgress{{PhaseID: 1, ValidAnswers: 4}}, 1},
		{"職種1回＋興味2回＋適性3回", []models.UserAnalysisProgress{{PhaseID: 1, ValidAnswers: 4}, {PhaseID: 2, ValidAnswers: 2}, {PhaseID: 3, ValidAnswers: 3}}, 6},
		{"フェーズが引けないときは、そのまま数える", []models.UserAnalysisProgress{{PhaseID: 99, ValidAnswers: 3}}, 3},
		{"Phase が付いていれば、それを使う", []models.UserAnalysisProgress{{PhaseID: 99, ValidAnswers: 4, Phase: &models.AnalysisPhase{PhaseName: "job_analysis"}}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validAnswersFromProgress(tt.in, phases); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConsultIntentContext(t *testing.T) {
	tests := []struct {
		name   string
		intent string
		want   string // 含まれるべき文言。空なら、何も足さない
	}{
		{"未選択は何も足さない", "", ""},
		{"知らない値は何も足さない", "unknown", ""},
		{"強みの整理", ConsultIntentOrganizeStrengths, "自分の強みを整理したい"},
		{"自由記述は、本文を入れず、固定のラベルだけ", ConsultIntentFree, "自分の言葉で相談内容を入力した"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChatService{conversationContextRepo: &intentCtxRepo{intent: tt.intent}}
			got := s.consultIntentContext("s1")
			if tt.want == "" {
				if got != "" {
					t.Fatalf("何も足さないはず: %q", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("%q が含まれるはず: %q", tt.want, got)
			}
		})
	}
	if got := (&ChatService{}).consultIntentContext("s1"); got != "" {
		t.Fatalf("リポジトリが無いときは何も足さない: %q", got)
	}
}

// マスキング: 意図の経路は外部 AI を一切呼ばない。自由記述の本文は、会話（chat_messages）にだけ残り、
// 意図の保存・次の質問の生成のどちらにも渡らない。（PII は ProcessChat 冒頭の withPIIGuard で伏字になる）
func TestConsultIntent_NoExternalAICall(t *testing.T) {
	srv, bodies := newChatBodyServer(t)
	ctxRepo, msgs := &intentCtxRepo{}, &intentMsgRepo{}
	s := newIntentService(ctxRepo, msgs)
	s.aiClient = internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")

	// 合成データ。実在しない氏名・メール
	msg := "山田太郎（taro.yamada@example.com）です。情報系の仕事に就きたいです。"
	req := ChatRequest{UserID: 1, SessionID: "s1", Message: msg, ConsultIntent: ConsultIntentFree}
	history := []models.ChatMessage{{Role: "user", Content: msg}}
	if _, err := s.resolveJobCategoryForChat(context.Background(), req, history); err != nil {
		t.Fatal(err)
	}
	if n := len(bodies()); n != 0 {
		t.Fatalf("意図の経路で外部 AI が %d 回呼ばれた", n)
	}
	for _, m := range msgs.created {
		if strings.Contains(m.Content, "taro.yamada") || strings.Contains(m.Content, "山田") {
			t.Fatalf("返答に本文が写った: %q", m.Content)
		}
	}
	if len(ctxRepo.intentSet) != 1 || ctxRepo.intentSet[0] != ConsultIntentFree {
		t.Fatalf("保存するのは意図の値だけ: %v", ctxRepo.intentSet)
	}
}
