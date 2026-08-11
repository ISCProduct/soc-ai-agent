package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/ragclient"
	"Backend/internal/scraper"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// JobFetchService は企業の求人情報と求める人物像を取得・保存するサービス。
type JobFetchService struct {
	repo         repository.CompanyRepository
	openaiClient *openai.Client
	careers      *scraper.CareersScraper
	flight       *CompanySearchFlight
}

// NewJobFetchService は JobFetchService を生成する。
func NewJobFetchService(repo repository.CompanyRepository, client *openai.Client) *JobFetchService {
	return &JobFetchService{
		repo:         repo,
		openaiClient: client,
		careers:      scraper.NewCareersScraper(client),
	}
}

// SetSearchBudget は月次 Search 予算ガードを注入する。
func (s *JobFetchService) SetSearchBudget(budget companyfetch.SearchBudget) {
	if s == nil {
		return
	}
	if s.careers != nil {
		s.careers.SetSearchBudget(budget)
	}
}

// SetSearchFlight は企業キー単位の singleflight を注入する。
func (s *JobFetchService) SetSearchFlight(flight *CompanySearchFlight) {
	if s != nil {
		s.flight = flight
	}
}

// FetchAndSaveJobs は Web Search→Parse から企業の求人情報を取得してDBに保存する。
// forceRefresh=false かつ JobsFetchedAt が TTL 内なら AI を呼ばずに返す。
func (s *JobFetchService) FetchAndSaveJobs(ctx context.Context, companyID uint, forceRefresh bool) ([]models.CompanyJobPosition, error) {
	company, err := s.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("会社が見つかりません: %w", err)
	}

	if !forceRefresh && companyfetch.IsFresh(company.JobsFetchedAt, companyfetch.TTLJobs) {
		existing, _ := s.repo.ListJobPositions(&companyID, nil, 100)
		if len(existing) > 0 {
			return existing, nil
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	run := func() (any, error) {
		return s.careers.FetchJobs(reqCtx, company.Name, company.WebsiteURL)
	}
	var allJobs []scraper.JobPostingResult
	if s.flight != nil {
		v, ferr := s.flight.Do("jobs", normalizeCompanyKey(company.Name), run)
		err = ferr
		if v != nil {
			allJobs, _ = v.([]scraper.JobPostingResult)
		}
	} else {
		allJobs, err = s.careers.FetchJobs(reqCtx, company.Name, company.WebsiteURL)
	}
	if err != nil {
		if errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
			existing, _ := s.repo.ListJobPositions(&companyID, nil, 100)
			return existing, nil
		}
		return nil, err
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

	now := time.Now()
	company.JobsFetchedAt = &now
	company.SourceFetchedAt = &now
	if len(allJobs) > 0 {
		company.SourceType = allJobs[0].Source
		company.LastFetchConfidence = confidenceForJobSource(allJobs[0].Source)
		company.LastModelUsed = companyfetch.ExtractModel()
		if allJobs[0].Source == companyfetch.SourceWebSearch {
			company.LastModelUsed = companyfetch.SearchModel() + "+" + companyfetch.ParseModel()
		}
		// web_search 時は旧スクレイプ URL を残さない
		if allJobs[0].Source == companyfetch.SourceWebSearch {
			company.SourceURL = strings.TrimSpace(company.WebsiteURL)
		}
	}
	if err := s.repo.Update(company); err != nil {
		return nil, fmt.Errorf("求人取得メタデータの保存に失敗しました: %w", err)
	}

	// RAGのChromaDBに求人情報を保存してレビュー精度を向上させる
	if len(saved) > 0 {
		ragContent := buildJobsRAGContent(company.Name, allJobs)
		go s.pushContextToRAG(company.Name, "jobs", ragContent)
	}

	return saved, nil
}

func confidenceForJobSource(source string) string {
	switch source {
	case companyfetch.SourceScrape, companyfetch.SourceGBiz:
		return companyfetch.ConfidenceHigh
	case companyfetch.SourceWebSearch:
		return companyfetch.ConfidenceMedium
	default:
		return companyfetch.ConfidenceLow
	}
}

// FetchAndSavePersona は企業の求める人物像をAIで分析し、CompanyWeightProfileに保存する。
// forceRefresh=false かつ DB に既存プロファイルがある場合は AI を呼ばずに返す。
func (s *JobFetchService) FetchAndSavePersona(ctx context.Context, companyID uint, forceRefresh bool) (*models.CompanyWeightProfile, error) {
	company, err := s.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("会社が見つかりません: %w", err)
	}

	if !forceRefresh {
		existing, err := s.repo.GetWeightProfile(companyID, nil)
		if err == nil && existing != nil {
			return existing, nil
		}
	}

	// 既存の求人情報も参考にする
	positions, _ := s.repo.FindJobPositionsByCompany(companyID)
	positionText := buildPositionSummary(positions)

	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// DB保存済みの企業情報を分析材料として使う（WebSearchは使わない）
	companyInfo := buildCompanyInfoText(company)

	profile, err := s.analyzePersonaProfile(reqCtx, company.Name, companyInfo, positionText)
	if err != nil {
		return nil, err
	}

	profile.CompanyID = companyID
	if err := s.repo.CreateOrUpdateWeightProfile(profile); err != nil {
		return nil, fmt.Errorf("プロファイル保存失敗: %w", err)
	}

	// RAGのChromaDBに人物像データを保存して履歴書・ESレビューの精度を向上させる
	ragContent := buildPersonaRAGContent(company.Name, companyInfo, profile)
	go s.pushContextToRAG(company.Name, "persona", ragContent)

	return profile, nil
}

// pushContextToRAG は取得した企業情報をRAGサービスのChromaDBに非同期でpushする。
func (s *JobFetchService) pushContextToRAG(companyName, contextType, content string) {
	ragURL := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	if ragURL == "" {
		return
	}
	payload, err := json.Marshal(map[string]string{
		"company_name": companyName,
		"context_type": contextType,
		"content":      content,
	})
	if err != nil {
		return
	}
	url := strings.TrimRight(ragURL, "/") + "/company/context"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	ragclient.SetAuthHeader(req)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("rag push failed company=%q type=%s error=%v", companyName, contextType, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("rag push ok company=%q type=%s status=%d", companyName, contextType, resp.StatusCode)
}

// buildJobsRAGContent は求人情報リストを人が読めるテキストに変換する。
func buildJobsRAGContent(companyName string, jobs []scraper.JobPostingResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "【%s】求人情報\n\n", companyName)
	for _, job := range jobs {
		fmt.Fprintf(&sb, "■ %s\n", job.Title)
		if job.URL != "" {
			fmt.Fprintf(&sb, "  URL: %s\n", job.URL)
		}
		if job.EmploymentType != "" {
			fmt.Fprintf(&sb, "  雇用形態: %s\n", job.EmploymentType)
		}
		if job.WorkLocation != "" {
			fmt.Fprintf(&sb, "  勤務地: %s\n", job.WorkLocation)
		}
		if job.RemoteOption {
			fmt.Fprintf(&sb, "  リモート: 可\n")
		}
		if job.MinSalary > 0 || job.MaxSalary > 0 {
			fmt.Fprintf(&sb, "  年収: %d〜%d万円\n", job.MinSalary, job.MaxSalary)
		}
		if len(job.RequiredSkills) > 0 {
			fmt.Fprintf(&sb, "  必須スキル: %s\n", strings.Join(job.RequiredSkills, "、"))
		}
		if job.Description != "" {
			fmt.Fprintf(&sb, "  説明: %s\n", job.Description)
		}
		if len(job.PersonaKeywords) > 0 {
			fmt.Fprintf(&sb, "  求める人物像: %s\n", strings.Join(job.PersonaKeywords, "、"))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildCompanyInfoText はDB保存済みの企業情報を人物像分析用テキストに整形する。
func buildCompanyInfoText(company *models.Company) string {
	var sb strings.Builder
	if company.Description != "" {
		fmt.Fprintf(&sb, "概要: %s\n", company.Description)
	}
	if company.Industry != "" {
		fmt.Fprintf(&sb, "業種: %s\n", company.Industry)
	}
	if company.MainBusiness != "" {
		fmt.Fprintf(&sb, "主要事業: %s\n", company.MainBusiness)
	}
	if company.Culture != "" {
		fmt.Fprintf(&sb, "企業文化: %s\n", company.Culture)
	}
	if company.WorkStyle != "" {
		fmt.Fprintf(&sb, "勤務スタイル: %s\n", company.WorkStyle)
	}
	return sb.String()
}

// buildPersonaRAGContent は人物像スコアと企業情報を人が読めるテキストに変換する。
func buildPersonaRAGContent(companyName, companyInfo string, profile *models.CompanyWeightProfile) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "【%s】求める人物像・採用基準\n\n", companyName)
	if companyInfo != "" {
		fmt.Fprintf(&sb, "■ 企業情報\n%s\n\n", companyInfo)
	}
	fmt.Fprintf(&sb, "■ 重視度スコア（0-100）\n")
	fmt.Fprintf(&sb, "  技術志向: %d\n", profile.TechnicalOrientation)
	fmt.Fprintf(&sb, "  チームワーク志向: %d\n", profile.TeamworkOrientation)
	fmt.Fprintf(&sb, "  リーダーシップ志向: %d\n", profile.LeadershipOrientation)
	fmt.Fprintf(&sb, "  創造性志向: %d\n", profile.CreativityOrientation)
	fmt.Fprintf(&sb, "  安定志向: %d\n", profile.StabilityOrientation)
	fmt.Fprintf(&sb, "  成長志向: %d\n", profile.GrowthOrientation)
	fmt.Fprintf(&sb, "  ワークライフバランス: %d\n", profile.WorkLifeBalance)
	fmt.Fprintf(&sb, "  チャレンジ精神: %d\n", profile.ChallengeSeeking)
	fmt.Fprintf(&sb, "  細部への注意力: %d\n", profile.DetailOrientation)
	fmt.Fprintf(&sb, "  コミュニケーション力: %d\n", profile.CommunicationSkill)
	return sb.String()
}

func (s *JobFetchService) upsertJobPosition(companyID uint, job scraper.JobPostingResult) (*models.CompanyJobPosition, error) {
	if strings.TrimSpace(job.Title) == "" {
		return nil, fmt.Errorf("求人タイトルが空です")
	}

	requiredJSON, _ := json.Marshal(job.RequiredSkills)
	preferredJSON, _ := json.Marshal(job.PreferredSkills)

	existing, err := s.repo.FindJobPositionByCompanyAndTitle(companyID, job.Title)
	if err == nil && existing != nil {
		existing.JobURL = job.URL
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
		JobURL:          job.URL,
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

// analyzePersonaProfile は企業情報と求人情報テキストから10カテゴリのスコアを導出する。
func (s *JobFetchService) analyzePersonaProfile(ctx context.Context, companyName, companyInfo, positionText string) (*models.CompanyWeightProfile, error) {
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
%s`, companyName, companyInfo, positionText)

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
