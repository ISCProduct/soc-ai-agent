package release

import (
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/usagectx"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrReleaseNoteLLMClientNil = errors.New("openai client is nil")

const releaseNoteSummarySystemPrompt = `あなたはtoC向けSaaSのリリースノート編集者です。開発者向けのPRタイトル・本文から、エンドユーザー向けにやさしい日本語で1〜2文の更新内容を要約し、対象読者(audience)を分類してください。

読み手はパソコンやITに詳しくない学生・教員です。専門用語を知らない人が読んで意味が分かる文章にしてください。

書き方のルール:
- 「〜できるようになりました」「〜が見やすくなりました」のように、利用者から見て何が変わったかを書く
- カタカナの専門用語・英字の略語は使わない
  悪い例: API / バッチ / キャッシュ / セッション / ログ / デプロイ / パフォーマンス / UI / マッチングアルゴリズム
  言い換え例: 「読み込みが速くなりました」「入力した内容が保存されるようになりました」「おすすめの企業がより合うようになりました」
- 社内の仕組みや画面の裏側の話は書かない。利用者が画面で見て分かることだけを書く
- 「対応しました」「改善しました」だけで終わらせず、利用者にとって何が良くなるかを書く

タイトルも同じ基準で、専門用語を使わずに書いてください。

audience は以下のいずれかにしてください:
- "student": 就活中の学生ユーザーが画面や機能として体験できる新機能・UI改善・不具合修正・パフォーマンス向上
- "teacher": 教員が使う機能（学生の指導・面談・進捗確認など教員向け画面）の変更
- "admin": システム管理者・学校管理者が管理画面（admin）で使う機能の変更
- "all": 学生・教員・管理者の全員が同じ画面で体験できる変更のみ。開発者向けの変更を all にしてはいけない

以下は誰の目にも触れない変更のため、summary は必ず空文字にしてください（開発部門・運用担当者のみが関わる内容はユーザーに提供しない）:
- インフラ・運用・デプロイ関連（AWS/ECS/Terraform構成、起動停止設定、コスト最適化、サーバー移行など）
- 社内ツール連携・開発フロー（Notion/Backlog/Slack通知連携、CI/CD、コードレビュー体制、レビュー指摘対応など）
- リファクタリング・依存関係更新・セキュリティパッチ（ユーザーの体験に見た目上の変化がないもの）
- 開発者向けドキュメント整備

summaryを書いてよいのは、いずれかのaudienceのユーザーが実際に体験できる新機能・UI改善・不具合修正・パフォーマンス向上のみです。迷ったら空文字にしてください。

JSON形式のみで出力し、他のテキストは含めないでください。`

const releaseNoteSummarySchema = `{"title": "ユーザー向けの短いタイトル(20字以内)", "summary": "ユーザー向けの説明文(1〜2文)。誰にも関係ない変更なら空文字", "audience": "student/teacher/admin/allのいずれか"}`

var validReleaseNoteAudiences = map[string]bool{
	"student": true,
	"teacher": true,
	"admin":   true,
	"all":     true,
}

var developerOnlyReleaseNoteNeedles = []string{
	"terraform",
	"github actions",
	".github/workflows",
	"coderabbit",
	"fargate",
	"docker compose",
	"infra/",
	"レビュー指摘",
	"インフラ構成",
	"デプロイパイプライン",
	"ci/cd",
	// 運用操作。Discordからの環境起動/停止や稼働日設定は開発・運用担当者のものであり、
	// 学生や教員には関係がない(#1289)。
	"discord",
	"staging",
	"ステージング",
	"デプロイ",
	"cloudwatch",
	"lambda",
	"rds",
	"ecs",
	"稼働日",
	"起動/停止",
}

func isDeveloperOnlyReleaseNote(title, body string) bool {
	lowerTitle := strings.ToLower(strings.TrimSpace(title))

	// release 傘PRは中身を要約させない。
	//
	// main へ直接マージされるのは傘PRだけで、その本文は運用担当者向けに書かれる。
	// 「起動ジョブの失敗通知が初めて有効になる」「スキーマ変更あり」といった記述が
	// そのまま要約され、学生向けの更新情報として表示された。
	//
	// 以前はタイトルだけニードル判定していたが、傘PRのタイトルは
	// 「本番反映 — SRE整備（通知・レート制限・可観測性）とAI利用量計測ほか34件」の
	// ように中身の要約になっており、運用語が入らないことがある。実際にすり抜けた。
	//
	// 更新情報は中身の個別PRから作る（.github/scripts/collect_whats_new_sources.py）。
	// 傘PRが渡ってきた場合はここで落とす。
	if strings.HasPrefix(lowerTitle, "release") {
		return true
	}

	// 開発・運用の接頭辞。個別PRのタイトルには主題が書かれるため判定できる。
	//
	// docs: は入れない。「docs: ユーザー向けヘルプを更新」のように
	// 利用者に関係する変更が入ることがあるため、本文の判定に委ねる。
	//
	// スコープ付き（refactor(backend): など）も拾う。Conventional Commits では
	// 接頭辞のあとに括弧でスコープが入るため、そこを外してから比較する。
	// "fix(ops): 起動判定を直す" のようにスコープ側が運用のこともある。
	// 型だけ見ると fix として通ってしまうため、スコープも判定する。
	typ, scope := commitType(lowerTitle)
	if isDeveloperOnlyCommitType(typ) || isDeveloperOnlyScope(scope) {
		return true
	}
	return containsDeveloperOnlyNeedle(strings.ToLower(title + "\n" + body))
}

// commitType は Conventional Commits の型とスコープを返す。
// "refactor(backend): x" なら ("refactor", "backend")、型が無ければ空文字。
func commitType(lowerTitle string) (typ, scope string) {
	colon := strings.Index(lowerTitle, ":")
	if colon <= 0 {
		return "", ""
	}
	head := lowerTitle[:colon]
	if paren := strings.Index(head, "("); paren > 0 {
		if close := strings.Index(head[paren:], ")"); close > 0 {
			scope = strings.TrimSpace(head[paren+1 : paren+close])
		}
		head = head[:paren]
	}
	return strings.TrimSpace(head), scope
}

// isDeveloperOnlyCommitType は利用者に関係しない変更の型かを返す。
func isDeveloperOnlyCommitType(t string) bool {
	switch t {
	case "ci", "chore", "ops", "build", "refactor", "test", "style", "perf":
		// perf は内部の速度改善が多く、利用者が体感できるものは
		// LLM が summary を書ける feat/fix として出されることが多い。
		return true
	}
	return false
}

// isDeveloperOnlyScope は利用者に関係しない領域のスコープかを返す。
func isDeveloperOnlyScope(scope string) bool {
	switch scope {
	case "ops", "ci", "infra", "deps", "deploy", "sre", "build":
		return true
	}
	return false
}

func containsDeveloperOnlyNeedle(lowerText string) bool {
	for _, needle := range developerOnlyReleaseNoteNeedles {
		if strings.Contains(lowerText, needle) {
			return true
		}
	}
	return false
}

func (s *ReleaseNoteService) deleteByPRNumber(ctx context.Context, prNumber uint) error {
	return s.db.WithContext(ctx).Where("pr_number = ?", prNumber).Delete(&models.ReleaseNote{}).Error
}

func (s *ReleaseNoteService) purgeStoredDeveloperOnlyNotes(ctx context.Context) error {
	var notes []models.ReleaseNote
	if err := s.db.WithContext(ctx).Find(&notes).Error; err != nil {
		return fmt.Errorf("list release notes for purge: %w", err)
	}
	for _, note := range notes {
		reason := ""
		switch {
		case isDeveloperOnlyReleaseNote(note.Title, note.Summary):
			reason = "開発者向け"
		default:
			// 判定を厳しくする前に保存された、専門用語まじりの更新情報も消す。
			// 基準を変えたときに古いものが残ると、いつまでも表示され続ける。
			if needle, found := containsJargon(note.Title, note.Summary); found {
				reason = "専門用語 " + needle
			}
		}
		if reason == "" {
			continue
		}
		if err := s.db.WithContext(ctx).Delete(&note).Error; err != nil {
			return fmt.Errorf("purge release note pr=%d: %w", note.PRNumber, err)
		}
		log.Printf("[ReleaseNote] PR #%d を削除しました(%s): %s", note.PRNumber, reason, note.Title)
	}
	return nil
}

// ReleaseNoteSource は取り込み対象のマージ済みPR情報。
type ReleaseNoteSource struct {
	PRNumber uint
	Title    string
	Body     string
	MergedAt time.Time
}

type releaseNoteSummaryResult struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Audience string `json:"audience"`
}

type ReleaseNoteService struct {
	db  *gorm.DB
	llm *openai.Client
}

func NewReleaseNoteService(db *gorm.DB, llm *openai.Client) *ReleaseNoteService {
	return &ReleaseNoteService{db: db, llm: llm}
}

// IngestMergedPRs は未取り込みのPRのみAIで要約しDBへ保存する（pr_numberでべき等）。
// 開発者向け（インフラ/CI等）は新規保存せず、既に載っている行があれば削除する（#969）。
func (s *ReleaseNoteService) IngestMergedPRs(ctx context.Context, sources []ReleaseNoteSource) (int, error) {
	if err := s.purgeStoredDeveloperOnlyNotes(ctx); err != nil {
		return 0, err
	}
	if s.llm == nil {
		return 0, ErrReleaseNoteLLMClientNil
	}

	saved := 0
	for _, src := range sources {
		if isDeveloperOnlyReleaseNote(src.Title, src.Body) {
			if err := s.deleteByPRNumber(ctx, src.PRNumber); err != nil {
				return saved, fmt.Errorf("delete developer-only release note PR #%d: %w", src.PRNumber, err)
			}
			continue
		}

		var count int64
		if err := s.db.Model(&models.ReleaseNote{}).Where("pr_number = ?", src.PRNumber).Count(&count).Error; err != nil {
			return saved, fmt.Errorf("check existing release note: %w", err)
		}
		if count > 0 {
			continue
		}

		result, err := s.summarize(ctx, src)
		if err != nil {
			return saved, fmt.Errorf("summarize PR #%d: %w", src.PRNumber, err)
		}
		if strings.TrimSpace(result.Summary) == "" {
			continue
		}

		// 専門用語が残っていたら出さない。指示しても従わないことがあるため、
		// 出力側でも検査する。専門用語まじりの文章を出すくらいなら、
		// その回の更新情報を出さないほうがよい(#1290)。
		if needle, found := containsJargon(result.Title, result.Summary); found {
			log.Printf("[ReleaseNote] PR #%d: 専門用語 %q が残っているため出力しない: %s",
				src.PRNumber, needle, result.Title)
			continue
		}

		audience := strings.TrimSpace(result.Audience)
		if !validReleaseNoteAudiences[audience] {
			audience = models.ReleaseNoteAudienceAll
		}

		note := models.ReleaseNote{
			PRNumber: src.PRNumber,
			Title:    result.Title,
			Summary:  result.Summary,
			Audience: audience,
			MergedAt: src.MergedAt,
		}
		if err := s.db.Create(&note).Error; err != nil {
			return saved, fmt.Errorf("save release note for PR #%d: %w", src.PRNumber, err)
		}
		saved++
	}
	return saved, nil
}

func (s *ReleaseNoteService) summarize(ctx context.Context, src ReleaseNoteSource) (*releaseNoteSummaryResult, error) {
	userPrompt := fmt.Sprintf(
		"以下のPR内容を次のJSON形式で要約してください。\n%s\n\n---\nPRタイトル: %s\nPR本文:\n%s",
		releaseNoteSummarySchema, src.Title, src.Body,
	)
	ctx = usagectx.WithFeature(ctx, usagectx.FeatureReleaseNote)
	raw, err := s.llm.ChatCompletionJSON(ctx, releaseNoteSummarySystemPrompt, userPrompt, 0.3, 300)
	if err != nil {
		return nil, err
	}
	var result releaseNoteSummaryResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse summary JSON: %w", err)
	}
	return &result, nil
}

// audienceForRole は閲覧者のrole/isAdminから、表示してよいaudienceの集合を返す。
// システム管理者向け（admin）、教員向け（teacher）、学生向け（student、デフォルト）を
// 排他的に判定し、いずれの場合も全員向け（all）は必ず含める。
func audienceForRole(role string, isAdmin bool) []string {
	if isAdmin {
		return []string{models.ReleaseNoteAudienceAll, "admin"}
	}
	if role == "teacher" {
		return []string{models.ReleaseNoteAudienceAll, "teacher"}
	}
	return []string{models.ReleaseNoteAudienceAll, "student"}
}

// List は閲覧者のrole/isAdminに応じた更新情報を新しい順に返す。
func (s *ReleaseNoteService) List(ctx context.Context, limit int, role string, isAdmin bool) ([]models.ReleaseNote, error) {
	if limit <= 0 {
		limit = 20
	}
	var notes []models.ReleaseNote
	audiences := audienceForRole(role, isAdmin)
	if err := s.db.WithContext(ctx).
		Where("audience IN ?", audiences).
		Order("merged_at DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, fmt.Errorf("list release notes: %w", err)
	}
	return notes, nil
}
