package interview

import (
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// ---- M1: 履歴書の属性欄を送らない ----

const resumeWithHeader = `履歴書
氏名 山田 太郎
ふりがな やまだ たろう
生年月日 2002年4月1日生（満22歳）
性別 男
現住所 〒150-0001 東京都渋谷区神南1-2-3
電話 090-1234-5678
メール taro.yamada@example.com
学歴
2021年4月 ○○大学 情報工学部 入学
職歴
2023年4月 株式会社サンプル アルバイト 受付システムの改修を担当
資格
基本情報技術者
自己PR
ゼミで開発リーダーを務め、リリースを3回経験しました。
志望動機
御社の事業に関心があります。
本人希望記入欄
特になし`

func TestFormatResumeContextDropsAttributes(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		keep    []string
		dropped []string
	}{
		{
			name:    "見出しがある履歴書は節だけ残す",
			text:    resumeWithHeader,
			keep:    []string{"○○大学 情報工学部", "受付システムの改修", "基本情報技術者", "開発リーダー", "御社の事業に関心"},
			dropped: []string{"山田", "やまだ", "2002", "22歳", "男", "渋谷", "150-0001", "090-1234", "taro.yamada", "特になし"},
		},
		{
			name: "見出しが取れないときは属性行を除外する",
			text: "山田 太郎\n2002年4月1日生\n東京都渋谷区神南1-2-3\nTEL 090-1234-5678\nmail: a@example.com\n性別: 男\n" +
				"2021年4月 ○○大学入学\nゼミで開発リーダーを務めた。",
			keep:    []string{"○○大学入学", "開発リーダー"},
			dropped: []string{"山田", "2002", "渋谷", "090-1234", "a@example.com", "男"},
		},
		{
			name: "属性除去のあとで切り詰める（属性が長くても職歴が残る）",
			text: strings.Repeat("住所 東京都渋谷区神南1-2-3\n", 200) + "職歴の本文ここ",
			keep: []string{"職歴の本文ここ"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResumeContext(tt.text)
			for _, k := range tt.keep {
				if !strings.Contains(got, k) {
					t.Errorf("%q が残っていない: %s", k, got)
				}
			}
			for _, d := range tt.dropped {
				if strings.Contains(got, d) {
					t.Errorf("属性 %q が送信文字列に含まれる: %s", d, got)
				}
			}
		})
	}
	if formatResumeContext("氏名 山田 太郎\n生年月日 2002年4月1日") != "" {
		t.Error("属性しか無い履歴書は何も足さない")
	}
}

// ---- M2: 差別質問フィルタ ----

func TestContainsDiscriminatoryQuestionTable(t *testing.T) {
	positives := []string{
		"〇〇県のご出身ですね。", "出身はどちらですか。", "どちらのご出身ですか。", "ご出身は関西ですか。",
		"地元はどちらですか。", "ご実家はどのあたりですか。", "お住まいはどちらですか。",
		"ご両親の仕事を教えてください。", "親の仕事は何ですか。", "お父さんの職業は何ですか。",
		"ご家族の構成を教えてください。", "ご兄弟はいらっしゃいますか。",
		"住宅状況を教えてください。", "持ち家ですか。", "あなたの人生観を教えてください。", "生活信条は何ですか。",
		"尊敬する人物は誰ですか。", "労働組合に加入していましたか。", "学生運動の経験はありますか。",
		"購読している新聞は何ですか。", "愛読書を教えてください。", "どの雑誌を読んでいますか。",
		"性別を教えてください。", "年齢はおいくつですか。", "何歳ですか。", "生年月日を教えてください。", "お誕生日はいつですか。",
		"結婚のご予定はありますか。", "ご結婚されていますか。", "お子さんはいらっしゃいますか。", "出産の予定はありますか。",
		"健康状態に問題はありませんか。", "持病はありますか。", "病歴を教えてください。", "障害はありますか。", "障害者手帳をお持ちですか。",
		"国籍はどちらですか。", "何人の方ですか。", "宗教は何ですか。", "支持政党はどこですか。", "本籍地を教えてください。",
	}
	for _, s := range positives {
		if !ContainsDiscriminatoryQuestion(s) {
			t.Errorf("検出できない: %q", s)
		}
	}
	negatives := []string{
		"出身大学を教えてください。", "ご出身の大学ではどんな研究をしましたか。", "東京大学のご出身ですね。", "○○高校出身のあなたは部活で何を。",
		"出身校でのゼミ活動を教えてください。", "出身を問わず活躍できる環境です。", "出身地を問わず応募できます。",
		"チームは何人ですか。", "プロジェクトは何人で進めましたか。", "何人くらいのチームでしたか。",
		"親会社の仕事を担当した経験はありますか。", "システム障害に対応した経験を教えてください。",
		"平均年齢は28歳です。", "男女問わず活躍しています。", "性別を問わず応募できます。", "何歳までに管理職を目指しますか。",
		"結婚式場向けのシステムを開発した経験は。", "健康経営に関心はありますか。", "設計思想を教えてください。", "宗教学を専攻した理由は。",
		genericFallbackQuestion, "自己紹介・志望動機", "職務経験・実績", "強み・弱み", "キャリアビジョン", "逆質問",
		"チームで何かに取り組んだ経験で、あなたが担った役割を教えてください。",
	}
	for _, s := range negatives {
		if ContainsDiscriminatoryQuestion(s) {
			t.Errorf("誤検知: %q", s)
		}
	}
}

// ---- M3: 軸スロットの現在のトピック・進め方が実キューに合う ----

func TestAdvanceQuestionPlanFillsPlanForAxisQueue(t *testing.T) {
	stateRepo := &questionPlanStateRepoStub{}
	svc := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	svc.SetQuestionStateRepo(stateRepo)
	svc.SetCompanyRepo(&axisRepoStub{company: &models.Company{ID: 1}, company0: profileWith(func(p *models.CompanyWeightProfile) {
		p.TeamworkOrientation, p.TechnicalOrientation, p.DetailOrientation = 90, 80, 10
	})})
	d, err := svc.advanceQuestionPlan(context.Background(), &models.InterviewSession{ID: 5, CompanyID: 1}, 1, "", "", "", true)
	if err != nil || d == nil {
		t.Fatalf("directive = %+v, %v", d, err)
	}
	want := []string{"自己紹介・志望動機", "職務経験・実績", axisTopicName, axisTopicName, "逆質問"}
	if strings.Join(d.Plan, "|") != strings.Join(want, "|") {
		t.Fatalf("Plan = %v, want %v", d.Plan, want)
	}
	if d.PlanPosition != 1 || d.PendingQuestions != 4 {
		t.Fatalf("PlanPosition=%d Pending=%d, want 1/4", d.PlanPosition, d.PendingQuestions)
	}
}

func TestSystemPromptForAxisSlotMatchesQueue(t *testing.T) {
	plan := []string{"自己紹介・志望動機", "職務経験・実績", axisTopicName, axisTopicName, "逆質問"}
	d := axisDirective("teamwork")
	d.Plan, d.PlanPosition, d.PendingQuestions = plan, 3, 2
	p := buildInterviewSystemPrompt("テスト株式会社", "", "エンジニア", "", "general", nil, nil, 3, 120, 3, 5, 0, 180, d)
	for _, want := range []string{
		"現在のトピック: " + axisTopicName + "（3/5）",
		"残り2トピックあります",
		"1. 自己紹介・志望動機\n2. 職務経験・実績\n3. " + axisTopicName + "\n4. " + axisTopicName + "\n5. 逆質問",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("プロンプトに %q が無い\n%s", want, p)
		}
	}
	for _, bad := range []string{"現在のトピック: 企業別", "3. 強み・弱み", "4. キャリアビジョン"} {
		if strings.Contains(p, bad) {
			t.Errorf("実キューと食い違う %q がある", bad)
		}
	}
	// 軸の深掘りでも同じ固定名
	d.Source, d.IsDeepening = questionSourceFollowUp, true
	if p := buildInterviewSystemPrompt("テスト株式会社", "", "", "", "general", nil, nil, 3, 0, 3, 5, 0, 180, d); !strings.Contains(p, "現在のトピック: "+axisTopicName) {
		t.Error("軸の深掘りでも固定名")
	}
	// 軸なし（Plan なし）は従来どおり標準5トピック
	std := &questionDirective{Text: "強み・弱み", Source: questionSourceTopic, Category: "強み・弱み", TotalQuestions: 5}
	if p := promptWith(std, ""); !strings.Contains(p, "3. 強み・弱み") {
		t.Error("軸なしの進め方が変わっている")
	}
}

// ---- M4 ----

func TestAxisDirectiveBlockTiesToJobSkillsAndKeepsShortReplies(t *testing.T) {
	b := buildAxisDirectiveBlock(axisByKey("technical"), axisByKey("technical").Question)
	for _, want := range []string{"求人の必須スキル", "求人の歓迎スキル", "150文字", "【応募者の履歴書】"} {
		if !strings.Contains(b, want) {
			t.Errorf("軸ブロックに %q が無い", want)
		}
	}
}

func TestBuildQuestionQueueWithAxesKeepsScoreOrder(t *testing.T) {
	axes := SelectTargetAxes(profileWith(func(p *models.CompanyWeightProfile) {
		p.GrowthOrientation, p.TechnicalOrientation, p.TeamworkOrientation, p.DetailOrientation = 95, 85, 75, 10
	}))
	var got []string
	for _, q := range BuildQuestionQueueWithAxes(1, nil, axes, nil) {
		if q.TargetAxis != nil {
			got = append(got, *q.TargetAxis)
		}
	}
	if strings.Join(got, ",") != "growth,technical,teamwork" {
		t.Fatalf("出題順 = %v, want 重視度の降順", got)
	}
}

// 軸スロットの質問・追質問には LLM 推定の「重視傾向」を渡さない。
func TestAxisSlotPromptsOmitInferredTendency(t *testing.T) {
	const info = "事業: 受託開発\n重視傾向: チームワーク、技術志向\n求人の必須スキル: Go"
	axis := &turnFixture{t: t, companyInfo: info, replies: []string{"役割を教えてください。"}, axis: true}
	axis.run()
	if sys := axis.lastSystem(); strings.Contains(sys, "重視傾向") || !strings.Contains(sys, "求人の必須スキル: Go") {
		t.Error("軸スロットのプロンプトから重視傾向が除かれていない（他の事実は残す）")
	}
	topic := &turnFixture{t: t, companyInfo: info, replies: []string{"強みを教えてください。"}}
	topic.run()
	if !strings.Contains(topic.lastSystem(), "重視傾向") {
		t.Error("標準トピックの質問は従来どおり")
	}
}

// ---- M5: 配線（fake LLM で Turn を通す） ----

type chatRecorder struct {
	mu      sync.Mutex
	systems []string
	replies []string
	n       int
}

func (c *chatRecorder) handler(sttText string, responsesText string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			_, _ = w.Write([]byte(`{"text":` + jsonString(sttText) + `}`))
		case "/chat/completions":
			b, _ := io.ReadAll(r.Body)
			var req struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.Unmarshal(b, &req)
			c.mu.Lock()
			for _, m := range req.Messages {
				if m.Role == "system" {
					c.systems = append(c.systems, m.Content)
					break
				}
			}
			reply := c.replies[min(c.n, len(c.replies)-1)]
			c.n++
			c.mu.Unlock()
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonString(reply) + `}}]}`))
		case "/responses":
			_, _ = w.Write([]byte(`{"output_text":` + jsonString(responsesText) + `}`))
		case "/audio/speech":
			_, _ = w.Write([]byte("audio"))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
}

func jsonString(s string) string { b, _ := json.Marshal(s); return string(b) }

// recordingStateRepo は更新内容を記録する質問キューのスタブ。
type recordingStateRepo struct {
	questionPlanStateRepoStub
	asked   *models.InterviewQuestionState
	updates []models.InterviewQuestionState
	created []models.InterviewQuestionState
}

func (r *recordingStateRepo) Update(s *models.InterviewQuestionState) error {
	r.updates = append(r.updates, *s)
	return nil
}
func (r *recordingStateRepo) Create(s *models.InterviewQuestionState) error {
	s.ID = 100 + uint(len(r.created))
	r.created = append(r.created, *s)
	return nil
}
func (r *recordingStateRepo) FindLatestAsked(uint) (*models.InterviewQuestionState, error) {
	return r.asked, nil
}

// lastQuestionText は最後に保存された質問文。
func (r *recordingStateRepo) lastQuestionText() string {
	if len(r.updates) == 0 {
		return ""
	}
	return r.updates[len(r.updates)-1].QuestionText
}

type turnFixture struct {
	t           *testing.T
	resume      string
	companyInfo string
	replies     []string
	axis        bool // 次の質問を軸スロットにする
	closing     bool // 次の質問が無い（終了挨拶のターン）
	rec         *chatRecorder
	reader      *resumeReaderStub
	states      *recordingStateRepo
	result      *TurnResult
	startTrn    bool
}

func (f *turnFixture) run() {
	f.t.Helper()
	f.rec = &chatRecorder{replies: f.replies}
	srv := httptest.NewServer(f.rec.handler("チームで開発しました", ""))
	f.t.Cleanup(srv.Close)

	key := "teamwork"
	f.states = &recordingStateRepo{}
	switch {
	case f.closing:
		f.states.states = []models.InterviewQuestionState{{ID: 1, Source: questionSourceTopic, Category: "逆質問", QuestionText: "逆質問", Status: questionStatusAnswered}}
	case f.axis:
		f.states.states = []models.InterviewQuestionState{{ID: 1, Source: questionSourceAxis, Category: axisByKey(key).CategoryLabel(),
			QuestionText: axisByKey(key).Question, TargetAxis: &key, Status: questionStatusPending}}
	default:
		f.states.states = []models.InterviewQuestionState{{ID: 1, Source: questionSourceTopic, Category: "強み・弱み", QuestionText: "強み・弱み", Status: questionStatusPending}}
	}
	f.reader = &resumeReaderStub{text: f.resume}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, OrganizationID: 2, Status: "in_progress", Language: "ja"}),
		&utterRepoStub{}, nil, nil, nil, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil,
	)
	svc.SetQuestionStateRepo(f.states)
	svc.SetResumeTextReader(f.reader)
	name, reading := "", ""
	if f.companyInfo != "" {
		name, reading = "テスト株式会社", "テスト"
	}
	var err error
	if f.startTrn {
		f.result, err = svc.StartTurn(context.Background(), 10, 1, name, reading, "", f.companyInfo, "general", 0, 1, 1, 0, 0)
	} else {
		f.result, err = svc.Turn(context.Background(), 10, 1, []byte("audio"), name, reading, false, "", f.companyInfo, "", 0, 1, 0, 1, 1, 0, 0)
	}
	if err != nil {
		f.t.Fatalf("Turn error: %v", err)
	}
}

func (f *turnFixture) lastSystem() string {
	if len(f.rec.systems) == 0 {
		f.t.Fatal("LLM が呼ばれていない")
	}
	return f.rec.systems[0]
}

func TestTurnInjectsResumeOnlyForAxisQuestions(t *testing.T) {
	const resume = "職歴\n2023年 ○○社でアルバイト\n氏名 山田 太郎\n生年月日 2002年4月1日"
	topic := &turnFixture{t: t, resume: resume, replies: []string{"強みを教えてください。"}}
	topic.run()
	if topic.reader.calls != 0 || strings.Contains(topic.lastSystem(), "応募者の履歴書") {
		t.Fatalf("トピック質問に履歴書が入っている (reads=%d)", topic.reader.calls)
	}

	axis := &turnFixture{t: t, resume: resume, replies: []string{"アルバイトでの役割を教えてください。"}, axis: true}
	axis.run()
	sys := axis.lastSystem()
	if axis.reader.gotUser != 10 || axis.reader.gotOr != 2 {
		t.Errorf("セッションの所有者の履歴書を読んでいない: %d/%d", axis.reader.gotUser, axis.reader.gotOr)
	}
	if !strings.Contains(sys, "○○社でアルバイト") {
		t.Error("軸質問に履歴書が入っていない")
	}
	if strings.Contains(sys, "山田") || strings.Contains(sys, "2002") {
		t.Error("属性が送信されている")
	}
}

func TestTurnReplacesDiscriminatoryReplyAndRecordsAskedText(t *testing.T) {
	// 2回（初回＋再生成）とも差別質問 -> 定型質問に差し替え、記録も更新
	f := &turnFixture{t: t, replies: []string{"ご出身はどちらですか？"}, axis: true}
	f.run()
	if f.result.AIText != genericFallbackQuestion {
		t.Fatalf("AIText = %q, want 定型質問", f.result.AIText)
	}
	if f.states.lastQuestionText() != genericFallbackQuestion {
		t.Fatalf("記録された質問文 = %q, want 実際に聞いた定型質問", f.states.lastQuestionText())
	}

	// 1回目だけ差別質問 -> 再生成の文を採用
	g := &turnFixture{t: t, replies: []string{"〇〇県のご出身ですね。", "チームでの役割を教えてください。"}}
	g.run()
	if g.result.AIText != "チームでの役割を教えてください。" {
		t.Fatalf("AIText = %q", g.result.AIText)
	}
}

func TestGuardOnlyAppliesToQuestionTurns(t *testing.T) {
	// 質問が残っていない（終了挨拶）ターンは差し替えない
	f := &turnFixture{t: t, replies: []string{"本日は地元の出身の方もお招きしました。以上で面接を終了します。"}, closing: true}
	f.run()
	if !strings.Contains(f.result.AIText, "以上で面接を終了します") || f.result.AIText == genericFallbackQuestion {
		t.Fatalf("終了挨拶が差し替えられた: %q", f.result.AIText)
	}
	if f.rec.n != 1 {
		t.Fatalf("終了挨拶で再生成された (calls=%d)", f.rec.n)
	}
}

func TestStartTurnReplacesDiscriminatoryReply(t *testing.T) {
	f := &turnFixture{t: t, replies: []string{"ご実家はどちらですか？"}, startTrn: true}
	f.run()
	if f.result.AIText != genericFallbackQuestion {
		t.Fatalf("AIText = %q", f.result.AIText)
	}
}

// 逆質問（面接官が答える）ターンは質問ではないので差し替えない。
func TestGuardSkipsReverseQuestionTopic(t *testing.T) {
	if directiveAsksQuestion(&questionDirective{Source: questionSourceTopic, Category: "逆質問"}) {
		t.Error("逆質問は質問を求めるターンではない")
	}
	if !directiveAsksQuestion(&questionDirective{Source: questionSourceAxis, Category: "x", TargetAxis: "teamwork"}) ||
		!directiveAsksQuestion(&questionDirective{Source: questionSourceTopic, Category: "強み・弱み"}) {
		t.Error("通常の質問は対象")
	}
	if directiveAsksQuestion(nil) {
		t.Error("directive なしは対象外")
	}
}

// ---- 深掘り経路 ----

func TestAdvanceQuestionPlanDeepeningCarriesAxisAndDiscardsDiscriminatory(t *testing.T) {
	key := "teamwork"
	tests := []struct {
		name      string
		llm       string
		wantLLM   bool // LLM の文が採用されるか
		wantAxis  string
		wantBrief string
	}{
		{"LLMの追質問を採用し軸を引き継ぐ", "そのときのあなたの役割を教えてください。", true, key, ""},
		{"差別質問はテンプレートにフォールバック", "ご出身はどちらですか。", false, key, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &chatRecorder{}
			srv := httptest.NewServer(rec.handler("", tt.llm))
			defer srv.Close()
			repo := &recordingStateRepo{asked: &models.InterviewQuestionState{
				ID: 9, SessionID: 5, Source: questionSourceAxis, Category: axisByKey(key).CategoryLabel(),
				QuestionText: axisByKey(key).Question, TargetAxis: &key, Status: questionStatusAsked, Depth: 0,
			}}
			repo.states = []models.InterviewQuestionState{*repo.asked}
			svc := NewInterviewService(nil, nil, nil, nil, nil, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil)
			svc.SetQuestionStateRepo(repo)

			d, err := svc.advanceQuestionPlan(context.Background(), &models.InterviewSession{ID: 5}, 0, "", "", "頑張りました。", false)
			if err != nil || d == nil || !d.IsDeepening {
				t.Fatalf("directive = %+v, %v", d, err)
			}
			if d.TargetAxis != tt.wantAxis || len(repo.created) != 1 || repo.created[0].TargetAxis == nil || *repo.created[0].TargetAxis != tt.wantAxis {
				t.Fatalf("TargetAxis を引き継いでいない: directive=%q created=%+v", d.TargetAxis, repo.created)
			}
			template := BuildFollowUpQuestionText(axisByKey(key).Question, "頑張りました。")
			if tt.wantLLM && d.Text != tt.llm {
				t.Errorf("Text = %q, want LLM の文", d.Text)
			}
			if !tt.wantLLM && d.Text != template {
				t.Errorf("Text = %q, want テンプレート %q", d.Text, template)
			}
			if repo.created[0].QuestionText != d.Text {
				t.Errorf("記録した質問文 %q と directive %q が違う", repo.created[0].QuestionText, d.Text)
			}
		})
	}
}

// 軸の追質問に渡す brief から重視傾向を除く。
func TestStripInferredTendencyLine(t *testing.T) {
	got := stripInferredTendency("企業名: テスト\n事業: 受託開発\n重視傾向: チームワーク、技術志向\n求人の必須スキル: Go")
	if strings.Contains(got, "重視傾向") || !strings.Contains(got, "求人の必須スキル: Go") || !strings.Contains(got, "事業: 受託開発") {
		t.Fatalf("got %q", got)
	}
}

// ---- 型アサーションの適合 ----

var _ jobPositionFinder = (*repositories.CompanyRepository)(nil)

// ---- #2123: 軸あり/なしで採点（レポート生成）の入力が変わらない ----

var nonceRe = regexp.MustCompile(`UNTRUSTED_[^_]+_[A-Z0-9]+_`)

func TestReportInputIsIdenticalWithAndWithoutAxes(t *testing.T) {
	key := "teamwork"
	axisStates := []models.InterviewQuestionState{
		{ID: 1, Source: questionSourceTopic, Category: "自己紹介・志望動機", QuestionText: "自己紹介・志望動機", Status: questionStatusAnswered},
		{ID: 2, Source: questionSourceAxis, Category: axisByKey(key).CategoryLabel(), QuestionText: axisByKey(key).Question, TargetAxis: &key, Status: questionStatusAnswered},
		{ID: 3, Source: questionSourceFollowUp, Category: axisByKey(key).CategoryLabel(), QuestionText: "役割は？", TargetAxis: &key, Status: questionStatusAsked},
	}
	plainStates := []models.InterviewQuestionState{
		{ID: 1, Source: questionSourceTopic, Category: "自己紹介・志望動機", QuestionText: "自己紹介・志望動機", Status: questionStatusAnswered},
		{ID: 2, Source: questionSourceTopic, Category: "強み・弱み", QuestionText: "強み・弱み", Status: questionStatusAnswered},
		{ID: 3, Source: questionSourceFollowUp, Category: "強み・弱み", QuestionText: "役割は？", Status: questionStatusAsked},
	}
	run := func(states []models.InterviewQuestionState) (requestBody string, teacherJSON string) {
		var mu sync.Mutex
		var bodies []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			bodies = append(bodies, string(b))
			mu.Unlock()
			resp, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{
				"content": reportJSON(t, genuineEvidence(), genuineTeacherEvidence())}}}})
			_, _ = w.Write(resp)
		}))
		defer srv.Close()
		sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: 7, UserID: 3, Status: "finished", Language: "ja"})
		reportRepo := &reportRepoStub{}
		svc := NewInterviewService(sessionRepo, &utterRepoStub{utterances: evidenceTestUtterances()}, reportRepo, nil, nil, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil)
		svc.SetQuestionStateRepo(&questionPlanStateRepoStub{states: states})
		if err := svc.generateReport(context.Background(), 7); err != nil {
			t.Fatalf("generateReport: %v", err)
		}
		if len(bodies) == 0 {
			t.Fatal("LLM が呼ばれていない")
		}
		// 非信頼テキストの囲みには呼び出しごとのランダムな nonce が入るので揃える
		return nonceRe.ReplaceAllString(strings.Join(bodies, "\n"), "UNTRUSTED_X_"), reportRepo.last.TeacherReportJSON
	}
	withAxis, teacherA := run(axisStates)
	without, teacherB := run(plainStates)

	if withAxis != without {
		t.Fatalf("採点の入力が軸の有無で変わった\n軸あり: %s\n軸なし: %s", withAxis, without)
	}
	if teacherA != teacherB {
		t.Fatalf("保存されるレポート（質問カバレッジ）が軸の有無で変わった\n%s\n%s", teacherA, teacherB)
	}
	for _, s := range []string{"人物像", "axis", "target_axis", axisByKey(key).Label, axisByKey(key).Question} {
		if strings.Contains(withAxis, s) || strings.Contains(teacherA, s) {
			t.Errorf("採点入力・レポートに軸由来の文字列 %q が現れる", s)
		}
	}
}
