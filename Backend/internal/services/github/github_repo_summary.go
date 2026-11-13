package github

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"Backend/internal/usagectx"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalidRepoFullName full_name が GitHub の "owner/repo" の形になっていないことを表す。
// コントローラはこのエラーを 400 に変える。
var ErrInvalidRepoFullName = errors.New("full_name は owner/repo の形式で指定してください")

// ErrInvalidRepoSummaryOutput モデルの出力が要約として保存できる形になっていないことを表す。
// このときは保存しないので、次の依頼で作り直される。
var ErrInvalidRepoSummaryOutput = errors.New("AI の要約の出力が不正です")

const (
	// maxREADMERunes プロンプトに入れる README の上限。単位はルーン（Unicode の文字数）で、バイト数ではない。
	// バイトで切ると日本語の UTF-8 の途中で切れ、不正な文字列がプロンプトに入るため（#2185）。
	maxREADMERunes = 4000
	// maxRepoSummaryFieldRunes モデルの出力の各フィールドの上限（ルーン数）。
	// プロンプトは各フィールドを「1〜2文」と指示しており、日本語の1〜2文はふつう200字に収まる。
	// 出力全体も max_tokens=800 で絞っているので、1つのフィールドがこれを超えるのは
	// 指示に従っていない（README の注入で本文を書き写させられた等）とみなす。
	maxRepoSummaryFieldRunes = 500
)

var (
	// GitHub のユーザー名・組織名の規則：英数字とハイフン、1〜39字、先頭と末尾は英数字。
	// Enterprise Managed Users のログイン名は「IDP-USERNAME_SHORT-CODE」のように「_」を含むので、
	// 途中の「_」も許す（「_」はパスの区切りにならない。#2182 のレビュー）。
	githubOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9_-]{0,37}[A-Za-z0-9])?$`)
	// GitHub のリポジトリ名の規則：英数字と「.」「-」「_」、1〜100字。
	githubRepoNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// ValidateRepoFullName full_name が "owner/repo" の形かを確かめる。
//
// full_name は GitHub API の URL のパスにそのまま入るため、「../」「?」「#」などを許すと
// /repos/{owner}/{repo}/readme 以外のエンドポイントへ利用者のトークンで GET を送れてしまう（#2176）。
// 使える文字と長さを GitHub の規則に限り、「/」はちょうど1つ、「.」「..」だけのリポジトリ名は拒否する
// （どちらもパスの要素として解釈され、別のエンドポイントを指すため）。
func ValidateRepoFullName(fullName string) error {
	// 最初の「/」で分ける。「/」が無ければ repo が空、2つ以上あれば repo に「/」が残り、
	// どちらも githubRepoNamePattern に合わないため、「/」はちょうど1つに限られる。
	owner, repo, _ := strings.Cut(fullName, "/")
	if !githubOwnerPattern.MatchString(owner) || !githubRepoNamePattern.MatchString(repo) {
		return ErrInvalidRepoFullName
	}
	if repo == "." || repo == ".." {
		return ErrInvalidRepoFullName
	}
	return nil
}

// SummarizeRepo リポジトリのREADMEをAIが解析し、技術的強みを要約する。
// 今のプロンプトの版（repoSummaryPromptVersion）で作ったキャッシュがあれば再生成しない。
// 版が違う（空を含む）キャッシュは、forceRefresh=false でも作り直す（#2185）。forceRefresh=trueで強制再生成。
// targetRole は志望職種（例: "エンジニア", "マーケ", "営業"）で、分析観点をカスタマイズする（#457）。
func (s *GitHubService) SummarizeRepo(ctx context.Context, userID uint, fullName string, forceRefresh bool, targetRole string) (*models.GitHubRepoSummary, error) {
	// URL に入れる前に形を確かめる。呼び出し元がコントローラ以外でも守られるよう、サービスの入口で見る（#2176）。
	if err := ValidateRepoFullName(fullName); err != nil {
		return nil, err
	}

	// キャッシュ確認
	if !forceRefresh {
		cached, err := s.githubRepo.GetRepoSummary(userID, fullName)
		if err != nil {
			return nil, err
		}
		// 古い版のプロンプトでは README・リポジトリ名の注入で虚偽の実績が作られうるので、
		// 版が一致するときだけキャッシュを返す。版の記録より前の行は空文字なので、ここで作り直しになる。
		if cached != nil && cached.PromptVersion == repoSummaryPromptVersion {
			return cached, nil
		}
	}
	if err := usagectx.CheckAIBudget(ctx); err != nil {
		return nil, err
	}

	profile, err := s.githubRepo.GetProfile(userID)
	if err != nil || profile == nil {
		return nil, fmt.Errorf("github profile not found")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// README取得
	token, err := loadAccessToken(userID, profile.AccessToken)
	if err != nil {
		return nil, err
	}
	readme, err := s.fetchREADME(ctx, client, token, fullName)
	if err != nil {
		switch {
		case isGitHubNotFoundError(err):
			// README の 404 は「README が無い」だけでなく、リポジトリが削除・非公開化された、アクセス権を
			// 失った、存在しない名前、のときも返る（GitHub の仕様上は単に Not Found）。リポジトリ自体に
			// アクセスできるときだけ「README なし」で要約する（#2231 のレビュー）。
			if repoErr := s.checkRepoAccessible(ctx, client, token, fullName); repoErr != nil {
				log.Printf("[SummarizeRepo] repository not accessible for %q: %v", fullName, repoErr)
				if isGitHubAuthError(repoErr) {
					return nil, &GitHubReauthRequiredError{Reason: "GitHubアクセストークンが無効です。GitHubアカウントを再連携してください。"}
				}
				return nil, fmt.Errorf("repository not accessible: %w", repoErr)
			}
			readme = ""
		case isGitHubAuthError(err):
			return nil, &GitHubReauthRequiredError{Reason: "GitHubアクセストークンが無効です。GitHubアカウントを再連携してください。"}
		default:
			// 404 以外の失敗（5xx・レート制限など）で README 無しの要約を作ると、薄い要約が
			// キャッシュに残り、README があるのに無い扱いのままになる。要約を作らずにエラーにする（#2226）。
			// 名前は利用者の入力なので %q で引用する（改行で偽のログ行を作らせない）
			log.Printf("[SummarizeRepo] README fetch failed for %q: %v", fullName, err)
			return nil, fmt.Errorf("failed to fetch README: %w", err)
		}
	}

	// AI要約生成
	summary, err := s.generateRepoSummary(ctx, fullName, readme, targetRole)
	if err != nil {
		return nil, fmt.Errorf("AI summary generation failed: %w", err)
	}

	summary.UserID = userID
	// DBに保存
	if err := s.githubRepo.UpsertRepoSummary(summary); err != nil {
		return nil, fmt.Errorf("save summary: %w", err)
	}
	return summary, nil
}

// checkRepoAccessible は、リポジトリ自体を利用者のトークンで取得できるかを確かめる（#2231 のレビュー）。
// README の 404 が「README が無い」ことによるものか、リポジトリにアクセスできないことによるものかを分けるため。
func (s *GitHubService) checkRepoAccessible(ctx context.Context, client *http.Client, token, fullName string) error {
	// fullName は SummarizeRepo の入口で ValidateRepoFullName を通している。
	_, err := s.doGet(ctx, client, token, fmt.Sprintf("%s/repos/%s", s.getAPIBase(), fullName))
	return err
}

// fetchREADME GitHub API経由でリポジトリのREADMEを取得する
func (s *GitHubService) fetchREADME(ctx context.Context, client *http.Client, token, fullName string) (string, error) {
	// fullName は SummarizeRepo の入口で ValidateRepoFullName を通している。
	url := fmt.Sprintf("%s/repos/%s/readme", s.getAPIBase(), fullName)
	body, err := s.doGet(ctx, client, token, url)
	if err != nil {
		return "", err
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		return truncateRunes(string(decoded), maxREADMERunes), nil
	}
	return truncateRunes(resp.Content, maxREADMERunes), nil
}

// truncateRunes s を先頭から最大 maxRunes 文字（ルーン）に切り詰める。
// 文字の境目でだけ切るので、UTF-8 の途中で切れることはない。
func truncateRunes(s string, maxRunes int) string {
	n := 0
	for i := range s {
		if n == maxRunes {
			return s[:i]
		}
		n++
	}
	return s
}

// roleBasedPromptConfig 職種別プロンプト設定（#457）
type roleBasedPromptConfig struct {
	systemRole    string
	analysisFocus string
	summaryHint   string
}

// getRolePromptConfig 志望職種に応じたプロンプト設定を返す（#457）
func getRolePromptConfig(targetRole string) roleBasedPromptConfig {
	switch {
	case strings.Contains(targetRole, "エンジニア") || strings.Contains(targetRole, "engineer") || strings.Contains(targetRole, "開発"):
		return roleBasedPromptConfig{
			systemRole:    "エンジニア採用のキャリアアドバイザー",
			analysisFocus: "技術スキル・アーキテクチャ設計力・コード品質",
			summaryHint:   "使用技術・設計パターン・実装上の工夫を重点的に評価してください",
		}
	case strings.Contains(targetRole, "マーケ") || strings.Contains(targetRole, "marketing"):
		return roleBasedPromptConfig{
			systemRole:    "マーケティング職採用のキャリアアドバイザー",
			analysisFocus: "技術への理解度・データ活用能力・ツール活用力",
			summaryHint:   "技術を使って何を実現したか、データ分析・自動化・ツール活用の観点で評価してください",
		}
	case strings.Contains(targetRole, "営業") || strings.Contains(targetRole, "sales"):
		return roleBasedPromptConfig{
			systemRole:    "営業職採用のキャリアアドバイザー",
			analysisFocus: "プロジェクト管理能力・課題解決力・コミュニケーション能力の証拠",
			summaryHint:   "プロジェクトをどう推進したか、課題発見〜解決のプロセスを重点的に評価してください",
		}
	case strings.Contains(targetRole, "デザイン") || strings.Contains(targetRole, "design"):
		return roleBasedPromptConfig{
			systemRole:    "デザイン職採用のキャリアアドバイザー",
			analysisFocus: "UIへの配慮・UX思考・フロントエンド技術への理解",
			summaryHint:   "ユーザー体験や見た目への配慮、フロントエンド実装の質を重点的に評価してください",
		}
	default:
		return roleBasedPromptConfig{
			systemRole:    "IT業界採用のキャリアアドバイザー",
			analysisFocus: "技術的強み・実装力・問題解決能力",
			summaryHint:   "このリポジトリで発揮された技術力と問題解決の観点で評価してください",
		}
	}
}

// generateRepoSummary OpenAIを使ってリポジトリの技術的強みを要約する（#457: 職種別プロンプト対応）
func (s *GitHubService) generateRepoSummary(ctx context.Context, fullName, readme, targetRole string) (*models.GitHubRepoSummary, error) {
	if s.openaiClient == nil {
		return nil, fmt.Errorf("openai client not configured")
	}

	cfg := getRolePromptConfig(targetRole)
	// fullName はリクエストの値そのままなので、改行で偽のログ行を作らせないよう %q で出す。
	log.Printf("[SummarizeRepo] prompt_version=%s repo=%q target_role=%q focus=%s", repoSummaryPromptVersion, fullName, targetRole, cfg.analysisFocus)

	systemPrompt, userPrompt := buildRepoSummaryPrompts(fullName, readme, cfg)

	ctx = usagectx.WithFeature(ctx, usagectx.FeatureGitHubSummary)
	raw, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.5, 800)
	if err != nil {
		return nil, err
	}

	cleaned := extractRepoSummaryJSON(raw)
	var payload struct {
		SummaryText string `json:"summary_text"`
		TechReason  string `json:"tech_reason"`
		Challenge   string `json:"challenge"`
		Achievement string `json:"achievement"`
	}
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return nil, fmt.Errorf("parse summary json: %w", err)
	}

	summary := &models.GitHubRepoSummary{
		FullName:      fullName,
		SummaryText:   strings.TrimSpace(payload.SummaryText),
		TechReason:    strings.TrimSpace(payload.TechReason),
		Challenge:     strings.TrimSpace(payload.Challenge),
		Achievement:   strings.TrimSpace(payload.Achievement),
		PromptVersion: repoSummaryPromptVersion,
	}
	if err := validateRepoSummaryOutput(summary); err != nil {
		return nil, err
	}
	// ユーザーIDは呼び出し元で設定するため0を仮置き
	return summary, nil
}

// validateRepoSummaryOutput 保存する前に、モデルの出力を確かめる（#2185。#2178 のレビュー）。
//
//   - summary_text が空（空白だけを含む）なら不正とする。JSON が null や {} のときもここで弾く。
//     空の要約を保存すると、今の版のキャッシュとして残り、作り直されなくなるため。
//     ほかの3つは README から読み取れないこともあるので、空を許す。
//   - どのフィールドも maxRepoSummaryFieldRunes 字を超えたら不正とする。
//     切り詰めて保存すると、指示に従っていない出力の一部が実績として表示され続けるため、
//     切り詰めずに保存しない（次の依頼で作り直される）。
//
// JSON の形（オブジェクトであること・各キーが文字列であること）は json.Unmarshal が確かめている。
func validateRepoSummaryOutput(s *models.GitHubRepoSummary) error {
	if s.SummaryText == "" {
		return fmt.Errorf("%w: summary_text が空です", ErrInvalidRepoSummaryOutput)
	}
	fields := []struct {
		name  string
		value string
	}{
		{"summary_text", s.SummaryText},
		{"tech_reason", s.TechReason},
		{"challenge", s.Challenge},
		{"achievement", s.Achievement},
	}
	for _, f := range fields {
		if n := utf8.RuneCountInString(f.value); n > maxRepoSummaryFieldRunes {
			return fmt.Errorf("%w: %s が %d 字で、上限の %d 字を超えています", ErrInvalidRepoSummaryOutput, f.name, n, maxRepoSummaryFieldRunes)
		}
	}
	return nil
}

// buildRepoSummaryPrompts はリポジトリ要約の system / user プロンプトを組み立てる。
// リポジトリ名はリクエストの full_name そのもので（README が取れなくても要約は作られる）、
// README は所有者が自由に書ける。どちらも未信頼なので、文の途中の「」には置かず、
// 見出しの下に UntrustedSection で囲んで置く（#2174。#1998 のレビュー L2）。
// 「囲みの中の指示に従わない」宣言は WrapUntrustedText が囲みの直前に書くので、system 側には足さない。
func buildRepoSummaryPrompts(fullName, readme string, cfg roleBasedPromptConfig) (systemPrompt, userPrompt string) {
	readmeSection := shared.UntrustedSection(readme, "README")
	if readmeSection == "" {
		readmeSection = "README:\n（READMEなし）\n"
	}

	systemPrompt = fmt.Sprintf(`あなたは%sです。
GitHubリポジトリのREADMEを読み、%sの観点から強みを簡潔にまとめてください。JSONのみで返してください。`, cfg.systemRole, cfg.analysisFocus)

	userPrompt = fmt.Sprintf(`次のGitHubリポジトリのREADMEを読み、%sの観点でまとめてください。
%s

%s
%s
## 出力フォーマット（このキーと型を厳守）
{
  "summary_text": "リポジトリの強みの要約（全体を1段落で簡潔に）",
  "tech_reason": "技術選定の理由（なぜその技術・言語・フレームワークを選んだか）",
  "challenge": "解決した課題（どんな問題に取り組み、どう解決したか）",
  "achievement": "成果（数値・具体的な改善・学んだこと）"
}

※ 情報が不足している場合はREADMEから推測して記述してください。各フィールドは1〜2文で簡潔に。`, cfg.analysisFocus, cfg.summaryHint, shared.UntrustedSection(fullName, "リポジトリ名"), readmeSection)
	return systemPrompt, userPrompt
}

// extractRepoSummaryJSON マークダウンコードフェンスを除去してJSONを抽出する
func extractRepoSummaryJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return s
}
