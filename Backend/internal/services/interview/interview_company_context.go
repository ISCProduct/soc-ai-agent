package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/company"
	"context"
	"fmt"
	"strings"
)

// resolveCompanyInfo は共有キャッシュの brief を優先し、無ければクライアント文面を使う。
// Search/LLM 調査はしない。companyType（general/sier）ではゲートしない。
func (s *InterviewService) resolveCompanyInfo(companyID uint, companyName, clientInfo string) string {
	brief := ""
	if companyID > 0 || strings.TrimSpace(companyName) != "" {
		cacheKey := fmt.Sprintf("%d:%s", companyID, strings.TrimSpace(companyName))
		if cached, ok := s.companyProfileCache.Load(cacheKey); ok {
			brief, _ = cached.(string)
		} else {
			value, _, _ := s.companyProfileFlight.Do(cacheKey, func() (any, error) {
				if cached, ok := s.companyProfileCache.Load(cacheKey); ok {
					return cached, nil
				}
				lookup := s.lookupCompanyProfile(companyID, companyName)
				if strings.TrimSpace(lookup) != "" {
					s.companyProfileCache.Store(cacheKey, lookup)
				}
				return lookup, nil
			})
			brief, _ = value.(string)
		}
	}
	if brief != "" {
		return brief
	}
	return strings.TrimSpace(clientInfo)
}

// lookupCompanyProfile は共有キャッシュ（companies）から企業スナップショットを読む。
// Search / LLM による企業調査は行わない。未登録・未整備なら空文字。
func (s *InterviewService) lookupCompanyProfile(companyID uint, companyName string) string {
	comp := s.findCompany(companyID, companyName)
	if comp == nil {
		return ""
	}
	var profile *models.CompanyWeightProfile
	if s.companyRepo != nil {
		if p, err := s.companyRepo.GetWeightProfile(comp.ID, nil); err == nil {
			profile = p
		}
	}
	return company.BuildCompanyBrief(comp, profile)
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

// resolveCompanyID は company_id=0（WEB検索・手入力）でも企業名が DB と一致すれば ID を解決する。
// 解決できない場合は 0 のまま（カスタム質問・深掘りは無効）。
func (s *InterviewService) resolveCompanyID(companyID uint, companyName string) uint {
	if company := s.findCompany(companyID, companyName); company != nil {
		return company.ID
	}
	return 0
}

// resolveCompanyReading は共有DBの NameReading を優先し、無ければモデル知識で補完する。
func (s *InterviewService) resolveCompanyReading(ctx context.Context, companyID uint, companyName string) string {
	if company := s.findCompany(companyID, companyName); company != nil {
		if reading := strings.TrimSpace(company.NameReading); reading != "" {
			return reading
		}
	}
	cacheKey := strings.TrimSpace(companyName)
	if cached, ok := s.companyReadingCache.Load(cacheKey); ok {
		reading, _ := cached.(string)
		return reading
	}
	value, _, _ := s.companyReadingFlight.Do(cacheKey, func() (any, error) {
		if cached, ok := s.companyReadingCache.Load(cacheKey); ok {
			return cached, nil
		}
		reading := s.lookupCompanyReading(ctx, companyName)
		if strings.TrimSpace(reading) != "" {
			s.companyReadingCache.Store(cacheKey, reading)
		}
		return reading, nil
	})
	reading, _ := value.(string)
	return reading
}
