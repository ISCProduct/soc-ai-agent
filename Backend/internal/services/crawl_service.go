package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type CrawlService struct {
	repo        repository.CrawlRepository
	companyRepo repository.CompanyRepository
	popularRepo repository.CompanyPopularityRepository
	aiClient    *openai.Client
	jobFetcher  *JobFetchService
	infoFetcher *CompanyInfoFetcher
	mu          sync.Mutex
}

func NewCrawlService(repo repository.CrawlRepository, companyRepo repository.CompanyRepository, popularRepo repository.CompanyPopularityRepository, aiClient *openai.Client) *CrawlService {
	return &CrawlService{repo: repo, companyRepo: companyRepo, popularRepo: popularRepo, aiClient: aiClient}
}

func (s *CrawlService) SetJobFetcher(f *JobFetchService)     { s.jobFetcher = f }
func (s *CrawlService) SetInfoFetcher(f *CompanyInfoFetcher) { s.infoFetcher = f }

type CrawlSourcePayload struct {
	Name         string `json:"name"`
	TargetType   string `json:"target_type"`
	SourceType   string `json:"source_type"`
	SourceURL    string `json:"source_url"`
	ScheduleType string `json:"schedule_type"`
	ScheduleDay  *int   `json:"schedule_day"`
	ScheduleTime string `json:"schedule_time"`
	IsActive     *bool  `json:"is_active"`
}

func (s *CrawlService) ListSources() ([]models.CrawlSource, error) {
	return s.repo.ListSources()
}

func (s *CrawlService) ListRuns(sourceID uint, limit int) ([]models.CrawlRun, error) {
	return s.repo.ListRuns(sourceID, limit)
}

func (s *CrawlService) CreateSource(payload CrawlSourcePayload) (*models.CrawlSource, error) {
	if payload.ScheduleDay == nil {
		return nil, errors.New("schedule_day is required")
	}
	source := &models.CrawlSource{
		Name:         strings.TrimSpace(payload.Name),
		TargetType:   strings.TrimSpace(payload.TargetType),
		SourceType:   strings.TrimSpace(payload.SourceType),
		SourceURL:    strings.TrimSpace(payload.SourceURL),
		ScheduleType: strings.TrimSpace(payload.ScheduleType),
		ScheduleDay:  *payload.ScheduleDay,
		ScheduleTime: strings.TrimSpace(payload.ScheduleTime),
		IsActive:     true,
	}
	if payload.IsActive != nil {
		source.IsActive = *payload.IsActive
	}
	if err := validateCrawlSource(source); err != nil {
		return nil, err
	}
	next := computeNextRun(time.Now(), source)
	source.NextRunAt = next
	if err := s.repo.CreateSource(source); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *CrawlService) UpdateSource(id uint, payload CrawlSourcePayload) (*models.CrawlSource, error) {
	source, err := s.repo.GetSource(id)
	if err != nil {
		return nil, err
	}
	if payload.Name != "" {
		source.Name = strings.TrimSpace(payload.Name)
	}
	if payload.TargetType != "" {
		source.TargetType = strings.TrimSpace(payload.TargetType)
	}
	if payload.SourceType != "" {
		source.SourceType = strings.TrimSpace(payload.SourceType)
	}
	if payload.SourceURL != "" {
		source.SourceURL = strings.TrimSpace(payload.SourceURL)
	}
	if payload.ScheduleType != "" {
		source.ScheduleType = strings.TrimSpace(payload.ScheduleType)
	}
	if payload.ScheduleDay != nil {
		source.ScheduleDay = *payload.ScheduleDay
	}
	if payload.ScheduleTime != "" {
		source.ScheduleTime = strings.TrimSpace(payload.ScheduleTime)
	}
	if payload.IsActive != nil {
		source.IsActive = *payload.IsActive
	}
	if err := validateCrawlSource(source); err != nil {
		return nil, err
	}
	source.NextRunAt = computeNextRun(time.Now(), source)
	if err := s.repo.UpdateSource(source); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *CrawlService) RunSource(id uint) (*models.CrawlRun, error) {
	source, err := s.repo.GetSource(id)
	if err != nil {
		return nil, err
	}
	return s.runSource(source)
}

// EnsureL1WarmCrawlSource は日次 L1 温存ジョブが無ければ作成する（#586）。
func (s *CrawlService) EnsureL1WarmCrawlSource() {
	sources, err := s.repo.ListSources()
	if err != nil {
		log.Printf("ensure warm_l1_catalog: list failed: %v", err)
		return
	}
	for _, src := range sources {
		if src.TargetType == "warm_l1_catalog" {
			return
		}
	}
	timeOfDay := strings.TrimSpace(os.Getenv("L1_WARM_SCHEDULE_TIME"))
	if timeOfDay == "" {
		timeOfDay = "03:00"
	}
	source := &models.CrawlSource{
		Name:         "L1 catalog daily warm",
		TargetType:   "warm_l1_catalog",
		SourceType:   "manual",
		ScheduleType: "daily",
		ScheduleDay:  0,
		ScheduleTime: timeOfDay,
		IsActive:     true,
	}
	if err := validateCrawlSource(source); err != nil {
		log.Printf("ensure warm_l1_catalog: invalid: %v", err)
		return
	}
	now := time.Now()
	source.NextRunAt = computeNextRun(now, source)
	if err := s.repo.CreateSource(source); err != nil {
		log.Printf("ensure warm_l1_catalog: create failed: %v", err)
		return
	}
	log.Printf("ensure warm_l1_catalog: created id=%d next_run_at=%v", source.ID, source.NextRunAt)
}

func validateCrawlSource(source *models.CrawlSource) error {
	if strings.TrimSpace(source.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(source.TargetType) == "" {
		return errors.New("target_type is required")
	}
	urlRequiredTypes := map[string]bool{
		"popular_companies": true,
		"job_site_company":  true,
		"job_listing":       true,
		"mynavi_company":    true,
		"openwork_company":  true,
	}
	allTypes := []string{"company", "popular_companies", "job_site_company", "job_listing", "mynavi_company", "openwork_company", "fetch_info_all", "fetch_jobs_all", "fetch_persona_all", "warm_l1_catalog"}
	validType := false
	for _, t := range allTypes {
		if source.TargetType == t {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("target_type must be one of: %s", strings.Join(allTypes, ", "))
	}
	if urlRequiredTypes[source.TargetType] && strings.TrimSpace(source.SourceURL) == "" {
		return fmt.Errorf("source_url is required for %s", source.TargetType)
	}
	if source.ScheduleType != "daily" && source.ScheduleType != "weekly" && source.ScheduleType != "monthly" {
		return errors.New("schedule_type must be daily, weekly or monthly")
	}
	if source.ScheduleType == "weekly" {
		if source.ScheduleDay < 0 || source.ScheduleDay > 6 {
			return errors.New("schedule_day must be 0-6 for weekly")
		}
	}
	if source.ScheduleType == "monthly" {
		if source.ScheduleDay < 1 || source.ScheduleDay > 31 {
			return errors.New("schedule_day must be 1-31 for monthly")
		}
	}
	if source.ScheduleTime == "" || !isValidTime(source.ScheduleTime) {
		return errors.New("schedule_time must be HH:MM")
	}
	return nil
}

// Exported wrapper for testing from external packages.
func ValidateCrawlSource(source *models.CrawlSource) error { return validateCrawlSource(source) }
