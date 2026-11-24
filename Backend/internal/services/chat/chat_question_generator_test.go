package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/domain/valueobject"
	"Backend/internal/models"
	internalOpenAI "Backend/internal/openai"
)

type captureAIGeneratedQuestionRepository struct {
	question *models.AIGeneratedQuestion
}

func (r *captureAIGeneratedQuestionRepository) Create(question *models.AIGeneratedQuestion) error {
	r.question = question
	return nil
}

func (*captureAIGeneratedQuestionRepository) FindBySessionID(string) ([]models.AIGeneratedQuestion, error) {
	return nil, nil
}

func (*captureAIGeneratedQuestionRepository) FindByUserAndSession(uint, string) ([]models.AIGeneratedQuestion, error) {
	return nil, nil
}

func (*captureAIGeneratedQuestionRepository) UpdateAnswer(uint, string, int) error {
	return nil
}

func TestGenerateStrategicQuestion_FallbackDoesNotRecordPromptVersion(t *testing.T) {
	t.Setenv("PROMPT_VERSION", "v2")
	t.Setenv("PROMPT_VERSION_QUESTION_GENERATION", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": `"`,
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 1},
		})
	}))
	defer server.Close()

	questionRepo := &captureAIGeneratedQuestionRepository{}
	service := &ChatService{
		aiClient:                internalOpenAI.NewWithBaseURL(server.URL, "gpt-4o-mini"),
		aiGeneratedQuestionRepo: questionRepo,
	}

	question, _, _, err := service.generateStrategicQuestion(
		guardedCtx(),
		nil,
		42,
		"session-1",
		map[string]float64{},
		[]string{"創造性志向"},
		map[string]bool{},
		0,
		0,
		"新卒",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("generateStrategicQuestion returned error: %v", err)
	}
	if question == "" {
		t.Fatal("expected a fallback question")
	}
	if questionRepo.question == nil {
		t.Fatal("expected the fallback question to be saved")
	}
	if questionRepo.question.PromptVersion != "" {
		t.Fatalf("fallback prompt version = %q, want empty", questionRepo.question.PromptVersion)
	}
}

// 戦略を通らない質問生成の経路も版を保存することを検証する（#1778）。
func TestQuestionGenerationPaths_RecordPromptVersion(t *testing.T) {
	t.Setenv("PROMPT_VERSION", "")
	t.Setenv("PROMPT_VERSION_QUESTION_GENERATION", "question-t1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": "最近頑張ったことを教えてください",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 1},
		})
	}))
	defer server.Close()

	newService := func(usages *[]internalOpenAI.Usage) (*ChatService, *captureAIGeneratedQuestionRepository) {
		client := internalOpenAI.NewWithBaseURL(server.URL, "gpt-4o-mini")
		client.OnUsage = func(u internalOpenAI.Usage) { *usages = append(*usages, u) }
		questionRepo := &captureAIGeneratedQuestionRepository{}
		return &ChatService{
			aiClient:                client,
			aiGeneratedQuestionRepo: questionRepo,
			userWeightScoreRepo:     &mockWeightScoreRepo{},
		}, questionRepo
	}
	assertUsage := func(t *testing.T, usages []internalOpenAI.Usage) {
		t.Helper()
		if len(usages) == 0 {
			t.Fatal("AI呼び出しの利用量が記録されていない")
		}
		for _, u := range usages {
			if u.PromptStage != "question_generation" || u.PromptVersion != "question-t1" {
				t.Fatalf("usage=(%q,%q), want (question_generation,question-t1)", u.PromptStage, u.PromptVersion)
			}
		}
	}

	t.Run("generateQuestionWithAI", func(t *testing.T) {
		var usages []internalOpenAI.Usage
		service, questionRepo := newService(&usages)

		if _, _, err := service.generateQuestionWithAI(guardedCtx(), nil, 42, "session-1", 0, 0); err != nil {
			t.Fatalf("generateQuestionWithAI returned error: %v", err)
		}
		if questionRepo.question == nil || questionRepo.question.PromptVersion != "question-t1" {
			t.Fatalf("保存した質問=%+v, want PromptVersion=question-t1", questionRepo.question)
		}
		assertUsage(t, usages)
	})

	t.Run("simplifyQuestionWithAI", func(t *testing.T) {
		var usages []internalOpenAI.Usage
		service, _ := newService(&usages)

		if _, err := service.simplifyQuestionWithAI(guardedCtx(), "難しい質問"); err != nil {
			t.Fatalf("simplifyQuestionWithAI returned error: %v", err)
		}
		assertUsage(t, usages)
	})
}

func TestIsJobSelectionQuestion(t *testing.T) {
	s := &ChatService{}
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"job keyword", "どの職種に興味がありますか？", true},
		{"select with job context", "以下の職種から選んでください", true},
		{"select without job context", "以下から選んでください", false},
		{"number hint", "番号で答えても職種名でも構いません", true},
		{"undecided", "まだ決めていない場合も教えてください", true},
		{"preference clarification", "どんな作業が好きですか？", true},
		{"closest with job options", "どれが近いですか？\n1. エンジニア\n2. 営業", true},
		{"closest without job context", "どれが近いですか？", false},
		{"interview mcq closest", "その方向性で作るときに一番モヤっとしたのはどれですか？（一番近いものを選んでください）", false},
		{"unrelated", "最近頑張ったことを教えてください", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.isJobSelectionQuestion(tc.text)
			if got != tc.want {
				t.Fatalf("isJobSelectionQuestion(%q)=%v want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestShouldValidateJobCategory(t *testing.T) {
	s := &ChatService{}
	cases := []struct {
		name    string
		history []models.ChatMessage
		want    bool
	}{
		{
			name:    "empty history",
			history: nil,
			want:    true,
		},
		{
			name: "job selection last assistant",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "どの職種に興味がありますか？"},
			},
			want: true,
		},
		{
			name: "non job selection last assistant",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "最近頑張ったことを教えてください"},
				{Role: "user", Content: "ハッカソンに参加しました"},
			},
			want: false,
		},
		{
			name: "interview choice after experience must not re-enter job validation",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "経験について具体的に教えてください。"},
				{Role: "user", Content: "児童養護施設向けのUIを作りました"},
				{Role: "assistant", Content: "一番モヤっとしたのはどれですか？（一番近いものを選んでください）\nA) 要件が曖昧\nB) 技術制約\nC) ステークホルダー調整"},
				{Role: "user", Content: "A"},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.shouldValidateJobCategory(tc.history)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

// TestGetCategoryOrder_ReturnsCanonicalCategories は、質問の優先順位が
// 正典カテゴリだけで構成され、10種すべてを重複なく網羅することを検証する（#929）。
//
// ここが正典と食い違うと、chat_question_predefined.go の絞り込みが一致せず
// 事前定義質問が使われないまま毎回AI生成に落ち、未評価カテゴリの判定も
// scoreMap(正典キー)と突き合わないため機能しなくなる。
func TestGetCategoryOrder_ReturnsCanonicalCategories(t *testing.T) {
	canonical := map[string]bool{}
	for _, c := range valueobject.AllWeightCategories() {
		canonical[string(c)] = true
	}

	check := func(label string, got []string) {
		t.Helper()
		if len(got) != 10 {
			t.Fatalf("%s: len=%d want 10 (%v)", label, len(got), got)
		}
		seen := map[string]bool{}
		for _, c := range got {
			if !canonical[c] {
				t.Errorf("%s: 正典外のカテゴリ %q が含まれる", label, c)
			}
			if seen[c] {
				t.Errorf("%s: カテゴリ %q が重複している", label, c)
			}
			seen[c] = true
		}
		if len(seen) != 10 {
			t.Errorf("%s: 10種を網羅していない (%d種)", label, len(seen))
		}
	}

	// 職種未決定
	check("undecided", (&ChatService{}).getCategoryOrder(guardedCtx(), 0))

	// 職種ごとの優先順位（先頭に何を聞くか）は仕様なので固定する。
	// 網羅性チェックだけだと順序を入れ替えても検出できない。
	for _, tt := range []struct {
		code      string
		wantFirst string
	}{
		{code: "ENG01", wantFirst: "技術志向"},
		{code: "SALES01", wantFirst: "コミュニケーション力"},
		{code: "MKT01", wantFirst: "創造性志向"},
		{code: "HR01", wantFirst: "コミュニケーション力"},
		{code: "FIN01", wantFirst: "細部志向"},
		{code: "CONS01", wantFirst: "技術志向"},
		{code: "OTHER", wantFirst: "技術志向"},
	} {
		got := categoryOrderForJobCode(tt.code)
		check(tt.code, got)
		if got[0] != tt.wantFirst {
			t.Errorf("%s: 先頭カテゴリ = %q, want %q", tt.code, got[0], tt.wantFirst)
		}
	}
}

// 全ての正典カテゴリにフォールバック質問が用意されていること。
// 用意が無いとカテゴリ指定が無視され、汎用質問に落ちる。
func TestFallbackQuestions_CoverAllCanonicalCategories(t *testing.T) {
	s := &ChatService{}
	for _, c := range valueobject.AllWeightCategories() {
		for _, level := range []string{"新卒", "中途"} {
			if got := s.fallbackQuestionForCategory(guardedCtx(), string(c), 0, level); got == "" {
				t.Errorf("%s(%s): 単数フォールバックが空", c, level)
			}
			if got := s.fallbackQuestionsForCategory(guardedCtx(), string(c), 0, level); len(got) == 0 {
				t.Errorf("%s(%s): 複数フォールバックが空", c, level)
			}
		}
	}
}

func TestSelectFallbackQuestion_SkipsAsked(t *testing.T) {
	s := &ChatService{}
	asked := map[string]bool{
		"最近頑張ったことはありますか？": true,
	}
	got := s.selectFallbackQuestion(guardedCtx(), "不明カテゴリ", 0, "新卒", asked)
	if got == "" {
		t.Fatal("expected non-empty fallback")
	}
	if asked[got] {
		t.Fatalf("returned already-asked question: %q", got)
	}
}

func TestSelectFallbackQuestion_CategoryOptions(t *testing.T) {
	s := &ChatService{}
	got := s.selectFallbackQuestion(guardedCtx(), "創造性志向", 0, "新卒", map[string]bool{})
	if got == "" {
		t.Fatal("expected creativity fallback")
	}
	options := s.fallbackQuestionsForCategory(guardedCtx(), "創造性志向", 0, "新卒")
	found := false
	for _, q := range options {
		if q == got {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("got %q not in category options %+v", got, options)
	}
}

func TestFallbackQuestionForCategory_DefaultEmpty(t *testing.T) {
	s := &ChatService{}
	if got := s.fallbackQuestionForCategory(guardedCtx(), "存在しないカテゴリ", 0, "新卒"); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

// AI が空を返して代わりの質問にしても、そのあと AI が作り直した文を保存するときは版を残す（#2573）。
func TestGenerateStrategicQuestion_RecordsPromptVersionWhenAIRewritesFallback(t *testing.T) {
	t.Setenv("PROMPT_VERSION", "")
	t.Setenv("PROMPT_VERSION_QUESTION_GENERATION", "char-v1")

	cases := []struct {
		name          string
		outputs       []string
		history       []models.ChatMessage
		allCategories []string
		askedTexts    map[string]bool
		phase         *models.UserAnalysisProgress
		wantQuestion  string
		wantVersion   string
	}{
		{
			// 代わりの質問が自由記述で、選択肢の作り直しを AI がした
			name:    "choice_regenerated",
			outputs: []string{`"`, "合成の質問はどれですか？\nA) 合成1\nB) 合成2\nC) その他（自由記述）"},
			history: []models.ChatMessage{
				{Role: "assistant", Content: "合成の過去質問を具体的に教えてください。"},
				{Role: "user", Content: "合成の回答です"},
			},
			allCategories: []string{"技術志向"},
			askedTexts:    map[string]bool{},
			phase:         &models.UserAnalysisProgress{QuestionsAsked: 1, Phase: charPhase(1, "job_analysis", 3, 5)},
			wantQuestion:  "合成の質問はどれですか？\nA) 合成1\nB) 合成2\nC) その他（自由記述）",
			wantVersion:   "char-v1",
		},
		{
			// 代わりの質問が既に聞いた質問と似ていて、AI が作り直した
			name:          "duplicate_regenerated",
			outputs:       []string{`"`, "合成のまったく別の質問はありますか？"},
			allCategories: []string{"ワークライフバランス"},
			askedTexts:    map[string]bool{"仕事とプライベートのバランスについて、どんな働き方が理想ですか？（合成）": true},
			wantQuestion:  "合成のまったく別の質問はありますか？",
			wantVersion:   "char-v1",
		},
		{
			// 作り直しが無ければ、代わりの質問は AI の文ではないので版は空のまま
			name:          "fallback_only",
			outputs:       []string{`"`},
			allCategories: []string{"ワークライフバランス"},
			askedTexts:    map[string]bool{},
			wantQuestion:  "仕事とプライベートのバランスについて、どんな働き方が理想ですか？",
			wantVersion:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ai, client := newScriptedAIServer(t, tc.outputs...)
			questionRepo := &charAIQuestionRepo{}
			s := &ChatService{
				aiClient:                client,
				aiGeneratedQuestionRepo: questionRepo,
				conversationContextRepo: &charConversationRepo{},
			}
			q, _, _, err := s.generateStrategicQuestion(guardedCtx(), tc.history, 42, "char-session",
				map[string]float64{}, tc.allCategories, tc.askedTexts, 0, 0, "新卒", tc.phase, nil)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if q != tc.wantQuestion {
				t.Fatalf("question=%q, want %q", q, tc.wantQuestion)
			}
			if len(ai.prompts) != len(tc.outputs) {
				t.Fatalf("AI呼び出し回数=%d, want %d", len(ai.prompts), len(tc.outputs))
			}
			if len(questionRepo.created) != 1 {
				t.Fatalf("保存件数=%d", len(questionRepo.created))
			}
			if got := questionRepo.created[0].PromptVersion; got != tc.wantVersion {
				t.Fatalf("PromptVersion=%q, want %q", got, tc.wantVersion)
			}
		})
	}
}

// 既に聞いた質問の一覧は、呼ぶたびに同じ順で並ぶ（#2573）。
func TestAskedQuestionLists_AreSorted(t *testing.T) {
	asked := map[string]bool{}
	for _, q := range []string{"合成質問E", "合成質問B", "合成質問D", "合成質問A", "合成質問C", "合成質問G", "合成質問F"} {
		asked[q] = true
	}
	wantList := "1. 合成質問A\n2. 合成質問B\n3. 合成質問C\n4. 合成質問D\n5. 合成質問E\n6. 合成質問F\n7. 合成質問G\n"

	t.Run("buildAskedQuestionsText", func(t *testing.T) {
		for range 20 {
			if got := buildAskedQuestionsText(asked); !strings.Contains(got, wantList) {
				t.Fatalf("並びが決まっていない:\n%s", got)
			}
		}
	})

	t.Run("regenerateIfDuplicate", func(t *testing.T) {
		for range 20 {
			ai, client := newScriptedAIServer(t, "合成のまったく別の新しい問い")
			s := &ChatService{aiClient: client}
			if _, _, err := s.regenerateIfDuplicate(guardedCtx(), "合成質問A", asked, "技術志向"); err != nil {
				t.Fatalf("err=%v", err)
			}
			if len(ai.prompts) != 1 {
				t.Fatalf("AI呼び出し回数=%d", len(ai.prompts))
			}
			// プロンプトは JSON に入っているので改行は \n で現れる
			if want := strings.ReplaceAll(wantList, "\n", `\n`); !strings.Contains(ai.prompts[0], want) {
				t.Fatalf("並びが決まっていない:\n%s", ai.prompts[0])
			}
		}
	})
}

// ContextData は値に引用符などが入っても正しい JSON になる（#2573）。
func TestGenerateStrategicQuestion_ContextDataIsValidJSON(t *testing.T) {
	_, client := newScriptedAIServer(t, "合成の新しい質問を教えてください。")
	questionRepo := &charAIQuestionRepo{}
	s := &ChatService{
		aiClient:                client,
		aiGeneratedQuestionRepo: questionRepo,
		conversationContextRepo: &charConversationRepo{},
	}
	category := `合成"軸\`
	if _, _, _, err := s.generateStrategicQuestion(guardedCtx(), nil, 42, "char-session",
		map[string]float64{}, []string{category}, map[string]bool{}, 0, 0, "新卒", nil, nil); err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(questionRepo.created) != 1 {
		t.Fatalf("保存件数=%d", len(questionRepo.created))
	}
	raw := questionRepo.created[0].ContextData
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("ContextData が JSON として読めない: %v\n%s", err, raw)
	}
	want := map[string]string{
		"target_category": category,
		"purpose":         "まだ評価できていない「" + category + "」を評価するため",
	}
	if len(got) != len(want) || got["target_category"] != want["target_category"] || got["purpose"] != want["purpose"] {
		t.Fatalf("ContextData=%v, want %v", got, want)
	}
}
