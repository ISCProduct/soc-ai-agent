package chat

import (
	"Backend/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *ChatService) completeJobAnalysisPhase(userID uint, sessionID string) error {
	if s.phaseRepo == nil || s.progressRepo == nil {
		return nil
	}
	phase, err := s.phaseRepo.FindByName("job_analysis")
	if err != nil || phase == nil {
		return nil
	}
	progress, err := s.progressRepo.FindOrCreate(userID, sessionID, phase.ID)
	if err != nil {
		return err
	}
	required := phase.MaxQuestions
	if required <= 0 {
		required = phase.MinQuestions
	}
	if required <= 0 || progress.IsCompleted {
		return nil
	}
	if progress.ValidAnswers < required {
		progress.ValidAnswers = required
	}
	if progress.QuestionsAsked < required {
		progress.QuestionsAsked = required
	}
	progress.CompletionScore = 100
	progress.IsCompleted = true
	now := time.Now()
	progress.CompletedAt = &now
	if progress.Phase == nil {
		progress.Phase = phase
	}
	return s.progressRepo.Update(progress)
}

func (s *ChatService) getJobCategoryCode(jobCategoryID uint) string {
	if jobCategoryID == 0 {
		return ""
	}
	category, err := s.jobCategoryRepo.FindByID(jobCategoryID)
	if err != nil || category == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(category.Code))
}

func (s *ChatService) getUserTargetLevel(userID uint) string {
	user, err := s.userRepo.GetUserByID(userID)
	if err == nil && user != nil && strings.TrimSpace(user.TargetLevel) == "中途" {
		return "中途"
	}
	return "新卒"
}

func (s *ChatService) getJobCategoryName(jobCategoryID uint) string {
	if jobCategoryID == 0 {
		return "未指定"
	}
	category, err := s.jobCategoryRepo.FindByID(jobCategoryID)
	if err != nil || category == nil {
		return "未指定"
	}
	return strings.TrimSpace(category.Name)
}

func (s *ChatService) getJobFitKeywords(jobCategoryID uint) ([]string, []string) {
	code := s.getJobCategoryCode(jobCategoryID)
	switch {
	case strings.HasPrefix(code, "ENG"):
		return []string{"プログラミング", "開発", "コード", "設計", "デバッグ"},
			[]string{"アルゴリズム", "API", "テスト", "Git", "サーバー", "データベース"}
	case strings.HasPrefix(code, "SALES"):
		return []string{"提案", "顧客", "ヒアリング", "関係構築", "課題"},
			[]string{"ニーズ", "交渉", "フォロー", "目標", "商談"}
	case strings.HasPrefix(code, "MKT"):
		return []string{"分析", "企画", "データ", "広告", "改善"},
			[]string{"SNS", "市場", "ターゲット", "施策", "検証"}
	case strings.HasPrefix(code, "HR"):
		return []string{"採用", "面接", "人材", "育成", "評価"},
			[]string{"研修", "面談", "制度", "組織", "コミュニケーション"}
	case strings.HasPrefix(code, "FIN"):
		return []string{"会計", "財務", "数値", "予算", "分析"},
			[]string{"収支", "コスト", "利益", "報告", "精算"}
	case strings.HasPrefix(code, "CONS"):
		return []string{"課題", "分析", "提案", "改善", "戦略"},
			[]string{"ヒアリング", "資料", "仮説", "整理", "意思決定"}
	default:
		return []string{}, []string{}
	}
}

func (s *ChatService) evaluateJobFitScoreWithAI(ctx context.Context, jobCategoryID uint, question, answer string, isChoice bool) (*jobFitEvaluation, error) {
	jobName := s.getJobCategoryName(jobCategoryID)
	jobCode := s.getJobCategoryCode(jobCategoryID)
	coreKeywords, relatedKeywords := s.getJobFitKeywords(jobCategoryID)

	questionType := "文章"
	if isChoice {
		questionType = "選択肢"
	}

	prompt := fmt.Sprintf(`あなたは就職適性診断の採点者です。以下のルールに従って採点してください。

## 職種
%s (%s)

## 質問（%s）
%s

## 回答
%s

## 職種理解キーワード
- 必須キーワード: %s
- 関連キーワード: %s

## 採点ルール
### 選択肢問題
- 回答が職種に最も適している場合: 90〜100点
- 適しても不適切でもない場合: 40〜70点
- 全く適していない場合: 0〜20点

### 文章問題
- 必須キーワードがすべて含まれる場合: 90〜100点
- 1語以上含まれる場合: 含まれた語数に応じて加点（1語=10点、最大80点）
- 1語も含まれない場合: 0点

## 出力形式（JSONのみ）
{"score": 0, "reason": "理由", "matched_keywords": ["キーワード"]}`,
		jobName,
		jobCode,
		questionType,
		question,
		answer,
		strings.Join(coreKeywords, ", "),
		strings.Join(relatedKeywords, ", "),
	)

	response, err := s.aiCallWithRetries(ctx, prompt)
	if err != nil {
		return nil, err
	}

	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("invalid JSON response for job fit evaluation")
	}

	var evaluation jobFitEvaluation
	if err := json.Unmarshal([]byte(response[jsonStart:jsonEnd+1]), &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse job fit evaluation: %w", err)
	}

	return &evaluation, nil
}

func (s *ChatService) getLastAssistantMessage(history []models.ChatMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			return history[i].Content
		}
	}
	return ""
}

type jobFitEvaluation struct {
	Score           int      `json:"score"`
	Reason          string   `json:"reason"`
	MatchedKeywords []string `json:"matched_keywords"`
}
