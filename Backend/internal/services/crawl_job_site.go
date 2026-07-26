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

type jobSiteCompanyExtraction struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Industry       string  `json:"industry"`
	EmployeeCount  int     `json:"employee_count"`
	FoundedYear    int     `json:"founded_year"`
	Location       string  `json:"location"`
	WebsiteURL     string  `json:"website_url"`
	Culture        string  `json:"culture"`
	WorkStyle      string  `json:"work_style"`
	WelfareDetails string  `json:"welfare_details"`
	MainBusiness   string  `json:"main_business"`
	AverageAge     float64 `json:"average_age"`
	FemaleRatio    float64 `json:"female_ratio"`
}

func (s *CrawlService) executeJobSiteCompanyCrawl(source *models.CrawlSource) error {
	if s.aiClient == nil {
		return errors.New("openai client is required for job_site_company crawl")
	}
	body, err := fetchText(source.SourceURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("empty content from source_url")
	}

	extracted, err := s.extractJobSiteCompany(source, body)
	if err != nil {
		return err
	}
	if strings.TrimSpace(extracted.Name) == "" {
		return errors.New("could not extract company name from source")
	}

	now := time.Now()
	company, err := s.companyRepo.FindByName(extracted.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newCompany := &models.Company{
			Name:            extracted.Name,
			Description:     extracted.Description,
			Industry:        extracted.Industry,
			EmployeeCount:   extracted.EmployeeCount,
			FoundedYear:     extracted.FoundedYear,
			Location:        extracted.Location,
			WebsiteURL:      extracted.WebsiteURL,
			Culture:         extracted.Culture,
			WorkStyle:       extracted.WorkStyle,
			WelfareDetails:  extracted.WelfareDetails,
			MainBusiness:    extracted.MainBusiness,
			AverageAge:      extracted.AverageAge,
			FemaleRatio:     extracted.FemaleRatio,
			SourceType:      source.SourceType,
			SourceURL:       source.SourceURL,
			SourceFetchedAt: &now,
			IsProvisional:   true,
			DataStatus:      "draft",
		}
		return s.companyRepo.Create(newCompany)
	}

	// 既存企業を更新（空でないフィールドのみ上書き）
	if extracted.Description != "" {
		company.Description = extracted.Description
	}
	if extracted.Industry != "" {
		company.Industry = extracted.Industry
	}
	if extracted.EmployeeCount > 0 {
		company.EmployeeCount = extracted.EmployeeCount
	}
	if extracted.FoundedYear > 0 {
		company.FoundedYear = extracted.FoundedYear
	}
	if extracted.Location != "" {
		company.Location = extracted.Location
	}
	if extracted.WebsiteURL != "" {
		company.WebsiteURL = extracted.WebsiteURL
	}
	if extracted.Culture != "" {
		company.Culture = extracted.Culture
	}
	if extracted.WorkStyle != "" {
		company.WorkStyle = extracted.WorkStyle
	}
	if extracted.WelfareDetails != "" {
		company.WelfareDetails = extracted.WelfareDetails
	}
	if extracted.MainBusiness != "" {
		company.MainBusiness = extracted.MainBusiness
	}
	if extracted.AverageAge > 0 {
		company.AverageAge = extracted.AverageAge
	}
	if extracted.FemaleRatio > 0 {
		company.FemaleRatio = extracted.FemaleRatio
	}
	company.SourceType = source.SourceType
	company.SourceURL = source.SourceURL
	company.SourceFetchedAt = &now
	return s.companyRepo.Update(company)
}

func (s *CrawlService) extractJobSiteCompany(source *models.CrawlSource, rawHTML string) (*jobSiteCompanyExtraction, error) {
	clean := normalizeHTMLText(rawHTML)
	if len(clean) > 12000 {
		clean = clean[:12000]
	}
	systemPrompt := `You are a data extraction assistant. Extract company information from new graduate job site pages. Use only the provided text. Do not infer or guess values not present in the text.`
	userPrompt := fmt.Sprintf(`Extract company information from the job site page text below.
Return JSON with the following shape:
{
  "name": "会社名",
  "description": "会社概要",
  "industry": "業界・業種",
  "employee_count": 1000,
  "founded_year": 2000,
  "location": "本社所在地",
  "website_url": "https://...",
  "culture": "企業文化・社風",
  "work_style": "リモート/ハイブリッド/オフィス",
  "welfare_details": "福利厚生",
  "main_business": "主要事業内容",
  "average_age": 32.5,
  "female_ratio": 40.0
}
Rules:
- Return 0 for numeric fields not found in the text.
- Return "" for string fields not found in the text.
- employee_count must be an integer.
- average_age and female_ratio must be floating-point numbers.
- Do not fabricate data.

Text:
%s`, clean)

	content, err := s.aiClient.ChatCompletionJSON(context.Background(), systemPrompt, userPrompt, 0.2, 800)
	if err != nil {
		return nil, err
	}
	var parsed jobSiteCompanyExtraction
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
