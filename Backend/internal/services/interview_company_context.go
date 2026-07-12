package services

import (
	"Backend/internal/models"
	"context"
	"strings"
)

// resolveCompanyInfo は共有キャッシュの brief を優先し、無ければクライアント文面を使う。
// Search/LLM 調査はしない。companyType（general/sier）ではゲートしない。
func (s *InterviewService) resolveCompanyInfo(companyID uint, companyName, clientInfo string) string {
	brief := ""
	if companyID > 0 || strings.TrimSpace(companyName) != "" {
		brief = s.lookupCompanyProfile(companyID, companyName)
	}
	if brief != "" {
		return brief
	}
	return strings.TrimSpace(clientInfo)
}

// lookupCompanyProfile は共有キャッシュ（companies）から企業スナップショットを読む。
// Search / LLM による企業調査は行わない。未登録・未整備なら空文字。
func (s *InterviewService) lookupCompanyProfile(companyID uint, companyName string) string {
	company := s.findCompany(companyID, companyName)
	if company == nil {
		return ""
	}
	var profile *models.CompanyWeightProfile
	if s.companyRepo != nil {
		if p, err := s.companyRepo.GetWeightProfile(company.ID, nil); err == nil {
			profile = p
		}
	}
	return BuildCompanyBrief(company, profile)
}

func (s *InterviewService) findCompany(companyID uint, companyName string) *models.Company {
	if s.companyRepo == nil {
		return nil
	}
	if companyID > 0 {
		if c, err := s.companyRepo.FindByID(companyID); err == nil && c != nil {
			return c
		}
	}
	name := strings.TrimSpace(companyName)
	if name == "" {
		return nil
	}
	c, err := s.companyRepo.FindByName(name)
	if err != nil || c == nil {
		return nil
	}
	return c
}

// resolveCompanyReading は共有DBの NameReading を優先し、無ければモデル知識で補完する。
func (s *InterviewService) resolveCompanyReading(ctx context.Context, companyID uint, companyName string) string {
	if company := s.findCompany(companyID, companyName); company != nil {
		if reading := strings.TrimSpace(company.NameReading); reading != "" {
			return reading
		}
	}
	return s.lookupCompanyReading(ctx, companyName)
}
