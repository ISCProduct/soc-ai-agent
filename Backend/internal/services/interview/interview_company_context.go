package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/company"
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

// sttHintCompany は音声認識の補助語に使える企業名・読みを DB から返す。
//
// 解決できなければ両方空にする。補助語は「御社」＋ companyInfo の技術用語だけになり、
// #1603 で測った固有名詞の改善は DB 登録済みの企業で保たれる。
// クライアント直値を補助語へ入れないのが目的なので、ここで LLM 補完
// （lookupCompanyReading）は使わない。STT はターンの先頭にあり、
// ここに LLM 呼び出しを足すと応答遅延がそのまま増える。
func (s *InterviewService) sttHintCompany(companyID uint, companyName string) (string, string) {
	c := s.findCompany(companyID, companyName)
	if c == nil {
		return "", ""
	}
	return strings.TrimSpace(c.Name), strings.TrimSpace(c.NameReading)
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
