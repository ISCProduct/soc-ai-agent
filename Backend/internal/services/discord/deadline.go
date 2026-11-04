package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// maxDeadlineDays は、いまから何日先まで期限を動かせるか。年の打ち間違い(2027 など)で
	// 期限が1年先に飛ぶのを防ぐ。
	maxDeadlineDays = 180
	maxReasonRunes  = 200
	statusOnHold    = 416451 // 保留
)

// deadlineStatusIDs は、期限を変えてよいステータス。未対応・処理中・PR確認待ち・保留。
// 処理済み以降の課題の期限を後から動かすと、完了の記録が崩れる。
var deadlineStatusIDs = []int{statusOpen, 2, statusPRReview, statusOnHold}

var (
	issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[0-9]+$`)
	dateFullPattern = regexp.MustCompile(`^(\d{4})[-/](\d{1,2})[-/](\d{1,2})$`)
	dateMDPattern   = regexp.MustCompile(`^(\d{1,2})[-/](\d{1,2})$`)
)

// DeadlineBacklog は /deadline が使う Backlog の操作。テストで差し替えるために切っている。
type DeadlineBacklog interface {
	Issue(ctx context.Context, key string) (*BacklogIssue, error)
	OpenIssuesByAssignee(ctx context.Context, assigneeID int) ([]BacklogIssue, error)
	UpdateDue(ctx context.Context, key, due, comment string) error
}

// Issue は課題を1件取得する。
func (c *BacklogClient) Issue(ctx context.Context, key string) (*BacklogIssue, error) {
	var i BacklogIssue
	if err := c.get(ctx, "/issues/"+url.PathEscape(key), nil, &i); err != nil {
		return nil, err
	}
	return &i, nil
}

// OpenIssuesByAssignee は担当者の、期限を変えてよいステータスの課題を期限順に返す。
func (c *BacklogClient) OpenIssuesByAssignee(ctx context.Context, assigneeID int) ([]BacklogIssue, error) {
	var project struct {
		ID int `json:"id"`
	}
	if err := c.get(ctx, "/projects/"+url.PathEscape(c.projectKey), nil, &project); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("projectId[]", strconv.Itoa(project.ID))
	q.Add("assigneeId[]", strconv.Itoa(assigneeID))
	for _, id := range deadlineStatusIDs {
		q.Add("statusId[]", strconv.Itoa(id))
	}
	q.Set("sort", "dueDate")
	q.Set("order", "asc")
	q.Set("count", "100")
	var issues []BacklogIssue
	if err := c.get(ctx, "/issues", q, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

// UpdateDue は期限日を更新し、同じ操作で変更履歴のコメントを残す。
func (c *BacklogClient) UpdateDue(ctx context.Context, key, due, comment string) error {
	return c.do(ctx, http.MethodPatch, "/issues/"+url.PathEscape(key), nil, url.Values{
		"dueDate": {due},
		"comment": {comment},
	}, nil)
}

// ParseDeadlineDate は "2026-11-12" "2026/11/12" "11/12" を受け付け、YYYY-MM-DD にして返す。
// today は JST の今日(YYYY-MM-DD)。過去の日付と、先すぎる日付は拒否する。
// 年を省いた日付が過ぎていた場合に翌年と解釈しない。年末の入力ミスと区別がつかないため、
// 来年の日付は年から入力してもらう。
func ParseDeadlineDate(raw, today string) (string, error) {
	raw = strings.TrimSpace(raw)
	t, err := time.Parse(dateLayout, today)
	if err != nil {
		return "", errors.New("今日の日付を判定できませんでした")
	}
	year, month, day := t.Year(), 0, 0
	if m := dateFullPattern.FindStringSubmatch(raw); m != nil {
		year, _ = strconv.Atoi(m[1])
		month, _ = strconv.Atoi(m[2])
		day, _ = strconv.Atoi(m[3])
	} else if m := dateMDPattern.FindStringSubmatch(raw); m != nil {
		month, _ = strconv.Atoi(m[1])
		day, _ = strconv.Atoi(m[2])
	} else {
		return "", errors.New("日付は 2026-11-12 または 11/12 の形式で入力してください")
	}
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// time.Date は 2/30 を 3/2 に繰り上げる。実在しない日付は拒否する。
	if int(d.Month()) != month || d.Day() != day {
		return "", fmt.Errorf("存在しない日付です: %s", raw)
	}
	out := d.Format(dateLayout)
	if out < today {
		return "", fmt.Errorf("過去の日付は指定できません（今日: %s）。来年の日付は 2027-01-05 のように年から入力してください", today)
	}
	if d.After(t.AddDate(0, 0, maxDeadlineDays)) {
		return "", fmt.Errorf("今日から %d 日より先の日付は指定できません。年の打ち間違いではありませんか", maxDeadlineDays)
	}
	return out, nil
}

// deadline は /deadline issue:KEY date:日付 reason:理由 を処理する。
//
// 誰でも実行できるコマンドなので、本人確認が要る。Discord のユーザーIDを /link の対応表で
// Backlog のユーザーに引き当て、その人が課題の担当者のときだけ反映する。名前の文字列は使わない。
func (h *Handler) deadline(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if h.deadlines == nil || h.links == nil {
		return ephemeral(linkUnavailable)
	}
	if interaction.Member == nil || interaction.Member.User == nil {
		return ephemeral("サーバー内で実行してください。")
	}
	discordID := interaction.Member.User.ID

	key := strings.ToUpper(strings.TrimSpace(FindOptionString(interaction.Data.Options, OptionNameIssue)))
	if !issueKeyPattern.MatchString(key) || !strings.HasPrefix(key, h.deadlineProject+"-") {
		return ephemeral("課題は候補から選んでください（例: " + h.deadlineProject + "-403）。")
	}
	today := todayJST()
	due, err := ParseDeadlineDate(FindOptionString(interaction.Data.Options, OptionNameDate), today)
	if err != nil {
		return ephemeral(err.Error())
	}
	reason := sanitizeReason(FindOptionString(interaction.Data.Options, OptionNameReason))
	if reason == "" {
		return ephemeral("理由を入力してください。")
	}
	if utf8.RuneCountInString(reason) > maxReasonRunes {
		return ephemeral(fmt.Sprintf("理由は %d 文字以内にしてください。", maxReasonRunes))
	}

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	links, err := h.links.List(ctx)
	if err != nil {
		log.Printf("[Discord] deadline links error: %v", err)
		return ephemeral(userFacingErrorMessage(err))
	}
	me, ok := links[discordID]
	if !ok {
		return ephemeral("あなたの Discord アカウントが Backlog に紐づいていません。管理者に `/link` を依頼してください。")
	}

	issue, err := h.deadlines.Issue(ctx, key)
	if err != nil {
		var se *backlogStatusError
		if errors.As(err, &se) && se.Status == http.StatusNotFound {
			return ephemeral(key + " が見つかりません。")
		}
		log.Printf("[Discord] deadline issue error: %v", err)
		return ephemeral(deadlineFailure(err))
	}
	if issue.Assignee == nil {
		return ephemeral(key + " は担当者が未設定のため、期限を変更できません。先に担当者を設定してください。")
	}
	if issue.Assignee.ID != me.BacklogID {
		// 他人の課題は変えられない。誰が担当かは出さない（本人確認の失敗理由を最小限にする）。
		return ephemeral(key + " の担当者は あなた ではないため、期限を変更できません。担当者に依頼してください。")
	}
	if !contains(deadlineStatusIDs, issue.Status.ID) {
		return ephemeral(key + " は完了済みに近いステータスのため、期限を変更できません。")
	}
	old := issue.Due()
	if old == due {
		return ephemeral("期限は既に " + md(due) + " です。")
	}

	comment := fmt.Sprintf("【Discord から期限を変更】\n期限: %s → %s\n理由: %s\n変更者: %s（Discord: %s）",
		dueOrNone(old), due, reason, me.Name, interaction.ActorLabel())
	log.Printf("[Discord] /deadline %s %s -> %s by %s", key, dueOrNone(old), due, interaction.ActorLabel())
	if err := h.deadlines.UpdateDue(ctx, key, due, comment); err != nil {
		log.Printf("[Discord] deadline update error: %v", err)
		return ephemeral(deadlineFailure(err))
	}

	// 公開で返す。チーム全員が期限の変更を見られるようにする。
	// 理由は利用者の入力なので、メンションが効かないようにする。
	return &InteractionResponse{
		Type: ResponseTypeChannelMessageWithSource,
		Data: &InteractionResponseData{
			Content: fmt.Sprintf("📅 <@%s> が **%s** の期限を変更しました。\n%s → **%s**\n理由: %s\n%s/view/%s",
				discordID, key, dueOrNone(md(old)), md(due), reason, h.deadlineBase, key),
			AllowedMentions: &AllowedMentions{Parse: []string{}},
		},
	}
}

// deadlineFailure は Backlog 側の失敗を、利用者に出してよい文言にする。
func deadlineFailure(err error) string {
	var se *backlogStatusError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		// PATCH が通ったかどうか分からない。成功とも失敗とも言い切らない。
		return "Backlog の応答が時間内に返りませんでした。反映されたかは Backlog で確認してください。"
	case errors.As(err, &se) && se.Status == http.StatusBadRequest:
		return "Backlog が期限を受け付けませんでした（開始日より前の期限などの可能性があります）。"
	default:
		return "Backlog への反映に失敗しました。時間を置いて再度お試しください。"
	}
}

func dueOrNone(d string) string {
	if d == "" {
		return "なし"
	}
	return d
}

func contains(ids []int, id int) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// sanitizeReason は改行を空白にし、@ を全角にする（公開メッセージと Backlog コメントの両方に出すため）。
func sanitizeReason(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ", "@", "＠").Replace(s))
}

// suggestOwnIssues は /deadline の issue 引数の候補。呼び出した本人の課題だけを出す。
func suggestOwnIssues(issues []BacklogIssue, typed string) []Choice {
	typed = strings.ToLower(strings.TrimSpace(typed))
	var out []Choice
	for _, i := range issues {
		if typed != "" && !strings.Contains(strings.ToLower(i.IssueKey+" "+i.Summary), typed) {
			continue
		}
		label := i.IssueKey + " " + oneLine(i.Summary)
		if d := i.Due(); d != "" {
			label += " (期限 " + md(d) + ")"
		}
		if r := []rune(label); len(r) > 100 { // Discord の選択肢名の上限
			label = string(r[:99]) + "…"
		}
		out = append(out, Choice{Name: label, Value: i.IssueKey})
		if len(out) == 25 { // Discord の上限
			break
		}
	}
	return out
}
