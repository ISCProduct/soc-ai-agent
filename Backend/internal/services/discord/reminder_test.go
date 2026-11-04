package discord

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func issue(key, summary, due string, status int, assigneeID int, assignee string) BacklogIssue {
	i := BacklogIssue{IssueKey: key, Summary: summary, DueDate: due + "T00:00:00Z"}
	i.Status.ID = status
	if assigneeID != 0 {
		i.Assignee = &struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}{ID: assigneeID, Name: assignee}
	}
	return i
}

var remindLinks = map[string]Link{"111": {BacklogID: 1, Name: "大橋和幸"}}

const base = "https://x.backlog.com"

func TestBuildReminder_対象が無ければ何も作らない(t *testing.T) {
	if got := BuildReminder(nil, remindLinks, "2026-11-04", base, nil); got != nil {
		t.Errorf("nil でない: %v", got)
	}
}

func TestBuildReminder_期限の表示と担当者のまとめ(t *testing.T) {
	issues := []BacklogIssue{
		issue("P-1", "切れている", "2026-11-02", 2, 1, "大橋和幸"),
		issue("P-2", "今日", "2026-11-04", 1, 1, "大橋和幸"),
		issue("P-3", "明日", "2026-11-05", 2, 2, "山田太郎"),
		issue("P-4", "明後日", "2026-11-06", 2, 0, ""),
	}
	out := strings.Join(BuildReminder(issues, remindLinks, "2026-11-04", base, nil), "\n")
	for _, want := range []string{
		"<@111>",                // 登録済みはメンション
		"期限切れ 2日",               // 期限切れ
		"今日が期限",                 // 当日
		"**山田太郎** (Discord未登録)", // 未登録は名前だけ
		"**担当者なし**",
		"明日が期限",
		"2日後",
		"⚠️ **未着手**", // 未対応(1)だけに付く
	} {
		if !strings.Contains(out, want) {
			t.Errorf("本文に %q が無い:\n%s", want, out)
		}
	}
	if strings.Count(out, "未着手") != 1 {
		t.Errorf("未着手の付与が未対応の課題だけでない:\n%s", out)
	}
	if strings.Count(out, "<@111>") != 1 {
		t.Errorf("同じ担当者が1ブロックにまとまっていない:\n%s", out)
	}
	if !strings.Contains(out, "(<https://x.backlog.com/view/P-1>)") {
		t.Errorf("リンクが埋め込み無効の形式でない:\n%s", out)
	}
}

func TestBuildReminder_件名のメンションを無効にする(t *testing.T) {
	out := strings.Join(BuildReminder([]BacklogIssue{issue("P-1", "@everyone 確認\n改行", "2026-11-04", 2, 1, "大橋和幸")}, remindLinks, "2026-11-04", base, nil), "")
	if strings.Contains(out, "@everyone") || strings.Contains(out, "確認\n改行") {
		t.Errorf("件名がそのまま出ている: %s", out)
	}
}

func TestBuildReminder_長いときは担当者の区切りで分割する(t *testing.T) {
	var issues []BacklogIssue
	for i := 0; i < 40; i++ {
		issues = append(issues, issue("P-"+strings.Repeat("9", 3)+string(rune('A'+i%26)), strings.Repeat("あ", 60), "2026-11-04", 2, 100+i, "メンバー"+string(rune('A'+i%26))+strings.Repeat("x", i)))
	}
	msgs := BuildReminder(issues, nil, "2026-11-04", base, nil)
	if len(msgs) < 2 {
		t.Fatalf("分割されない: %d通", len(msgs))
	}
	assertWithinDiscordLimit(t, msgs) // Discord の上限は文字数。len はバイト数で比べられない
}

type issuesStub struct {
	issues []BacklogIssue
	err    error
	until  string
}

func (s *issuesStub) OpenIssuesDueBy(_ context.Context, until string) ([]BacklogIssue, error) {
	s.until = until
	return s.issues, s.err
}

type linksStub struct{ m map[string]Link }

func (l linksStub) List(context.Context) (map[string]Link, error) { return l.m, nil }

func newReminder(hook string, st *issuesStub) *ReminderService {
	return &ReminderService{issues: st, links: linksStub{remindLinks}, webhookURL: hook, issueBase: base, http: &http.Client{}}
}

func TestReminder_Run_投稿内容とallowed_mentions(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		mu.Lock()
		bodies = append(bodies, m)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := &issuesStub{issues: []BacklogIssue{issue("P-1", "x", "2026-11-04", 2, 1, "大橋和幸"), issue("P-2", "y", "2026-11-04", 2, 2, "山田太郎")}}
	res, err := newReminder(srv.URL, st).Run(context.Background(), false)
	if err != nil || !res.Posted || len(bodies) != 1 {
		t.Fatalf("投稿されない: res=%+v err=%v bodies=%d", res, err, len(bodies))
	}
	am := bodies[0]["allowed_mentions"].(map[string]any)
	users := am["users"].([]any)
	if len(users) != 1 || users[0] != "111" {
		t.Errorf("メンション許可が登録済みの担当者だけでない: %v", users)
	}
	if _, hasParse := am["parse"]; hasParse {
		t.Errorf("parse を指定すると @everyone が有効になりうる: %v", am)
	}
	if st.until == "" {
		t.Errorf("期限の上限が渡っていない")
	}
}

func TestReminder_Run_dryRunは投稿しない(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	st := &issuesStub{issues: []BacklogIssue{issue("P-1", "x", "2026-11-04", 2, 1, "大橋和幸")}}
	res, err := newReminder(srv.URL, st).Run(context.Background(), true)
	if err != nil || called || res.Posted || len(res.Messages) != 1 {
		t.Errorf("dry_run で投稿された/本文が無い: res=%+v err=%v called=%v", res, err, called)
	}
}

func TestReminder_Run_対象が無ければ投稿しない(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	res, err := newReminder(srv.URL, &issuesStub{}).Run(context.Background(), false)
	if err != nil || called || res.Posted {
		t.Errorf("対象ゼロで投稿した: res=%+v err=%v", res, err)
	}
}

func TestReminder_Run_Backlog失敗はエラーで投稿しない(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	_, err := newReminder(srv.URL, &issuesStub{err: errors.New("boom")}).Run(context.Background(), false)
	if err == nil || called {
		t.Errorf("失敗を握りつぶした/投稿した: err=%v called=%v", err, called)
	}
}

func TestReminder_投稿失敗のエラーにWebhookのトークンを含めない(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + "/api/webhooks/1/SECRETTOKEN"
	srv.Close() // 接続失敗
	st := &issuesStub{issues: []BacklogIssue{issue("P-1", "x", "2026-11-04", 2, 1, "大橋和幸")}}
	_, err := newReminder(url, st).Run(context.Background(), false)
	if err == nil || strings.Contains(err.Error(), "SECRETTOKEN") {
		t.Errorf("エラーにトークンが含まれる、またはエラーが無い: %v", err)
	}
}

func TestReminder_Webhook形式が不正なら無効にする(t *testing.T) {
	t.Setenv("DISCORD_REMINDER_WEBHOOK_URL", "https://evil.example.com/api/webhooks/1/x")
	if r := NewReminderServiceFromEnv(&BacklogClient{}, &LinkService{}); r != nil {
		t.Errorf("不正なURLで有効になった")
	}
	t.Setenv("DISCORD_REMINDER_WEBHOOK_URL", "https://discord.com/api/webhooks/123/abc_DEF-9")
	if r := NewReminderServiceFromEnv(&BacklogClient{}, &LinkService{}); r == nil {
		t.Errorf("正しいURLで無効になった")
	}
}

func TestBacklogClient_OpenIssuesDueBy_検索条件(t *testing.T) {
	var gotIssues map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/projects/P"):
			_, _ = w.Write([]byte(`{"id":792541}`))
		case strings.HasSuffix(r.URL.Path, "/issues"):
			gotIssues = r.URL.Query()
			_, _ = w.Write([]byte(`[{"issueKey":"P-1","summary":"s","dueDate":"2026-11-04T00:00:00Z","status":{"id":2},"assignee":{"id":1,"name":"大橋和幸"}}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := &BacklogClient{baseURL: srv.URL, apiKey: "K", projectKey: "P", http: &http.Client{}}
	issues, err := c.OpenIssuesDueBy(context.Background(), "2026-11-06")
	if err != nil || len(issues) != 1 || issues[0].Due() != "2026-11-04" {
		t.Fatalf("取得できない: %v %v", issues, err)
	}
	if gotIssues["projectId[]"][0] != "792541" || gotIssues["dueDateUntil"][0] != "2026-11-06" {
		t.Errorf("絞り込み条件が違う: %v", gotIssues)
	}
	if got := strings.Join(gotIssues["statusId[]"], ","); got != "1,2,416375" {
		t.Errorf("対象ステータスが違う(保留・処理済み以降を含めない): %s", got)
	}
}

// reviewerLinks は 大橋(Backlog ID 1) が Discord 111、山田(ID 2) が Discord 222。
var reviewerLinks = map[string]Link{"111": {BacklogID: 1, Name: "大橋和幸"}, "222": {BacklogID: 2, Name: "山田太郎"}}

func TestBuildReminder_PR確認待ちはレビュアーにメンションする(t *testing.T) {
	issues := []BacklogIssue{
		issue("P-1", "レビュー待ち", "2026-11-04", statusPRReview, 2, "山田太郎"),
		issue("P-2", "処理中", "2026-11-04", 2, 2, "山田太郎"),
	}
	out := strings.Join(BuildReminder(issues, reviewerLinks, "2026-11-04", base, []int{1}), "\n")
	if !strings.Contains(out, "👀 **レビュー依頼** <@111> PR確認待ちです") {
		t.Errorf("レビュアーへの依頼が無い:\n%s", out)
	}
	// 依頼ブロックに載るのはPR確認待ちだけ。処理中は載らない。
	review := out[strings.Index(out, "👀"):]
	if !strings.Contains(review, "P-1") || strings.Contains(review, "P-2") {
		t.Errorf("依頼ブロックの対象がPR確認待ちだけでない:\n%s", review)
	}
	if !strings.Contains(review, "担当: 山田太郎") || strings.Contains(review, "<@222>") {
		t.Errorf("担当者はメンションせず名前だけで出す:\n%s", review)
	}
}

func TestBuildReminder_PR確認待ちが無ければレビュー依頼を出さない(t *testing.T) {
	out := strings.Join(BuildReminder([]BacklogIssue{issue("P-2", "x", "2026-11-04", 2, 2, "山田太郎")}, reviewerLinks, "2026-11-04", base, []int{1}), "")
	if strings.Contains(out, "レビュー依頼") || strings.Contains(out, "<@111>") {
		t.Errorf("PR確認待ちが無いのにレビュー依頼が出た:\n%s", out)
	}
}

func TestBuildReminder_レビュアー本人の課題は依頼の対象にしない(t *testing.T) {
	out := strings.Join(BuildReminder([]BacklogIssue{issue("P-1", "x", "2026-11-04", statusPRReview, 1, "大橋和幸")}, reviewerLinks, "2026-11-04", base, []int{1}), "")
	if strings.Contains(out, "レビュー依頼") {
		t.Errorf("自分のPRを自分に依頼している:\n%s", out)
	}
}

func TestBuildReminder_レビュアー未登録なら黙って落とさずリンク登録を促す(t *testing.T) {
	out := strings.Join(BuildReminder([]BacklogIssue{issue("P-1", "x", "2026-11-04", statusPRReview, 2, "山田太郎")}, map[string]Link{"222": {BacklogID: 2, Name: "山田太郎"}}, "2026-11-04", base, []int{1}), "")
	if !strings.Contains(out, "レビュアー(Backlog ID 1)** (Discord未登録") {
		t.Errorf("未登録の通知が無い:\n%s", out)
	}
}

func TestBuildReminder_レビュアー未設定なら従来どおり(t *testing.T) {
	out := strings.Join(BuildReminder([]BacklogIssue{issue("P-1", "x", "2026-11-04", statusPRReview, 2, "山田太郎")}, reviewerLinks, "2026-11-04", base, nil), "")
	if strings.Contains(out, "レビュー依頼") {
		t.Errorf("レビュアー未設定なのに依頼が出た:\n%s", out)
	}
}

func TestMentionIDs_レビュアーはPR確認待ちがある日だけ許可する(t *testing.T) {
	withPR := []BacklogIssue{issue("P-1", "x", "2026-11-04", statusPRReview, 2, "山田太郎")}
	if got := strings.Join(mentionIDs(withPR, reviewerLinks, []int{1}), ","); got != "111,222" {
		t.Errorf("レビュアーと担当者の許可が揃っていない: %s", got)
	}
	noPR := []BacklogIssue{issue("P-1", "x", "2026-11-04", 2, 2, "山田太郎")}
	if got := strings.Join(mentionIDs(noPR, reviewerLinks, []int{1}), ","); got != "222" {
		t.Errorf("PR確認待ちが無いのにレビュアーを許可している: %s", got)
	}
}

func TestParseIDs(t *testing.T) {
	if got := parseIDs(" 1, 2 ,x,,3"); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("parseIDs = %v", got)
	}
	if parseIDs("") != nil {
		t.Errorf("空は nil")
	}
}

// 本番の実データで1通目が2101文字になり、Discord に拒否される欠陥があった。
// 1人の担当者に課題が多いと、担当者単位の分割では1ブロックが上限を超える。
func manyIssues(n int, assigneeID int, assignee string, status int) []BacklogIssue {
	var out []BacklogIssue
	for i := 0; i < n; i++ {
		key := "SOCAIAGENT-" + strconv.Itoa(400+i)
		out = append(out, issue(key, strings.Repeat("長い件名", 15), "2026-10-20", status, assigneeID, assignee))
	}
	return out
}

func assertWithinDiscordLimit(t *testing.T, msgs []string) {
	t.Helper()
	for i, m := range msgs {
		if n := utf8.RuneCountInString(m); n > 2000 {
			t.Errorf("%d通目が %d文字で Discord の上限(2000)を超える", i+1, n)
		}
	}
}

func TestBuildReminder_1人の課題が多くても1通を上限内に収める(t *testing.T) {
	issues := manyIssues(20, 1, "大橋和幸", 2)
	msgs := BuildReminder(issues, remindLinks, "2026-11-04", base, nil)
	if len(msgs) < 2 {
		t.Fatalf("分割されない: %d通", len(msgs))
	}
	assertWithinDiscordLimit(t, msgs)
	all := strings.Join(msgs, "\n")
	for _, i := range issues {
		if !strings.Contains(all, i.IssueKey+">") {
			t.Errorf("%s が本文から落ちた", i.IssueKey)
		}
	}
	// 続きの通にも担当者の見出し(メンション)が付く。誰の課題か分からなくならない。
	for i, m := range msgs {
		if !strings.Contains(m, "<@111>") {
			t.Errorf("%d通目に担当者の見出しが無い", i+1)
		}
	}
}

func TestBuildReminder_レビュー依頼が多くても上限内に収める(t *testing.T) {
	issues := manyIssues(25, 2, "山田太郎", statusPRReview)
	msgs := BuildReminder(issues, reviewerLinks, "2026-11-04", base, []int{1})
	assertWithinDiscordLimit(t, msgs)
	review := 0
	for _, m := range msgs {
		if strings.Contains(m, "👀 **レビュー依頼** <@111>") {
			review++
		}
	}
	if review < 2 {
		t.Errorf("レビュー依頼が分割され、続きにも見出しが付くはず: 見出し%d回", review)
	}
	if got := strings.Count(strings.Join(msgs, ""), "担当: 山田太郎"); got != 25 {
		t.Errorf("レビュー依頼の行が %d/25 件しか残っていない", got)
	}
}

func TestBuildReminder_複数レビュアーの未登録を1人ずつ促す(t *testing.T) {
	// 大橋(ID 1)は登録済み、ID 9 は未登録。
	out := strings.Join(BuildReminder([]BacklogIssue{issue("P-1", "x", "2026-11-04", statusPRReview, 2, "山田太郎")}, reviewerLinks, "2026-11-04", base, []int{1, 9}), "")
	if !strings.Contains(out, "<@111>") || !strings.Contains(out, "レビュアー(Backlog ID 9)** (Discord未登録") {
		t.Errorf("登録済みへのメンションと、未登録の案内が両方出ていない:\n%s", out)
	}
}

func TestReminder_Run_取得上限に達したら注意書きを出す(t *testing.T) {
	st := &issuesStub{issues: manyIssues(issueFetchLimit, 1, "大橋和幸", 2)}
	res, err := newReminder("http://unused.invalid", st).Run(context.Background(), true)
	if err != nil || !strings.Contains(res.Messages[len(res.Messages)-1], "取得上限") {
		t.Errorf("上限に達したのに注意書きが無い: err=%v", err)
	}
	st = &issuesStub{issues: manyIssues(3, 1, "大橋和幸", 2)}
	res, _ = newReminder("http://unused.invalid", st).Run(context.Background(), true)
	if strings.Contains(strings.Join(res.Messages, ""), "取得上限") {
		t.Errorf("上限に達していないのに注意書きが出た")
	}
}
