package company

import (
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"strings"
)

func (f *CompanyRelationsFetcher) resultFromDB(company *models.Company, source, model, confidence string) (*CompanyRelationsResult, error) {
	existingNames := map[string]struct{}{}
	if f.relationRepo != nil {
		relations, err := f.relationRepo.GetRelationsByCompanyID(company.ID)
		if err != nil {
			return nil, err
		}
		for _, rel := range relations {
			for _, name := range relationPartnerNames(company.ID, rel) {
				existingNames[normalizeRelationName(name)] = struct{}{}
			}
		}
	}
	var marketInfo *CompanyMarketInfoResult
	if f.relationRepo != nil {
		if info, err := f.relationRepo.GetMarketInfoByCompanyID(company.ID); err == nil && info != nil {
			marketInfo = marketInfoFromModel(info)
		}
	}
	return f.buildResultFromExisting(company, existingNames, marketInfo, source, model, confidence)
}

func (f *CompanyRelationsFetcher) buildResultFromExisting(
	company *models.Company,
	_ map[string]struct{},
	marketInfo *CompanyMarketInfoResult,
	source, modelUsed, confidence string,
) (*CompanyRelationsResult, error) {
	if f.relationRepo == nil {
		return &CompanyRelationsResult{
			Relations:  nil,
			MarketInfo: marketInfo,
			Source:     source,
			ModelUsed:  modelUsed,
			Confidence: confidence,
		}, nil
	}
	relations, err := f.relationRepo.GetRelationsByCompanyID(company.ID)
	if err != nil {
		return nil, err
	}
	entries := relationsToEntries(company.ID, relations)
	return &CompanyRelationsResult{
		Relations:  entries,
		MarketInfo: marketInfo,
		Source:     source,
		SourceURL:  company.WebsiteURL,
		ModelUsed:  modelUsed,
		Confidence: confidence,
	}, nil
}

func mergeRelationsResult(
	existing []RelationEntry,
	baseMarket *CompanyMarketInfoResult,
	ai *CompanyRelationsResult,
) *CompanyRelationsResult {
	out := &CompanyRelationsResult{}
	byKey := map[string]RelationEntry{}
	order := make([]string, 0)

	addOrMerge := func(entry RelationEntry) {
		name := strings.TrimSpace(entry.Name)
		if !companyfetch.IsClearOrganizationName(name) {
			return
		}
		entry.Name = name
		entry.Description = companyfetch.SanitizeRelationDescription(entry.Description)
		key := normalizeRelationName(name) + "|" + entry.RelationType
		if prev, ok := byKey[key]; ok {
			if prev.Description == "" && entry.Description != "" {
				prev.Description = entry.Description
			}
			if prev.Ratio == nil && entry.Ratio != nil {
				prev.Ratio = entry.Ratio
			}
			byKey[key] = prev
			return
		}
		// 同名で種別違いがある場合、空の説明だけ AI 内容で補完
		norm := normalizeRelationName(name)
		if entry.Description != "" {
			for k, prev := range byKey {
				if normalizeRelationName(prev.Name) != norm {
					continue
				}
				if prev.Description == "" {
					prev.Description = entry.Description
					byKey[k] = prev
				}
			}
		}
		byKey[key] = entry
		order = append(order, key)
	}

	for _, entry := range existing {
		addOrMerge(entry)
	}
	if ai != nil {
		for _, entry := range ai.Relations {
			addOrMerge(entry)
		}
	}
	for _, key := range order {
		out.Relations = append(out.Relations, byKey[key])
	}
	// order に無い（AI 新規のみで addOrMerge が order に追加済み）— already in order via addOrMerge
	// ただし既存に無く AI で新規追加されたものは order に入る。OK

	if baseMarket != nil {
		out.MarketInfo = baseMarket
	} else if ai != nil {
		out.MarketInfo = ai.MarketInfo
	}
	if out.MarketInfo == nil && ai != nil {
		out.MarketInfo = ai.MarketInfo
	}
	if baseMarket != nil && ai != nil && ai.MarketInfo != nil {
		merged := *baseMarket
		if merged.StockCode == "" {
			merged.StockCode = ai.MarketInfo.StockCode
		}
		if merged.MarketType == "" {
			merged.MarketType = ai.MarketInfo.MarketType
		}
		if !merged.IsListed && ai.MarketInfo.IsListed {
			merged.IsListed = ai.MarketInfo.IsListed
		}
		out.MarketInfo = &merged
	}
	return out
}

func relationsToEntries(companyID uint, relations []models.CompanyRelation) []RelationEntry {
	entries := make([]RelationEntry, 0, len(relations))
	for _, rel := range relations {
		name, relationType := relationEntryFromModel(companyID, rel)
		if name == "" {
			continue
		}
		entries = append(entries, RelationEntry{
			Name:         name,
			RelationType: relationType,
			Ratio:        rel.Ratio,
			Description:  companyfetch.SanitizeRelationDescription(rel.Description),
		})
	}
	return entries
}

func relationEntryFromModel(companyID uint, rel models.CompanyRelation) (name, relationType string) {
	relationType = rel.RelationType
	if models.IsCapitalRelationType(relationType) {
		if rel.ParentID != nil && *rel.ParentID == companyID && rel.Child != nil {
			return rel.Child.Name, relationType
		}
		if rel.ChildID != nil && *rel.ChildID == companyID && rel.Parent != nil {
			return rel.Parent.Name, relationType
		}
		return "", relationType
	}
	if rel.FromID != nil && *rel.FromID == companyID && rel.To != nil {
		return rel.To.Name, relationType
	}
	if rel.ToID != nil && *rel.ToID == companyID && rel.From != nil {
		return rel.From.Name, relationType
	}
	return "", relationType
}

func relationPartnerNames(companyID uint, rel models.CompanyRelation) []string {
	name, _ := relationEntryFromModel(companyID, rel)
	if name == "" {
		return nil
	}
	return []string{name}
}

func marketInfoFromModel(info *models.CompanyMarketInfo) *CompanyMarketInfoResult {
	if info == nil {
		return nil
	}
	return &CompanyMarketInfoResult{
		IsListed:   info.IsListed,
		MarketType: info.MarketType,
		StockCode:  info.StockCode,
	}
}

func normalizeRelationName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeMarketType(marketType string, isListed bool) string {
	m := strings.ToLower(strings.TrimSpace(marketType))
	switch m {
	case "prime", "standard", "growth", "unlisted":
		return m
	}
	if isListed {
		return "standard"
	}
	return "unlisted"
}

func markRelationsCacheSkip(r *CompanyRelationsResult, reason string, budgetExceeded bool) *CompanyRelationsResult {
	if r == nil {
		r = &CompanyRelationsResult{}
	}
	r.FromCache = true
	r.SkipReason = reason
	r.BudgetExceeded = budgetExceeded
	return r
}

func relationsResultHasData(r *CompanyRelationsResult) bool {
	if r == nil {
		return false
	}
	if len(r.Relations) > 0 {
		return true
	}
	if r.SavedCount > 0 {
		// SavedCount は meaningful market / 関係の upsert 成功数のみを数える
		return true
	}
	if r.MarketInfo == nil {
		return false
	}
	return companyfetch.HasMeaningfulMarketInfo(
		r.MarketInfo.IsListed, r.MarketInfo.MarketType, r.MarketInfo.StockCode,
	)
}
