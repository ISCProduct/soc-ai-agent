package services

import (
	"Backend/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *CrawlService) runSource(source *models.CrawlSource) (*models.CrawlRun, error) {
	if source == nil {
		return nil, errors.New("source not found")
	}
	run := &models.CrawlRun{
		SourceID:  source.ID,
		Status:    "running",
		Message:   "",
		StartedAt: time.Now(),
	}
	if err := s.repo.CreateRun(run); err != nil {
		return nil, err
	}

	message := ""
	if err := s.executeCrawl(source); err != nil {
		message = err.Error()
		run.Status = "failed"
	} else {
		run.Status = "success"
		message = "completed"
	}
	run.Message = message
	finished := time.Now()
	run.EndedAt = &finished
	if err := s.repo.UpdateRun(run); err != nil {
		log.Printf("[Crawl] UpdateRun failed source=%d run=%d: %v", source.ID, run.ID, err)
	}

	source.LastRunAt = &finished
	source.NextRunAt = computeNextRun(finished, source)
	if err := s.repo.UpdateSource(source); err != nil {
		log.Printf("[Crawl] UpdateSource failed source=%d: %v", source.ID, err)
	}

	return run, nil
}

func (s *CrawlService) executeCrawl(source *models.CrawlSource) error {
	switch source.TargetType {
	case "company":
		return s.executeCompanyCrawl(source)
	case "popular_companies":
		return s.executePopularCompaniesCrawl(source)
	case "job_site_company":
		return s.executeJobSiteCompanyCrawl(source)
	case "job_listing":
		return s.executeJobListingCrawl(source)
	case "mynavi_company":
		return s.executeMynaviCompanyCrawl(source)
	case "openwork_company":
		return s.executeOpenworkCompanyCrawl(source)
	case "fetch_info_all":
		return s.executeFetchInfoAll()
	case "fetch_jobs_all":
		return s.executeFetchJobsAll()
	case "fetch_persona_all":
		return s.executeFetchPersonaAll()
	case "warm_l1_catalog":
		return s.executeWarmL1Catalog()
	default:
		return fmt.Errorf("unsupported target_type: %s", source.TargetType)
	}
}

func (s *CrawlService) executeCompanyCrawl(source *models.CrawlSource) error {
	if strings.TrimSpace(source.Name) == "" {
		return errors.New("company name is required for company crawl")
	}
	now := time.Now()
	company, err := s.companyRepo.FindByName(source.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newCompany := &models.Company{
			Name:            source.Name,
			SourceType:      source.SourceType,
			SourceURL:       source.SourceURL,
			SourceFetchedAt: &now,
			IsProvisional:   true,
			DataStatus:      "draft",
		}
		return s.companyRepo.Create(newCompany)
	}
	company.SourceType = source.SourceType
	company.SourceURL = source.SourceURL
	company.SourceFetchedAt = &now
	return s.companyRepo.Update(company)
}

// executeFetchInfoAll は登録済み全企業の基本情報を未取得のもののみ自動取得する。
func (s *CrawlService) executeFetchInfoAll() error {
	if s.infoFetcher == nil {
		return fmt.Errorf("info fetcher not configured")
	}
	return s.forEachActiveCompanyPage(200, func(company models.Company) error {
		_, err := s.infoFetcher.FetchAndSave(context.Background(), company.ID, false)
		return err
	}, "fetch_info_all")
}

// executeFetchJobsAll は登録済み全企業の求人情報を未取得のもののみ自動取得する。
func (s *CrawlService) executeFetchJobsAll() error {
	if s.jobFetcher == nil {
		return fmt.Errorf("job fetcher not configured")
	}
	return s.forEachActiveCompanyPage(200, func(company models.Company) error {
		_, err := s.jobFetcher.FetchAndSaveJobs(context.Background(), company.ID, false)
		return err
	}, "fetch_jobs_all")
}

// executeFetchPersonaAll は登録済み全企業の人物像を未取得のもののみ自動分析する。
func (s *CrawlService) executeFetchPersonaAll() error {
	if s.jobFetcher == nil {
		return fmt.Errorf("job fetcher not configured")
	}
	return s.forEachActiveCompanyPage(200, func(company models.Company) error {
		_, err := s.jobFetcher.FetchAndSavePersona(context.Background(), company.ID, false)
		return err
	}, "fetch_persona_all")
}

// executeWarmL1Catalog は公開マッチングカタログの L1（info+persona）を日次上限で温存する。
func (s *CrawlService) executeWarmL1Catalog() error {
	if s.infoFetcher == nil || s.jobFetcher == nil {
		return fmt.Errorf("info/job fetcher not configured")
	}
	warm := NewCatalogWarmService(s.companyRepo, s.infoFetcher, s.jobFetcher)
	result, err := warm.WarmL1(context.Background(), L1WarmOptions{
		IncludeInfo:    true,
		IncludePersona: true,
	})
	if err != nil {
		return err
	}
	log.Printf(
		"warm_l1_catalog: processed=%d info_ok=%d persona_ok=%d errors=%d needs=%d",
		result.Processed, result.InfoOK, result.PersonaOK, result.Errors,
		func() int64 {
			if result.Coverage == nil {
				return 0
			}
			return result.Coverage.NeedsWarm
		}(),
	)
	return nil
}

func (s *CrawlService) forEachActiveCompanyPage(pageSize int, fn func(models.Company) error, label string) error {
	if pageSize <= 0 {
		pageSize = 200
	}
	var errs []string
	processed := 0
	for offset := 0; ; offset += pageSize {
		companies, err := s.companyRepo.FindAllActive(pageSize, offset)
		if err != nil {
			return fmt.Errorf("企業一覧取得失敗: %w", err)
		}
		if len(companies) == 0 {
			break
		}
		for _, company := range companies {
			if err := fn(company); err != nil {
				errs = append(errs, fmt.Sprintf("id=%d: %v", company.ID, err))
			}
			processed++
		}
		if len(companies) < pageSize {
			break
		}
	}
	if len(errs) > 0 {
		log.Printf("%s: %d errors: %s", label, len(errs), strings.Join(errs, "; "))
	}
	log.Printf("%s: processed %d companies, errors=%d", label, processed, len(errs))
	// 全件失敗は success 扱いにしない（部分失敗は継続しつつログ済み）
	if processed > 0 && len(errs) == processed {
		return fmt.Errorf("%s: 全 %d 件の処理に失敗しました", label, processed)
	}
	return nil
}
