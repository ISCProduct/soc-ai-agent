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

type jobListingExtraction struct {
	CompanyName string `json:"company_name"`
	Positions   []struct {
		Title           string `json:"title"`
		Description     string `json:"description"`
		EmploymentType  string `json:"employment_type"`
		WorkLocation    string `json:"work_location"`
		RemoteOption    *bool  `json:"remote_option,omitempty"` // nil = 未検出（既存値を保持）
		MinSalary       int    `json:"min_salary"`
		MaxSalary       int    `json:"max_salary"`
		RequiredSkills  string `json:"required_skills"`
		PreferredSkills string `json:"preferred_skills"`
	} `json:"positions"`
}

func (s *CrawlService) executeJobListingCrawl(source *models.CrawlSource) error {
	if s.aiClient == nil {
		return errors.New("openai client is required for job_listing crawl")
	}
	body, err := fetchText(source.SourceURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("empty content from source_url")
	}

	extracted, err := s.extractJobListings(source, body)
	if err != nil {
		return err
	}
	if strings.TrimSpace(extracted.CompanyName) == "" {
		return errors.New("could not extract company name from source")
	}
	if len(extracted.Positions) == 0 {
		return errors.New("no job positions extracted from source")
	}

	now := time.Now()
	company, err := s.companyRepo.FindByName(extracted.CompanyName)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if company == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		newCompany := &models.Company{
			Name:            extracted.CompanyName,
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

	for _, p := range extracted.Positions {
		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}
		existing, err := s.companyRepo.FindJobPositionByCompanyAndTitle(company.ID, title)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			remote := false
			if p.RemoteOption != nil {
				remote = *p.RemoteOption
			}
			pos := &models.CompanyJobPosition{
				CompanyID:       company.ID,
				Title:           title,
				Description:     p.Description,
				EmploymentType:  p.EmploymentType,
				WorkLocation:    p.WorkLocation,
				RemoteOption:    remote,
				MinSalary:       p.MinSalary,
				MaxSalary:       p.MaxSalary,
				RequiredSkills:  p.RequiredSkills,
				PreferredSkills: p.PreferredSkills,
				IsActive:        true,
			}
			if err := s.companyRepo.CreateJobPosition(pos); err != nil {
				return err
			}
		} else {
			if p.Description != "" {
				existing.Description = p.Description
			}
			if p.EmploymentType != "" {
				existing.EmploymentType = p.EmploymentType
			}
			if p.WorkLocation != "" {
				existing.WorkLocation = p.WorkLocation
			}
			if p.RemoteOption != nil {
				existing.RemoteOption = *p.RemoteOption
			}
			if p.MinSalary > 0 {
				existing.MinSalary = p.MinSalary
			}
			if p.MaxSalary > 0 {
				existing.MaxSalary = p.MaxSalary
			}
			if p.RequiredSkills != "" {
				existing.RequiredSkills = p.RequiredSkills
			}
			if p.PreferredSkills != "" {
				existing.PreferredSkills = p.PreferredSkills
			}
			if err := s.companyRepo.UpdateJobPosition(existing); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *CrawlService) extractJobListings(source *models.CrawlSource, rawHTML string) (*jobListingExtraction, error) {
	clean := normalizeHTMLText(rawHTML)
	if len(clean) > 12000 {
		clean = clean[:12000]
	}
	systemPrompt := `You are a data extraction assistant. Extract job listing information from new graduate job site pages. Use only the provided text. Do not infer or guess values not present in the text.`
	userPrompt := fmt.Sprintf(`Extract company name and job positions from the job site page text below.
Return JSON with the following shape:
{
  "company_name": "会社名",
  "positions": [
    {
      "title": "職種名",
      "description": "仕事内容",
      "employment_type": "正社員",
      "work_location": "東京都",
      "remote_option": false,
      "min_salary": 300,
      "max_salary": 500,
      "required_skills": "[\"Java\",\"Spring Boot\"]",
      "preferred_skills": "[\"AWS\"]"
    }
  ]
}
Rules:
- Return 0 for salary fields not found in the text.
- Return "" for string fields not found in the text.
- Omit remote_option or set it to null when remote work is not mentioned in the text.
- required_skills and preferred_skills must be JSON arrays serialized as a string (e.g. "[\"Java\"]"), or "" if not found.
- min_salary and max_salary are annual salary in 万円 (integer).
- Do not fabricate data.

Text:
%s`, clean)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	content, err := s.aiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.2, 1200)
	if err != nil {
		return nil, err
	}
	var parsed jobListingExtraction
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
