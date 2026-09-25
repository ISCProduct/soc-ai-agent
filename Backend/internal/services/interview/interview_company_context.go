package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/company"
	"context"
	"strings"
	"time"
)

const companyReadingCacheTTL = 24 * time.Hour

type companyReadingCacheEntry struct {
	value     string
	expiresAt time.Time
}

// resolveCompanyInfo は共有企業情報を優先し、無ければクライアント文面を使う。
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
		entry, ok := cached.(companyReadingCacheEntry)
		if ok && time.Now().Before(entry.expiresAt) {
			return entry.value
		}
		s.companyReadingCache.Delete(cacheKey)
	}
	value, err, shared := s.companyReadingFlight.Do(cacheKey, func() (any, error) {
		if cached, ok := s.companyReadingCache.Load(cacheKey); ok {
			if entry, ok := cached.(companyReadingCacheEntry); ok &&
				time.Now().Before(entry.expiresAt) {
				return entry.value, nil
			}
			s.companyReadingCache.Delete(cacheKey)
		}
		reading, err := s.lookupCompanyReading(ctx, companyName)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(reading) != "" {
			s.companyReadingCache.Store(cacheKey, companyReadingCacheEntry{
				value:     strings.TrimSpace(reading),
				expiresAt: time.Now().Add(companyReadingCacheTTL),
			})
		}
		return reading, nil
	})
	if err != nil && shared && ctx.Err() == nil {
		reading, retryErr := s.lookupCompanyReading(ctx, companyName)
		if retryErr == nil && strings.TrimSpace(reading) != "" {
			s.companyReadingCache.Store(cacheKey, companyReadingCacheEntry{
				value:     strings.TrimSpace(reading),
				expiresAt: time.Now().Add(companyReadingCacheTTL),
			})
		}
		return reading
	}
	reading, _ := value.(string)
	return reading
}
