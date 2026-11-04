package discord

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseDeadlineDate(t *testing.T) {
	const today = "2026-11-04"
	tests := []struct {
		name, raw, want string
		wantErr         string
	}{
		{"年あり ハイフン", "2026-11-12", "2026-11-12", ""},
		{"年あり スラッシュ", "2026/11/5", "2026-11-05", ""},
		{"年なし", "11/12", "2026-11-12", ""},
		{"年なし 1桁", "12/1", "2026-12-01", ""},
		{"今日は指定できる", "11/4", "2026-11-04", ""},
		{"前後の空白", "  11/12 ", "2026-11-12", ""},
		{"過去", "11/3", "", "過去"},
		{"年なしで過ぎた日は翌年にしない", "1/5", "", "過去"},
		{"来年は年から指定できる", "2027-01-05", "2027-01-05", ""},
		{"実在しない日", "2/30", "", "存在しない"},
		{"実在しない日 年あり", "2026-11-31", "", "存在しない"},
		{"形式違い", "来週", "", "形式"},
		{"空", "", "", "形式"},
		{"先すぎる(年の打ち間違い)", "2027-11-12", "", "180"},
		{"ちょうど180日先", "2027-05-03", "2027-05-03", ""},
		{"181日先", "2027-05-04", "", "180"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDeadlineDate(tt.raw, today)
			if tt.wantErr == "" {
				if err != nil || got != tt.want {
					t.Fatalf("got %q, %v want %q", got, err, tt.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("エラーに %q が含まれない: %v", tt.wantErr, err)
			}
		})
	}
}

type deadlineStub struct {
	issue                  *BacklogIssue
	issueErr, updErr       error
	own                    []BacklogIssue
	updKey, updDue, updMsg string
	updCalls               int
}

func (d *deadlineStub) Issue(context.Context, string) (*BacklogIssue, error) {
	return d.issue, d.issueErr
}
func (d *deadlineStub) OpenIssuesByAssignee(context.Context, int) ([]BacklogIssue, error) {
	return d.own, nil
}
func (d *deadlineStub) UpdateDue(_ context.Context, key, due, comment string) error {
	d.updCalls++
	d.updKey, d.updDue, d.updMsg = key, due, comment
	return d.updErr
}

// 大橋(Discord 111, Backlog 1) が自分の課題 SOCAIAGENT-403(処理中・期限 11/5)を変更する想定。
func newDeadlineHandler(st *deadlineStub, mem *memSSM) *Handler {
	return NewHandler(nil, nil, noopDispatcher(), "", "role-allowed").
		WithLinks(&LinkService{client: mem, paramName: "/p/links"}, backlogStub{}).
		WithDeadline(st, "SOCAIAGENT", "https://x.backlog.com")
}

func linkedMem() *memSSM {
	v := `{"111":{"id":1,"name":"大橋和幸"},"222":{"id":2,"name":"山田太郎"}}`
	return &memSSM{value: &v}
}

func iptr(key, summary, due string, status, assigneeID int, assignee string) *BacklogIssue {
	i := issue(key, summary, due, status, assigneeID, assignee)
	return &i
}

func ownIssue() *BacklogIssue {
	return iptr("SOCAIAGENT-403", "応募ボタン", "2026-11-05", 2, 1, "大橋和幸")
}

func deadlineCmd(userID, key, date, reason string) *Interaction {
	return &Interaction{
		Type:   InteractionTypeApplicationCommand,
		Member: &Member{User: &User{ID: userID, Username: "u" + userID}},
		Data: &InteractionData{Name: CommandNameDeadline, Options: []CommandOption{
			opt(OptionNameIssue, key), opt(OptionNameDate, date), opt(OptionNameReason, reason),
		}},
	}
}

func futureDate() string { return todayJSTPlus(7) }

func TestDeadline_本人の課題の期限を変更しコメントを残す(t *testing.T) {
	st := &deadlineStub{issue: ownIssue()}
	h := newDeadlineHandler(st, linkedMem())
	date := futureDate()
	resp := h.handleCommand(context.Background(), deadlineCmd("111", "socaiagent-403", date, "レビュー待ち"))

	if st.updCalls != 1 || st.updKey != "SOCAIAGENT-403" || st.updDue != date {
		t.Fatalf("更新内容が違う: calls=%d key=%s due=%s", st.updCalls, st.updKey, st.updDue)
	}
	for _, want := range []string{"2026-11-05 → " + date, "理由: レビュー待ち", "大橋和幸"} {
		if !strings.Contains(st.updMsg, want) {
			t.Errorf("コメントに %q が無い: %s", want, st.updMsg)
		}
	}
	if resp.Data.Flags == EphemeralFlag {
		t.Errorf("成功は公開で返す")
	}
	if resp.Data.AllowedMentions == nil || resp.Data.AllowedMentions.Parse == nil || len(resp.Data.AllowedMentions.Parse) != 0 {
		t.Errorf("公開メッセージはメンション通知を無効にする(parse は空配列で送る): %+v", resp.Data.AllowedMentions)
	}
	b, _ := json.Marshal(resp.Data)
	if !strings.Contains(string(b), `"allowed_mentions":{"parse":[]}`) {
		t.Errorf("JSON に parse:[] が出ていない(null や省略だと全メンションが有効になる): %s", b)
	}
}

func TestDeadline_拒否するケースでは何も書き込まない(t *testing.T) {
	cases := []struct {
		name     string
		userID   string
		issue    *BacklogIssue
		key      string
		date     string
		reason   string
		wantText string
	}{
		{"紐づけの無い人", "999", ownIssue(), "SOCAIAGENT-403", futureDate(), "x", "紐づいていません"},
		{"他人の課題", "222", ownIssue(), "SOCAIAGENT-403", futureDate(), "x", "あなた ではない"},
		{"担当者なし", "111", iptr("SOCAIAGENT-403", "x", "2026-11-05", 2, 0, ""), "SOCAIAGENT-403", futureDate(), "x", "担当者が未設定"},
		{"処理済み", "111", iptr("SOCAIAGENT-403", "x", "2026-11-05", 3, 1, "大橋和幸"), "SOCAIAGENT-403", futureDate(), "x", "ステータス"},
		{"ステージング反映", "111", iptr("SOCAIAGENT-403", "x", "2026-11-05", 437943, 1, "大橋和幸"), "SOCAIAGENT-403", futureDate(), "x", "ステータス"},
		{"別プロジェクトの課題", "111", ownIssue(), "OTHER-1", futureDate(), "x", "候補から選んで"},
		{"課題キーの形式違い", "111", ownIssue(), "応募ボタン", futureDate(), "x", "候補から選んで"},
		{"過去の日付", "111", ownIssue(), "SOCAIAGENT-403", "2020-01-01", "x", "過去"},
		{"理由が空", "111", ownIssue(), "SOCAIAGENT-403", futureDate(), "  ", "理由を入力"},
		{"理由が長すぎる", "111", ownIssue(), "SOCAIAGENT-403", futureDate(), strings.Repeat("あ", 201), "200 文字"},
		{"期限が同じ", "111", iptr("SOCAIAGENT-403", "x", futureDate(), 2, 1, "大橋和幸"), "SOCAIAGENT-403", futureDate(), "x", "既に"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := &deadlineStub{issue: c.issue}
			resp := newDeadlineHandler(st, linkedMem()).handleCommand(context.Background(), deadlineCmd(c.userID, c.key, c.date, c.reason))
			if st.updCalls != 0 {
				t.Fatalf("拒否すべきなのに更新した")
			}
			if resp.Data.Flags != EphemeralFlag || !strings.Contains(resp.Data.Content, c.wantText) {
				t.Errorf("本人にだけ「%s」を返すはず: flags=%d %s", c.wantText, resp.Data.Flags, resp.Data.Content)
			}
		})
	}
}

func TestDeadline_期限が未設定の課題にも設定できる(t *testing.T) {
	st := &deadlineStub{issue: &BacklogIssue{IssueKey: "SOCAIAGENT-403", Summary: "x", Status: ownIssue().Status, Assignee: ownIssue().Assignee}}
	h := newDeadlineHandler(st, linkedMem())
	h.handleCommand(context.Background(), deadlineCmd("111", "SOCAIAGENT-403", futureDate(), "x"))
	if st.updCalls != 1 || !strings.Contains(st.updMsg, "なし →") {
		t.Errorf("旧期限なしの表記が無い: %s", st.updMsg)
	}
}

func TestDeadline_理由の改行とメンションを無効化する(t *testing.T) {
	st := &deadlineStub{issue: ownIssue()}
	resp := newDeadlineHandler(st, linkedMem()).handleCommand(context.Background(), deadlineCmd("111", "SOCAIAGENT-403", futureDate(), "@everyone 確認\n待ち"))
	if strings.Contains(resp.Data.Content, "@everyone") || strings.Contains(st.updMsg, "@everyone") || strings.Contains(st.updMsg, "確認\n待ち") {
		t.Errorf("理由がそのまま出ている:\n%s\n%s", resp.Data.Content, st.updMsg)
	}
}

func TestDeadline_Backlog失敗の内部情報を返さず種類で言い分ける(t *testing.T) {
	run := func(issueErr, updErr error) string {
		st := &deadlineStub{issue: ownIssue(), issueErr: issueErr, updErr: updErr}
		if issueErr != nil {
			st.issue = nil
		}
		return contentOf(newDeadlineHandler(st, linkedMem()).handleCommand(context.Background(), deadlineCmd("111", "SOCAIAGENT-403", futureDate(), "x")))
	}
	if got := run(&backlogStatusError{Status: 404}, nil); !strings.Contains(got, "見つかりません") {
		t.Errorf("404: %s", got)
	}
	if got := run(nil, &backlogStatusError{Status: 400}); !strings.Contains(got, "受け付けませんでした") {
		t.Errorf("400: %s", got)
	}
	if got := run(nil, context.DeadlineExceeded); !strings.Contains(got, "反映されたかは Backlog で確認") {
		t.Errorf("タイムアウトは成否不明と伝える: %s", got)
	}
	if got := run(nil, errors.New("secret-internal-detail")); strings.Contains(got, "secret-internal-detail") {
		t.Errorf("内部エラーが漏れた: %s", got)
	}
}

func TestDeadline_未設定なら利用できない(t *testing.T) {
	h := NewHandler(nil, nil, noopDispatcher(), "", "r")
	if got := contentOf(h.handleCommand(context.Background(), deadlineCmd("111", "SOCAIAGENT-403", futureDate(), "x"))); !strings.Contains(got, "利用できません") {
		t.Errorf("未設定の応答: %s", got)
	}
}

func TestDeadlineAutocomplete_本人の課題だけを候補に出す(t *testing.T) {
	st := &deadlineStub{own: []BacklogIssue{
		issue("SOCAIAGENT-403", "応募ボタン", "2026-11-05", 2, 1, "大橋和幸"),
		issue("SOCAIAGENT-404", "別の件", "", 1, 1, "大橋和幸"),
	}}
	h := newDeadlineHandler(st, linkedMem())
	ac := func(userID, typed string) []Choice {
		i := deadlineCmd(userID, "", "", "")
		i.Type = InteractionTypeAutocomplete
		i.Data.Options = []CommandOption{{Name: OptionNameIssue, Value: typed, Focused: true}}
		return h.handleAutocomplete(context.Background(), i).Data.Choices
	}
	if got := ac("111", ""); len(got) != 2 || got[0].Value != "SOCAIAGENT-403" || !strings.Contains(got[0].Name, "期限 11/5") {
		t.Errorf("候補: %+v", got)
	}
	if got := ac("111", "応募"); len(got) != 1 || got[0].Value != "SOCAIAGENT-403" {
		t.Errorf("絞り込み: %+v", got)
	}
	if got := ac("999", ""); len(got) != 0 {
		t.Errorf("紐づけの無い人に候補が出た: %+v", got)
	}
}

func TestBacklogClient_UpdateDue_PATCHとフォームを送りキーを漏らさない(t *testing.T) {
	var method, path string
	var q, form url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, q = r.Method, r.URL.Path, r.URL.Query()
		b, _ := io.ReadAll(r.Body)
		form, _ = url.ParseQuery(string(b))
		_, _ = w.Write([]byte(`{}`))
	}))
	c := &BacklogClient{baseURL: srv.URL, apiKey: "SECRETKEY123", projectKey: "P", http: &http.Client{}}
	if err := c.UpdateDue(context.Background(), "P-1", "2026-11-12", "理由 & 記号=?"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPatch || !strings.HasSuffix(path, "/issues/P-1") {
		t.Errorf("PATCH /issues/P-1 でない: %s %s", method, path)
	}
	if q.Get("apiKey") != "SECRETKEY123" || form.Get("apiKey") != "" {
		t.Errorf("apiKey はクエリで渡し、ボディには入れない: q=%v form=%v", q, form)
	}
	if form.Get("dueDate") != "2026-11-12" || form.Get("comment") != "理由 & 記号=?" {
		t.Errorf("フォームの値が違う: %v", form)
	}
	srv.Close()
	err := c.UpdateDue(context.Background(), "P-1", "2026-11-12", "x")
	if err == nil || strings.Contains(err.Error(), "SECRETKEY123") {
		t.Errorf("エラーにキーが含まれる、またはエラーが無い: %v", err)
	}
}

func TestBacklogClient_ステータスエラーは型で判別できる(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := &BacklogClient{baseURL: srv.URL, apiKey: "K", projectKey: "P", http: &http.Client{}}
	_, err := c.Issue(context.Background(), "P-1")
	var se *backlogStatusError
	if !errors.As(err, &se) || se.Status != 404 {
		t.Errorf("404 を型で取れない: %v", err)
	}
}
