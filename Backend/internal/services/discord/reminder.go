package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 期限リマインドの対象にするステータス。
// 未対応(1) 処理中(2) PR確認待ち(416375)。保留・処理済み・ステージング反映以降は、
// 期限を過ぎていても「着手を促す」意味が無いので対象外にする。
const (
	statusOpen = 1
	// remindSoonDays は「期限が近い」とみなす日数（当日を含めて2日後まで）。
	remindSoonDays = 2
	// 1通の上限は2000文字。メンションや改行の余裕を見て切る。
	reminderChunkChars = 1800
	issueURLFormat     = "%s/view/%s"
)

var defaultRemindStatusIDs = []int{statusOpen, 2, 416375}

// BacklogIssue は期限リマインドに使う課題の最小情報。
type BacklogIssue struct {
	IssueKey string `json:"issueKey"`
	Summary  string `json:"summary"`
	DueDate  string `json:"dueDate"`
	Status   struct {
		ID int `json:"id"`
	} `json:"status"`
	Assignee *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"assignee"`
}

// Due は "2026-11-10T00:00:00Z" の日付部分（YYYY-MM-DD）。
func (i BacklogIssue) Due() string {
	if len(i.DueDate) >= 10 {
		return i.DueDate[:10]
	}
	return ""
}

// IssueLister はテストで差し替えるための面。
type IssueLister interface {
	OpenIssuesDueBy(ctx context.Context, until string) ([]BacklogIssue, error)
}

// OpenIssuesDueBy は期限が until(YYYY-MM-DD) 以前の、対象ステータスの課題を期限順に返す。
// 期限切れも含む。期限が未設定の課題は Backlog 側の絞り込みで除かれる。
func (c *BacklogClient) OpenIssuesDueBy(ctx context.Context, until string) ([]BacklogIssue, error) {
	var project struct {
		ID int `json:"id"`
	}
	if err := c.get(ctx, "/projects/"+url.PathEscape(c.projectKey), nil, &project); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("projectId[]", strconv.Itoa(project.ID))
	for _, id := range defaultRemindStatusIDs {
		q.Add("statusId[]", strconv.Itoa(id))
	}
	q.Set("dueDateUntil", until)
	q.Set("sort", "dueDate")
	q.Set("order", "asc")
	q.Set("count", "100") // 上限。超えたら古い期限切れから100件が出る
	var issues []BacklogIssue
	if err := c.get(ctx, "/issues", q, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

// LinkLister は対応表の読み出し面。
type LinkLister interface {
	List(ctx context.Context) (map[string]Link, error)
}

// ReminderService は期限が近い・切れた課題を担当者ごとにまとめて Discord へ投稿する。
type ReminderService struct {
	issues     IssueLister
	links      LinkLister
	webhookURL string
	issueBase  string // 課題リンクの基点（https://iwasaki-ac.backlog.com）
	http       *http.Client
}

var webhookPattern = regexp.MustCompile(`^https://discord\.com/api/(?:v10/)?webhooks/[0-9]+/[A-Za-z0-9_-]+$`)

// NewReminderServiceFromEnv は Webhook が無ければ nil を返す（機能を無効にする）。
func NewReminderServiceFromEnv(backlog *BacklogClient, links *LinkService) *ReminderService {
	hook := os.Getenv("DISCORD_REMINDER_WEBHOOK_URL")
	if hook == "" || backlog == nil || links == nil {
		return nil
	}
	if !webhookPattern.MatchString(hook) {
		// 値そのもの(トークン入り)はログに出さない。無効にして呼び出し時に気づかせる。
		log.Printf("[Reminder] DISCORD_REMINDER_WEBHOOK_URL が Discord の Webhook URL の形式ではありません")
		return nil
	}
	return &ReminderService{
		issues:     backlog,
		links:      links,
		webhookURL: hook,
		issueBase:  backlog.baseURL,
		http: &http.Client{
			Timeout: 5 * time.Second,
			// リダイレクト先へ Webhook のトークンごと送らない。
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// ReminderResult は実行結果。DryRun のときは Messages が投稿予定の本文。
type ReminderResult struct {
	Issues   int      `json:"issues"`
	Messages []string `json:"messages"`
	Posted   bool     `json:"posted"`
}

// Run は取得 → 本文作成 → 投稿までを行う。対象が無ければ何も投稿しない。
func (s *ReminderService) Run(ctx context.Context, dryRun bool) (*ReminderResult, error) {
	now := time.Now().In(jst())
	today := now.Format(dateLayout)
	until := now.AddDate(0, 0, remindSoonDays).Format(dateLayout)

	issues, err := s.issues.OpenIssuesDueBy(ctx, until)
	if err != nil {
		return nil, err
	}
	links, err := s.links.List(ctx)
	if err != nil {
		return nil, err
	}
	msgs := BuildReminder(issues, links, today, s.issueBase)
	res := &ReminderResult{Issues: len(issues), Messages: msgs}
	if dryRun || len(msgs) == 0 {
		return res, nil
	}
	for _, m := range msgs {
		if err := s.post(ctx, m, mentionIDs(issues, links)); err != nil {
			return res, err
		}
		res.Posted = true
	}
	return res, nil
}

// mentionIDs は本文に出てくる Discord ID。allowed_mentions に渡し、
// 課題名などに混ざった @everyone や別人のメンションを無効にする。
func mentionIDs(issues []BacklogIssue, links map[string]Link) []string {
	byBacklog := map[int]string{}
	for id, l := range links {
		byBacklog[l.BacklogID] = id
	}
	seen := map[string]bool{}
	var ids []string
	for _, i := range issues {
		if i.Assignee == nil {
			continue
		}
		if d, ok := byBacklog[i.Assignee.ID]; ok && !seen[d] {
			seen[d] = true
			ids = append(ids, d)
		}
	}
	sort.Strings(ids)
	return ids
}

func (s *ReminderService) post(ctx context.Context, content string, mentions []string) error {
	if mentions == nil {
		mentions = []string{}
	}
	body, err := json.Marshal(map[string]any{
		"username":         "期限リマインダー",
		"content":          content,
		"allowed_mentions": map[string]any{"users": mentions},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL+"?wait=true", bytes.NewReader(body))
	if err != nil {
		return errors.New("Discord 投稿のリクエスト作成に失敗しました")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		// url.Error には Webhook URL（トークン入り）が含まれるので理由だけ残す。
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("Discord への投稿に失敗しました: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Discord への投稿が %d を返しました", resp.StatusCode)
	}
	return nil
}

// BuildReminder は担当者ごとにまとめた投稿本文を返す。対象が無ければ nil。
// 1通に収まらない場合は担当者の区切りで分割する。
func BuildReminder(issues []BacklogIssue, links map[string]Link, today, issueBase string) []string {
	if len(issues) == 0 {
		return nil
	}
	byBacklog := map[int]string{}
	for id, l := range links {
		byBacklog[l.BacklogID] = id
	}

	type group struct {
		label string
		lines []string
	}
	groups := map[string]*group{}
	var order []string
	for _, i := range issues {
		key, label := "none", "**担当者なし**"
		if i.Assignee != nil {
			key = strconv.Itoa(i.Assignee.ID)
			if d, ok := byBacklog[i.Assignee.ID]; ok {
				label = "<@" + d + ">"
			} else {
				// 未登録の人はメンションできない。名前だけ出し、/link を促す。
				label = "**" + i.Assignee.Name + "** (Discord未登録)"
			}
		}
		g, ok := groups[key]
		if !ok {
			g = &group{label: label}
			groups[key] = g
			order = append(order, key)
		}
		g.lines = append(g.lines, issueLine(i, today, issueBase))
	}

	header := "📅 **Backlog 期限リマインド** (" + md(today) + ")\n"
	footer := "\n着手していない・間に合わない場合は、このチャンネルで相談してください。"
	var msgs []string
	cur := header
	for _, k := range order {
		g := groups[k]
		block := g.label + "\n" + strings.Join(g.lines, "\n") + "\n"
		if len(cur)+len(block)+len(footer) > reminderChunkChars && cur != header {
			msgs = append(msgs, cur)
			cur = ""
		}
		cur += block
	}
	return append(msgs, cur+footer)
}

func md(date string) string {
	t, err := time.Parse(dateLayout, date)
	if err != nil {
		return date
	}
	return t.Format("1/2") + "(" + []string{"日", "月", "火", "水", "木", "金", "土"}[t.Weekday()] + ")"
}

func issueLine(i BacklogIssue, today, issueBase string) string {
	due := i.Due()
	var when string
	switch d := dayDiff(today, due); {
	case due == "":
		when = "期限不明"
	case d < 0:
		when = fmt.Sprintf("🔴 期限切れ %d日 (%s)", -d, md(due))
	case d == 0:
		when = "🟠 今日が期限"
	case d == 1:
		when = "🟡 明日が期限 (" + md(due) + ")"
	default:
		when = fmt.Sprintf("🟡 %d日後 (%s)", d, md(due))
	}
	line := fmt.Sprintf("・[%s](<%s>) %s — %s", i.IssueKey, fmt.Sprintf(issueURLFormat, issueBase, i.IssueKey), oneLine(i.Summary), when)
	if i.Status.ID == statusOpen {
		line += " ⚠️ **未着手**"
	}
	return line
}

// oneLine は件名を1行・短くし、@everyone などが効かないよう @ を無害化する。
func oneLine(s string) string {
	s = strings.NewReplacer("\n", " ", "\r", " ", "@", "＠").Replace(s)
	if r := []rune(s); len(r) > 60 {
		s = string(r[:60]) + "…"
	}
	return s
}

// dayDiff は to - from の日数。どちらかが不正なら 0。
func dayDiff(from, to string) int {
	a, err1 := time.Parse(dateLayout, from)
	b, err2 := time.Parse(dateLayout, to)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(b.Sub(a).Hours() / 24)
}
