package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// JobCategoryValidator 職種判定サービス
type JobCategoryValidator struct {
	aiClient        *openai.Client
	jobCategoryRepo repository.JobCategoryRepository
}

func NewJobCategoryValidator(aiClient *openai.Client, jobCategoryRepo repository.JobCategoryRepository) *JobCategoryValidator {
	return &JobCategoryValidator{
		aiClient:        aiClient,
		jobCategoryRepo: jobCategoryRepo,
	}
}

// JobValidationResult 職種判定結果
type JobValidationResult struct {
	IsValid            bool                 `json:"is_valid"`
	MatchedCategories  []models.JobCategory `json:"matched_categories"`
	SuggestedQuestion  string               `json:"suggested_question"`
	NeedsClarification bool                 `json:"needs_clarification"`
}

// ValidateJobCategory ユーザーの職種回答を判定
func (v *JobCategoryValidator) ValidateJobCategory(ctx context.Context, userAnswer string) (*JobValidationResult, error) {
	normalizedAnswer, err := v.normalizeNumericAnswer(userAnswer)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize numeric answer: %w", err)
	}
	userAnswer = normalizedAnswer

	// 1. すべての職種を取得
	allCategories, err := v.jobCategoryRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get job categories: %w", err)
	}

	// 2. 職種リストをJSON形式に変換
	categoryList := make([]map[string]interface{}, 0)
	for _, cat := range allCategories {
		categoryList = append(categoryList, map[string]interface{}{
			"id":   cat.ID,
			"name": cat.Name,
			"code": cat.Code,
		})
	}
	categoryJSON, _ := json.Marshal(categoryList)

	// 3. AIで判定
	prompt := fmt.Sprintf(`あなたは職種判定の専門家です。
ユーザーが回答した職種が、以下の職種リストのどれに該当するかを判定してください。

## ユーザーの回答
「%s」

## 利用可能な職種リスト
%s

## 判定ルール

### 1. 明確に該当する場合
ユーザーの回答が職種リストのいずれかに明確に該当する場合：
- is_valid: true
- matched_ids: [該当する職種のID（最大3つまで）]
- needs_clarification: false

例:
回答「エンジニア」→ エンジニア関連の職種にマッチ
回答「営業」→ 営業関連の職種にマッチ

### 2. 曖昧または複数の可能性がある場合
ユーザーの回答が複数の職種に該当しうる、または曖昧な場合：
- is_valid: false
- matched_ids: [候補となる職種のID（3〜5個）]
- needs_clarification: true
- suggested_question: 選択肢を提示する質問文

例:
回答「IT系」→ エンジニア、Webエンジニア、データエンジニアなど複数候補
回答「営業系」→ 法人営業、個人営業など

### 3. 該当しない・わからない場合
ユーザーの回答が職種リストにない、または「わからない」等の場合：
- is_valid: false
- matched_ids: []
- needs_clarification: true
- suggested_question: 最適な職種を分析するための質問文（好みや得意なことを聞く）

例:
回答「まだ決めていない」→ どんな活動が好きかを聞く
回答「わからない」→ 具体的な選択肢で興味の方向性を聞く

## 出力形式（JSON）
{
  "is_valid": true/false,
  "matched_ids": [1, 2, 3],
  "needs_clarification": true/false,
  "suggested_question": "選択肢を提示する質問文"
}

## 質問文の作成ガイドライン
- 新卒学生向けの優しい表現
- 3〜5個の選択肢を提示
- 「どれに興味がありますか？」「どれが近いですか？」など
- 選択肢は具体的に列挙
- 職種未決定の場合は「好きな作業のタイプ」を聞く

例:
「以下のうち、どの職種に興味がありますか？
1. エンジニア（プログラミング、開発）
2. 営業（顧客対応、提案）
3. マーケティング（企画、分析）
4. 人事（採用、育成）
5. まだ決めていない」

「まだ決めていない場合、どんな作業が好きですか？
A) 人と話したり調整する
B) 数字やデータを分析する
C) ものづくりや工夫を考える
D) 調べてまとめる」

JSONのみを返してください。説明は不要です。`, userAnswer, string(categoryJSON))

	responseText, err := v.aiClient.Responses(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI validation failed: %w", err)
	}

	// 4. AI応答をパース
	var aiResponse struct {
		IsValid            bool   `json:"is_valid"`
		MatchedIDs         []uint `json:"matched_ids"`
		NeedsClarification bool   `json:"needs_clarification"`
		SuggestedQuestion  string `json:"suggested_question"`
	}

	// JSONのみを抽出（```json ... ``` を除去）
	cleanJSON := strings.TrimSpace(responseText)
	if strings.HasPrefix(cleanJSON, "```json") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	} else if strings.HasPrefix(cleanJSON, "```") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	}

	if err := json.Unmarshal([]byte(cleanJSON), &aiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w, response: %s", err, responseText)
	}

	// 5. マッチした職種の詳細を取得
	matchedCategories := []models.JobCategory{}
	for _, id := range aiResponse.MatchedIDs {
		category, err := v.jobCategoryRepo.FindByID(id)
		if err == nil {
			matchedCategories = append(matchedCategories, *category)
		}
	}

	return &JobValidationResult{
		IsValid:            aiResponse.IsValid,
		MatchedCategories:  matchedCategories,
		SuggestedQuestion:  aiResponse.SuggestedQuestion,
		NeedsClarification: aiResponse.NeedsClarification,
	}, nil
}

func (v *JobCategoryValidator) normalizeNumericAnswer(userAnswer string) (string, error) {
	trimmed := strings.TrimSpace(userAnswer)
	if trimmed == "" {
		return userAnswer, nil
	}

	choice, err := strconv.Atoi(trimmed)
	if err != nil || choice <= 0 {
		return userAnswer, nil
	}

	topCategories, err := v.jobCategoryRepo.GetTopCategories()
	if err != nil {
		return "", err
	}

	if choice <= len(topCategories) {
		return topCategories[choice-1].Name, nil
	}
	if choice == len(topCategories)+1 {
		return "まだ決めていない", nil
	}
	return userAnswer, nil
}

// GenerateJobSelectionQuestion 職種選択の質問を生成
func (v *JobCategoryValidator) GenerateJobSelectionQuestion(ctx context.Context) (string, error) {
	// 主要な職種カテゴリを取得
	topCategories, err := v.jobCategoryRepo.GetTopCategories()
	if err != nil {
		return "", err
	}

	question := "どの職種に興味がありますか？以下から選んでください：\n\n"
	for i, cat := range topCategories {
		question += fmt.Sprintf("%d. %s\n", i+1, cat.Name)
	}
	question += fmt.Sprintf("%d. まだ決めていない\n", len(topCategories)+1)
	question += "\n番号で答えても、職種名で答えても構いません。"

	return question, nil
}
