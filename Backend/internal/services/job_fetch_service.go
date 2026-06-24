package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/scraper"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JobFetchService は企業の求人情報と求める人物像を取得・保存するサービス。
type JobFetchService struct {
	repo         repository.CompanyRepository
	openaiClient *openai.Client
	careers      *scraper.CareersScraper
}

// NewJobFetchService は JobFetchService を生成する。
func NewJobFetchService(repo repository.CompanyRepository, client *openai.Client) *JobFetchService {
	return &JobFetchService{
		repo:         repo,
		openaiClient: client,
		careers:      scraper.NewCareersScraper(client),
	}
}

// FetchAndSaveJobs は企業の採用ページとWantedlyから求人情報を取得してDBに保存する。
// 既存レコードはタイトル一致で更新、新規は追加する。
func (s *JobFetchService) FetchAndSaveJobs(ctx context.Context, companyID uint) ([]models.CompanyJobPosition, error) {
	company, err := s.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("会社が見つかりません: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var allJobs []scraper.JobPostingResult

	// 方針1: 企業公式採用ページのクロール
	careersJobs, err := s.careers.FetchFromCareersPage(reqCtx, company.Name, company.WebsiteURL)
	if err == nil {
		allJobs = append(allJobs, careersJobs...)
	}

	// 方針2: Wantedlyから求人情報を取得
	wantedlyJobs, err := s.careers.FetchFromWantedly(reqCtx, company.Name)
	if err == nil {
		allJobs = append(allJobs, wantedlyJobs...)
	}

	if len(allJobs) == 0 {
		return nil, fmt.Errorf("求人情報を取得できませんでした")
	}

	saved := make([]models.CompanyJobPosition, 0, len(allJobs))
	for _, job := range allJobs {
		position, err := s.upsertJobPosition(companyID, job)
		if err != nil {
			continue
		}
		saved = append(saved, *position)
	}
	return saved, nil
}

// FetchAndSavePersona は企業の求める人物像をAIで分析し、CompanyWeightProfileに保存する。
func (s *JobFetchService) FetchAndSavePersona(ctx context.Context, companyID uint) (*models.CompanyWeightProfile, error) {
	company, err := s.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("会社が見つかりません: %w", err)
	}

	// 既存の求人情報も参考にする
	positions, _ := s.repo.FindJobPositionsByCompany(companyID)
	positionText := buildPositionSummary(positions)

	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	query := fmt.Sprintf(`「%s」が求める人物像・社風・採用基準を調査してください。`, company.Name)
	searchResult, err := s.openaiClient.WebSearchQuery(reqCtx, query)
	if err != nil {
		return nil, fmt.Errorf("人物像検索失敗: %w", err)
	}

	profile, err := s.analyzePersonaProfile(reqCtx, company.Name, searchResult, positionText)
	if err != nil {
		return nil, err
	}

	profile.CompanyID = companyID
	if err := s.repo.CreateOrUpdateWeightProfile(profile); err != nil {
		return nil, fmt.Errorf("プロファイル保存失敗: %w", err)
	}
	return profile, nil
}

func (s *JobFetchService) upsertJobPosition(companyID uint, job scraper.JobPostingResult) (*models.CompanyJobPosition, error) {
	requiredJSON, _ := json.Marshal(job.RequiredSkills)
	preferredJSON, _ := json.Marshal(job.PreferredSkills)

	existing, err := s.repo.FindJobPositionByCompanyAndTitle(companyID, job.Title)
	if err == nil && existing != nil {
		existing.Description = job.Description
		existing.EmploymentType = job.EmploymentType
		existing.WorkLocation = job.WorkLocation
		existing.RemoteOption = job.RemoteOption
		existing.MinSalary = job.MinSalary
		existing.MaxSalary = job.MaxSalary
		existing.RequiredSkills = string(requiredJSON)
		existing.PreferredSkills = string(preferredJSON)
		existing.IsActive = true
		if err := s.repo.UpdateJobPosition(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	position := &models.CompanyJobPosition{
		CompanyID:       companyID,
		Title:           job.Title,
		Description:     job.Description,
		EmploymentType:  job.EmploymentType,
		WorkLocation:    job.WorkLocation,
		RemoteOption:    job.RemoteOption,
		MinSalary:       job.MinSalary,
		MaxSalary:       job.MaxSalary,
		RequiredSkills:  string(requiredJSON),
		PreferredSkills: string(preferredJSON),
		IsActive:        true,
		DataStatus:      "draft",
	}
	if err := s.repo.CreateJobPosition(position); err != nil {
		return nil, err
	}
	return position, nil
}

// analyzePersonaProfile はWebSearch結果と求人情報テキストから10カテゴリのスコアを導出する。
func (s *JobFetchService) analyzePersonaProfile(ctx context.Context, companyName, searchResult, positionText string) (*models.CompanyWeightProfile, error) {
	systemPrompt := `あなたは採用コンサルタントです。企業情報から「企業が重視する人物像」を10カテゴリのスコア（0〜100）で評価し、指定のJSON形式のみで回答してください。`
	userPrompt := fmt.Sprintf(`「%s」が求める人物像を以下の10カテゴリでスコア化してください（0〜100、50が中立）。
JSON形式のみで回答してください（説明文は不要）。

{
  "technical_orientation": 技術志向の重視度（0-100）,
  "teamwork_orientation": チームワーク志向の重視度（0-100）,
  "leadership_orientation": リーダーシップ志向の重視度（0-100）,
  "creativity_orientation": 創造性志向の重視度（0-100）,
  "stability_orientation": 安定志向の重視度（0-100）,
  "growth_orientation": 成長志向の重視度（0-100）,
  "work_life_balance": ワークライフバランスの重視度（0-100）,
  "challenge_seeking": チャレンジ精神の重視度（0-100）,
  "detail_orientation": 細部への注意力の重視度（0-100）,
  "communication_skill": コミュニケーション力の重視度（0-100）
}

企業情報:
%s

求人情報:
%s`, companyName, searchResult, positionText)

	jsonStr, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.3, 500)
	if err != nil {
		return nil, fmt.Errorf("人物像分析失敗: %w", err)
	}

	start := strings.Index(jsonStr, "{")
	end := strings.LastIndex(jsonStr, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("人物像JSONパース失敗")
	}

	var raw struct {
		TechnicalOrientation  int `json:"technical_orientation"`
		TeamworkOrientation   int `json:"teamwork_orientation"`
		LeadershipOrientation int `json:"leadership_orientation"`
		CreativityOrientation int `json:"creativity_orientation"`
		StabilityOrientation  int `json:"stability_orientation"`
		GrowthOrientation     int `json:"growth_orientation"`
		WorkLifeBalance       int `json:"work_life_balance"`
		ChallengeSeeking      int `json:"challenge_seeking"`
		DetailOrientation     int `json:"detail_orientation"`
		CommunicationSkill    int `json:"communication_skill"`
	}
	if err := json.Unmarshal([]byte(jsonStr[start:end+1]), &raw); err != nil {
		return nil, fmt.Errorf("人物像JSONのunmarshal失敗: %w", err)
	}

	return &models.CompanyWeightProfile{
		TechnicalOrientation:  clampScore(raw.TechnicalOrientation),
		TeamworkOrientation:   clampScore(raw.TeamworkOrientation),
		LeadershipOrientation: clampScore(raw.LeadershipOrientation),
		CreativityOrientation: clampScore(raw.CreativityOrientation),
		StabilityOrientation:  clampScore(raw.StabilityOrientation),
		GrowthOrientation:     clampScore(raw.GrowthOrientation),
		WorkLifeBalance:       clampScore(raw.WorkLifeBalance),
		ChallengeSeeking:      clampScore(raw.ChallengeSeeking),
		DetailOrientation:     clampScore(raw.DetailOrientation),
		CommunicationSkill:    clampScore(raw.CommunicationSkill),
	}, nil
}

func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func buildPositionSummary(positions []models.CompanyJobPosition) string {
	if len(positions) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, p := range positions {
		fmt.Fprintf(&sb, "職種: %s / 雇用形態: %s / 必須: %s\n", p.Title, p.EmploymentType, p.RequiredSkills)
	}
	return sb.String()
}
