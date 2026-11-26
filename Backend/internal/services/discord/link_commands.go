package discord

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const linkUnavailable = "現在この機能は利用できません(未設定)。"

// link は /link member:@x backlog:名前 で Discord ユーザーと Backlog ユーザーを対応づける。
//
// 管理者ロールだけに許す。誰でも書けると、他人になりすまして通知の宛先や
// /deadline の本人確認を乗っ取れる。
func (h *Handler) link(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if !h.hasAllowedRole(interaction) {
		return ephemeral("このコマンドを実行する権限がありません。")
	}
	if h.links == nil || h.backlog == nil {
		return ephemeral(linkUnavailable)
	}
	memberID := strings.TrimSpace(FindOptionString(interaction.Data.Options, OptionNameMember))
	if memberID == "" {
		return ephemeral("メンバーを指定してください。")
	}

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	users, err := h.backlog.Users(ctx)
	if err != nil {
		log.Printf("[Discord] backlog users error: %v", err)
		return ephemeral("Backlog のメンバー一覧を取得できませんでした。時間を置いて再度お試しください。")
	}
	u, err := ResolveBacklogUser(users, FindOptionString(interaction.Data.Options, OptionNameBacklog))
	if err != nil {
		return ephemeral(err.Error())
	}

	githubLogin, err := h.githubLoginFor(ctx, memberID, interaction.Data.Options)
	if err != nil {
		return ephemeral(err.Error())
	}

	log.Printf("[Discord] /link %s -> backlog:%d by %s", memberID, u.ID, interaction.ActorLabel())
	if err := h.links.Set(ctx, memberID, Link{BacklogID: u.ID, Name: u.Name, GitHub: githubLogin}); err != nil {
		log.Printf("[Discord] link set error: %v", err)
		return ephemeral(userFacingErrorMessage(err))
	}
	msg := "✅ <@" + memberID + "> を Backlog の「" + u.Name + "」に紐づけました。"
	if githubLogin != "" {
		msg += " GitHub は「" + githubLogin + "」です。"
	}
	return ephemeral(msg)
}

// githubLoginFor は /link の任意の github 引数を読む。
// 引数が無い再登録では、以前のログイン名を残す（Backlog 名だけ直したときに PR のメンションが消えないように）。
func (h *Handler) githubLoginFor(ctx context.Context, memberID string, options []CommandOption) (string, error) {
	raw, ok := findOption(options, OptionNameGitHub)
	if !ok {
		existing, err := h.links.List(ctx)
		if err != nil {
			log.Printf("[Discord] link list for github error: %v", err)
			return "", errors.New("対応表を読めませんでした。時間を置いて再度お試しください。")
		}
		return existing[memberID].GitHub, nil
	}
	login, err := NormalizeGitHubLogin(raw)
	if err != nil {
		return "", err
	}
	return login, nil
}

func (h *Handler) unlink(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if !h.hasAllowedRole(interaction) {
		return ephemeral("このコマンドを実行する権限がありません。")
	}
	if h.links == nil {
		return ephemeral(linkUnavailable)
	}
	memberID := strings.TrimSpace(FindOptionString(interaction.Data.Options, OptionNameMember))
	if memberID == "" {
		return ephemeral("メンバーを指定してください。")
	}

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	log.Printf("[Discord] /unlink %s by %s", memberID, interaction.ActorLabel())
	removed, err := h.links.Remove(ctx, memberID)
	if err != nil {
		log.Printf("[Discord] link remove error: %v", err)
		return ephemeral(userFacingErrorMessage(err))
	}
	if !removed {
		return ephemeral("<@" + memberID + "> は登録されていません。")
	}
	return ephemeral("✅ <@" + memberID + "> の紐づけを解除しました。")
}

func (h *Handler) linkList(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if !h.hasAllowedRole(interaction) {
		return ephemeral("このコマンドを実行する権限がありません。")
	}
	if h.links == nil {
		return ephemeral(linkUnavailable)
	}

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	links, err := h.links.List(ctx)
	if err != nil {
		log.Printf("[Discord] link list error: %v", err)
		return ephemeral(userFacingErrorMessage(err))
	}
	if len(links) == 0 {
		return ephemeral("登録されているメンバーはいません。/link で登録してください。")
	}
	pages := linkListPages(links)
	page := findOptionInt(interaction.Data.Options, OptionNamePage)
	if page < 1 {
		page = 1
	}
	if page > len(pages) {
		return ephemeral("ページがありません。1 から " + strconv.Itoa(len(pages)) + " までです。")
	}
	return ephemeral(pages[page-1])
}

// discordMessageLimit は Discord のメッセージ本文の上限（文字数）。
const discordMessageLimit = 2000

// linkListPages は一覧を 2000 文字以内のページに分ける。1通にまとめると、人数が増えたとき Discord が拒否する。
func linkListPages(links map[string]Link) []string {
	title := "Discord ↔ Backlog の紐づけ(" + strconv.Itoa(len(links)) + "人)"
	var lines []string
	for _, id := range SortedIDs(links) {
		line := "・<@" + id + "> ↔ " + links[id].Name
		if links[id].GitHub != "" {
			line += "（GitHub: " + links[id].GitHub + "）"
		}
		lines = append(lines, line)
	}
	const pageNote = 16 // 「（999/999ページ）」の余裕
	var pages [][]string
	var cur []string
	used := utf8.RuneCountInString(title) + 1
	for _, ln := range lines {
		if utf8.RuneCountInString(ln) > discordMessageLimit-pageNote-used && len(cur) == 0 {
			ln = trimRunes(ln, discordMessageLimit-pageNote-utf8.RuneCountInString(title)-2)
		}
		n := utf8.RuneCountInString(ln) + 1
		if len(cur) > 0 && used+n+pageNote > discordMessageLimit {
			pages = append(pages, cur)
			cur = nil
			used = utf8.RuneCountInString(title) + 1
		}
		cur = append(cur, ln)
		used += n
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	out := make([]string, len(pages))
	for i, lines := range pages {
		body := title
		if len(pages) > 1 {
			body += "（" + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(pages)) + "ページ）"
		}
		out[i] = body + "\n" + strings.Join(lines, "\n")
	}
	return out
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if n < 1 || len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func findOptionInt(options []CommandOption, name string) int {
	for _, o := range options {
		if o.Name != name {
			continue
		}
		switch v := o.Value.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		}
	}
	return 0
}

// handleAutocomplete は /link の backlog 引数に Backlog のメンバー名を候補で出す。
// 候補は誰が開いても見えてしまうので、実行できるロールの人にだけ返す。
func (h *Handler) handleAutocomplete(ctx context.Context, interaction *Interaction) *InteractionResponse {
	empty := &InteractionResponse{
		Type: ResponseTypeAutocompleteResult,
		Data: &InteractionResponseData{Choices: []Choice{}},
	}
	if interaction.Data != nil && interaction.Data.Name == CommandNameDeadline {
		return h.deadlineAutocomplete(ctx, interaction, empty)
	}
	if interaction.Data == nil || interaction.Data.Name != CommandNameLink ||
		!h.hasAllowedRole(interaction) || h.backlog == nil {
		return empty
	}
	typed := ""
	for _, o := range interaction.Data.Options {
		if o.Focused && o.Name == OptionNameBacklog {
			typed, _ = o.Value.(string)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()
	users, err := h.backlog.Users(ctx)
	if err != nil {
		log.Printf("[Discord] backlog users (autocomplete) error: %v", err)
		return empty
	}
	for _, name := range SuggestBacklogUsers(users, typed) {
		empty.Data.Choices = append(empty.Data.Choices, Choice{Name: name, Value: name})
	}
	return empty
}

// deadlineAutocomplete は /deadline の issue 引数に、呼び出した本人の課題を候補で出す。
// 他人の課題は出さない。紐づけが無い人には何も出さない。
func (h *Handler) deadlineAutocomplete(ctx context.Context, interaction *Interaction, empty *InteractionResponse) *InteractionResponse {
	if h.deadlines == nil || h.links == nil || interaction.Member == nil || interaction.Member.User == nil {
		return empty
	}
	typed := ""
	for _, o := range interaction.Data.Options {
		if o.Focused && o.Name == OptionNameIssue {
			typed, _ = o.Value.(string)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()
	links, err := h.links.List(ctx)
	if err != nil {
		log.Printf("[Discord] deadline autocomplete links error: %v", err)
		return empty
	}
	me, ok := links[interaction.Member.User.ID]
	if !ok {
		return empty
	}
	issues, err := h.deadlines.OpenIssuesByAssignee(ctx, me.BacklogID)
	if err != nil {
		log.Printf("[Discord] deadline autocomplete issues error: %v", err)
		return empty
	}
	empty.Data.Choices = suggestOwnIssues(issues, typed)
	if empty.Data.Choices == nil {
		empty.Data.Choices = []Choice{}
	}
	return empty
}

// githubLoginPattern は GitHub のログイン名。先頭と末尾のハイフンは認めない。
var githubLoginPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

// NormalizeGitHubLogin は @ 付きや空白を外し、空なら未設定として通す。
func NormalizeGitHubLogin(raw string) (string, error) {
	login := strings.TrimSpace(raw)
	login = strings.TrimPrefix(login, "@")
	login = strings.TrimSpace(login)
	if login == "" {
		return "", nil
	}
	if !githubLoginPattern.MatchString(login) {
		return "", errors.New("GitHub のログイン名が不正です。例: octocat")
	}
	return login, nil
}

func findOption(options []CommandOption, name string) (string, bool) {
	for _, o := range options {
		if o.Name != name {
			continue
		}
		v, _ := o.Value.(string)
		return v, true
	}
	return "", false
}
