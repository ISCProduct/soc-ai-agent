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
)

// ErrInvalidRepoFullName full_name が GitHub の "owner/repo" の形になっていないことを表す。
// コントローラはこのエラーを 400 に変える。
var ErrInvalidRepoFullName = errors.New("full_name は owner/repo の形式で指定してください")

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
// キャッシュがあれば再生成しない。forceRefresh=trueで強制再生成。
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
		if cached != nil {
			return cached, nil
		}
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
		// 名前は利用者の入力なので %q で引用する（改行で偽のログ行を作らせない）
		log.Printf("[SummarizeRepo] README fetch warning for %q: %v", fullName, err)
		readme = ""
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
		text := string(decoded)
		// 長すぎる場合は先頭4000文字に絞る
		if len(text) > 4000 {
			text = text[:4000]
		}
		return text, nil
	}
	return resp.Content, nil
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

	// FullName からユーザーIDは呼び出し元で設定するため0を仮置き
	return &models.GitHubRepoSummary{
		FullName:    fullName,
		SummaryText: payload.SummaryText,
		TechReason:  payload.TechReason,
		Challenge:   payload.Challenge,
		Achievement: payload.Achievement,
	}, nil
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
