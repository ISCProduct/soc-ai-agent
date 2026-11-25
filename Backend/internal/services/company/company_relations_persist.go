package company

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/config"
	"Backend/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

func (f *CompanyRelationsFetcher) persistResult(ctx context.Context, company *models.Company, result *CompanyRelationsResult) (int, error) {
	if f.relationRepo == nil {
		return 0, fmt.Errorf("relation repository is nil")
	}
	saved := 0
	autofetched := 0
	autofetchMax := config.RelationAutofetchMax()
	now := time.Now()
	sourceTag := result.Source
	if sourceTag == "" {
		sourceTag = companyfetch.SourceWebSearch
	}

	for _, entry := range result.Relations {
		name := strings.TrimSpace(entry.Name)
		if !companyfetch.IsClearOrganizationName(name) {
			continue
		}
		toCompany, err := f.companyRepo.FindByName(name)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			continue
		}
		if toCompany == nil {
			toCompany = &models.Company{
				Name:            name,
				SourceType:      sourceTag,
				SourceFetchedAt: &now,
				IsProvisional:   true,
				DataStatus:      "draft",
			}
			if err := f.companyRepo.Create(toCompany); err != nil {
				continue
			}
			// 新規作成した関連企業自体の詳細情報(業種・住所・URL等)を埋める。
			// 失敗しても関係の保存自体はブロックしない(空データのまま残るがdraft/is_provisionalで
			// 識別でき、後から個別に再取得できるため)。
			if f.infoFetcher != nil && autofetched < autofetchMax {
				autofetched++
				if _, err := f.infoFetcher.FetchAndSave(ctx, toCompany.ID, false); err != nil {
					log.Printf("[CompanyRelations] 関連企業%q(id=%d)の詳細情報取得に失敗: %v", name, toCompany.ID, err)
				}
			}
		}
		// 種別ラベル（主要取引先など）は保存せず、具体的な取引内容のみ残す
		desc := companyfetch.SanitizeRelationDescription(entry.Description)
		var upsertErr error
		if models.IsCapitalRelationType(entry.RelationType) {
			ratio := entry.Ratio
			if ratio == nil && entry.RelationType == "capital_subsidiary" {
				r := 100.0
				ratio = &r
			}
			upsertErr = f.relationRepo.UpsertCapitalRelation(company.ID, toCompany.ID, entry.RelationType, ratio, desc)
		} else {
			relationType := entry.RelationType
			if relationType == "" {
				relationType = "business_partner"
			}
			upsertErr = f.relationRepo.UpsertBusinessRelation(company.ID, toCompany.ID, relationType, desc)
		}
		if upsertErr != nil {
			continue
		}
		saved++
	}

	if result.MarketInfo != nil && companyfetch.HasMeaningfulMarketInfo(
		result.MarketInfo.IsListed, result.MarketInfo.MarketType, result.MarketInfo.StockCode,
	) {
		marketType := normalizeMarketType(result.MarketInfo.MarketType, result.MarketInfo.IsListed)
		info := &models.CompanyMarketInfo{
			CompanyID:  company.ID,
			MarketType: marketType,
			IsListed:   result.MarketInfo.IsListed || marketType != "unlisted",
			StockCode:  strings.TrimSpace(result.MarketInfo.StockCode),
		}
		if err := f.relationRepo.UpsertMarketInfo(info); err == nil {
			saved++
		}
	}
	return saved, nil
}
