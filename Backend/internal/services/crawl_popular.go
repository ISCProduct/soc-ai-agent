package services

import (
	"Backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type popularCompanyExtraction struct {
	Companies []struct {
		Name     string `json:"name"`
		Evidence string `json:"evidence"`
		Summary  string `json:"summary"`
		Rank     *int   `json:"rank,omitempty"`
	} `json:"companies"`
}

func (s *CrawlService) executePopularCompaniesCrawl(source *models.CrawlSource) error {
	if s.aiClient == nil {
		return errors.New("openai client is required for popular_companies crawl")
	}
	body, err := fetchText(source.SourceURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("empty content from source_url")
	}

	extracted, err := s.extractPopularCompanies(source, body)
	if err != nil {
		return err
	}
	if len(extracted.Companies) == 0 {
		return errors.New("no companies extracted from source")
	}

	now := time.Now()
	for _, item := range extracted.Companies {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		company, err := s.companyRepo.FindByName(name)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			newCompany := &models.Company{
				Name:            name,
				SourceType:      source.SourceType,
				SourceURL:       source.SourceURL,
				SourceFetchedAt: &now,
				IsProvisional:   true,
				DataStatus:      "draft",
			}
			if err := s.companyRepo.Create(newCompany); err != nil {
				return err
			}
			company = newCompany
		}

		record := &models.CompanyPopularityRecord{
			CompanyID:    company.ID,
			SourceName:   source.Name,
			SourceURL:    source.SourceURL,
			EvidenceText: strings.TrimSpace(item.Evidence),
			Summary:      strings.TrimSpace(item.Summary),
			Rank:         item.Rank,
			FetchedAt:    now,
		}
		if err := s.popularRepo.Create(record); err != nil {
			return err
		}
	}
	return nil
}

func (s *CrawlService) extractPopularCompanies(source *models.CrawlSource, rawHTML string) (*popularCompanyExtraction, error) {
	clean := normalizeHTMLText(rawHTML)
	if len(clean) > 12000 {
		clean = clean[:12000]
	}
	systemPrompt := `You are a data extraction assistant. Use only the provided text. Do not infer or guess.`
	userPrompt := fmt.Sprintf(`Extract popular companies mentioned in the text below.
Return JSON with the following shape:
{
  "companies": [
    {
      "name": "Company Name",
      "evidence": "Exact excerpt from the text",
      "summary": "Why the company is described as popular, based only on the text",
      "rank": 1
    }
  ]
}
Rules:
- If rank is not shown, omit it or set it to null.
- evidence must be a verbatim excerpt from the text.
- summary must be a short, factual sentence based on the evidence only.

Text:
%s`, clean)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	content, err := s.aiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.2, 800)
	if err != nil {
		return nil, err
	}
	var parsed popularCompanyExtraction
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
