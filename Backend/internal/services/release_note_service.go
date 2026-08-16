package services

import (
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrReleaseNoteLLMClientNil = errors.New("openai client is nil")

const releaseNoteSummarySystemPrompt = `あなたはtoC向けSaaSのリリースノート編集者です。開発者向けのPRタイトル・本文から、エンドユーザー（就活中の学生）向けにやさしい日本語で1〜2文の更新内容を要約してください。技術用語（API名・関数名・内部実装の詳細）は避け、ユーザーにとって何が変わったか・何が嬉しいかを書いてください。

以下は学生ユーザーの目に触れない・直接使わない変更のため、summary を空文字にしてください（開発部門・運用担当者向けの内容はユーザーに提供しない）:
- インフラ・運用・デプロイ関連（AWS/ECS/Terraform構成、起動停止設定、コスト最適化、サーバー移行など）
- 社内ツール連携・開発フロー（Notion/Backlog/Slack通知連携、CI/CD、コードレビュー体制など）
- 管理者専用機能（管理画面/adminのみで使う機能、学校・企業側の運用機能で学生が直接触れないもの）
- リファクタリング・依存関係更新・セキュリティパッチ（ユーザーの体験に見た目上の変化がないもの）
- 開発者向けドキュメント整備

summaryを書いてよいのは、学生ユーザーが実際に画面や機能として体験できる新機能・UI改善・不具合修正・パフォーマンス向上のみです。迷ったら空文字にしてください。

JSON形式のみで出力し、他のテキストは含めないでください。`

const releaseNoteSummarySchema = `{"title": "ユーザー向けの短いタイトル(20字以内)", "summary": "ユーザー向けの説明文(1〜2文)。ユーザーに関係ない変更なら空文字"}`

// ReleaseNoteSource は取り込み対象のマージ済みPR情報。
type ReleaseNoteSource struct {
	PRNumber uint
	Title    string
	Body     string
	MergedAt time.Time
}

type releaseNoteSummaryResult struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type ReleaseNoteService struct {
	db  *gorm.DB
	llm *openai.Client
}

func NewReleaseNoteService(db *gorm.DB, llm *openai.Client) *ReleaseNoteService {
	return &ReleaseNoteService{db: db, llm: llm}
}

// IngestMergedPRs は未取り込みのPRのみAIで要約しDBへ保存する（pr_numberでべき等）。
// 戻り値は新規に保存された件数。
func (s *ReleaseNoteService) IngestMergedPRs(ctx context.Context, sources []ReleaseNoteSource) (int, error) {
	if s.llm == nil {
		return 0, ErrReleaseNoteLLMClientNil
	}

	saved := 0
	for _, src := range sources {
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
			// ユーザーに関係ない変更として要約対象外
			continue
		}

		note := models.ReleaseNote{
			PRNumber: src.PRNumber,
			Title:    result.Title,
			Summary:  result.Summary,
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

// List は更新情報を新しい順に返す。
func (s *ReleaseNoteService) List(ctx context.Context, limit int) ([]models.ReleaseNote, error) {
	if limit <= 0 {
		limit = 20
	}
	var notes []models.ReleaseNote
	if err := s.db.WithContext(ctx).Order("merged_at DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, fmt.Errorf("list release notes: %w", err)
	}
	return notes, nil
}
