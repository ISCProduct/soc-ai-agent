package discord

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// Discord のメッセージ本文は 2000 文字まで。超えると応答が拒否されて一覧が見えなくなる。
// linkListNoticeReserve は「ほか N 人」の案内を入れる余白。
const (
	linkListMaxChars      = 2000
	linkListNoticeReserve = 60
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
	header := "Discord ↔ Backlog の紐づけ(" + strconv.Itoa(len(links)) + "人)\n"
	var b strings.Builder
	b.WriteString(header)
	ids := SortedIDs(links)
	for i, id := range ids {
		line := "・<@" + id + "> ↔ " + links[id].Name
		if links[id].GitHub != "" {
			line += "（GitHub: " + links[id].GitHub + "）"
		}
		line += "\n"
		// 切り捨ての案内（最大 linkListNoticeReserve 文字）を残せる範囲までで止める。
		if len([]rune(b.String()))+len([]rune(line)) > linkListMaxChars-linkListNoticeReserve {
			b.WriteString("…ほか" + strconv.Itoa(len(ids)-i) + "人は表示しきれません\n")
			break
		}
		b.WriteString(line)
	}
	return ephemeral(b.String())
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
