package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const factRelationDescSuffix = "（出所: 公開情報）"

type factCapitalRelationSpec struct {
	parentCorpNum string
	parentName    string
	childCorpNum  string
	childName     string
	relationType  string
	ratio         float64
	note          string
}

type factBusinessRelationSpec struct {
	fromCorpNum  string
	fromName     string
	toCorpNum    string
	toName       string
	relationType string
	note         string
}

// 公開されているグループ体制・子会社一覧等に基づく資本関係（法人番号で特定）。
var factCapitalRelations = []factCapitalRelationSpec{
	{
		parentCorpNum: "7010401026738", parentName: "トヨタ自動車株式会社",
		childCorpNum: "7180301018923", childName: "トヨタテクニカルディベロップメント株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "トヨタ自動車の完全子会社",
	},
	{
		parentCorpNum: "7010401026738", parentName: "トヨタ自動車株式会社",
		childCorpNum: "9180301002173", childName: "ベルエアーシステムズ株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "トヨタ自動車グループの連結子会社",
	},
	{
		parentCorpNum: "7010401026738", parentName: "トヨタ自動車株式会社",
		childCorpNum: "5180301037157", childName: "株式会社ビーネックスソリューションズ",
		relationType: "capital_affiliate", ratio: 50,
		note: "トヨタ自動車グループ（ITサービス）",
	},
	{
		parentCorpNum: "4180301012460", parentName: "株式会社豊田自動織機",
		childCorpNum: "5180301014305", childName: "株式会社豊田自動織機ＩＴソリューションズ",
		relationType: "capital_subsidiary", ratio: 100,
		note: "豊田自動織機の子会社",
	},
	{
		parentCorpNum: "4010401019905", parentName: "NEC株式会社",
		childCorpNum: "7010601022674", childName: "ＮＥＣソリューションイノベータ株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "NECの連結子会社",
	},
	{
		parentCorpNum: "8010105001109", parentName: "株式会社三菱ＵＦＪフィナンシャル・グループ",
		childCorpNum: "6010001008770", childName: "三菱ＵＦＪ信託銀行株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "MUFGグループの連結子会社",
	},
	{
		parentCorpNum: "8010105001109", parentName: "株式会社三菱ＵＦＪフィナンシャル・グループ",
		childCorpNum: "8010001000016", childName: "三菱ＵＦＪニコス株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "MUFGグループの連結子会社",
	},
	{
		parentCorpNum: "9010401013693", parentName: "ヤマトホールディングス株式会社",
		childCorpNum: "9010601029263", childName: "ヤマトシステム開発株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "ヤマトホールディングスグループの子会社",
	},
	{
		parentCorpNum: "4010401010393", parentName: "株式会社内田洋行",
		childCorpNum: "3010401099784", childName: "株式会社内田洋行ＩＴソリューションズ",
		relationType: "capital_subsidiary", ratio: 100,
		note: "内田洋行の子会社",
	},
	{
		parentCorpNum: "3010401028474", parentName: "パーソルホールディングス株式会社",
		childCorpNum: "3180001032055", childName: "パーソルクロステクノロジー株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "パーソルホールディングスグループの子会社",
	},
	{
		parentCorpNum: "9060001000184", parentName: "味の素株式会社",
		childCorpNum: "2500001014640", childName: "サンヨー食品株式会社",
		relationType: "capital_subsidiary", ratio: 100,
		note: "味の素グループ（2010年に買収）",
	},
}

var factBusinessRelations = []factBusinessRelationSpec{
	{
		fromCorpNum: "6010001008770", fromName: "三菱ＵＦＪ信託銀行株式会社",
		toCorpNum: "8010001000016", toName: "三菱ＵＦＪニコス株式会社",
		relationType: "business_partner",
		note: "MUFGグループ内の金融サービス連携",
	},
}

// seedCompanyRelations は公開情報に基づく実在企業の関係を投入する（冪等）。
func seedCompanyRelations(db *gorm.DB) error {
	if err := db.Where("description LIKE ?", "%開発用シード%").Delete(&CompanyRelation{}).Error; err != nil {
		return err
	}

	var factCount int64
	if err := db.Model(&CompanyRelation{}).
		Where("description LIKE ?", "%"+factRelationDescSuffix).
		Count(&factCount).Error; err != nil {
		return err
	}
	if factCount > 0 {
		return nil
	}

	for _, spec := range factCapitalRelations {
		if err := upsertFactCapitalRelation(db, spec); err != nil {
			return fmt.Errorf("capital relation %s→%s: %w", spec.parentCorpNum, spec.childCorpNum, err)
		}
	}
	for _, spec := range factBusinessRelations {
		if err := upsertFactBusinessRelation(db, spec); err != nil {
			return fmt.Errorf("business relation %s→%s: %w", spec.fromCorpNum, spec.toCorpNum, err)
		}
	}

	return nil
}

func upsertFactCapitalRelation(db *gorm.DB, spec factCapitalRelationSpec) error {
	parentID, err := ensureCompanyByCorporateNumber(db, spec.parentCorpNum, spec.parentName)
	if err != nil {
		return err
	}
	childID, err := ensureCompanyByCorporateNumber(db, spec.childCorpNum, spec.childName)
	if err != nil {
		return err
	}

	relationType := spec.relationType
	if !IsCapitalRelationType(relationType) {
		relationType = "capital_affiliate"
	}

	description := spec.note + factRelationDescSuffix
	ratio := spec.ratio

	var existing CompanyRelation
	err = db.Where("parent_id = ? AND child_id = ? AND relation_type = ?", parentID, childID, relationType).
		First(&existing).Error
	if err == nil {
		existing.Description = description
		existing.Ratio = &ratio
		existing.IsActive = true
		return db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return db.Create(&CompanyRelation{
		ParentID:     &parentID,
		ChildID:      &childID,
		RelationType: relationType,
		Ratio:        &ratio,
		Description:  description,
		IsActive:     true,
	}).Error
}

func upsertFactBusinessRelation(db *gorm.DB, spec factBusinessRelationSpec) error {
	fromID, err := ensureCompanyByCorporateNumber(db, spec.fromCorpNum, spec.fromName)
	if err != nil {
		return err
	}
	toID, err := ensureCompanyByCorporateNumber(db, spec.toCorpNum, spec.toName)
	if err != nil {
		return err
	}

	description := spec.note + factRelationDescSuffix
	relationType := spec.relationType
	if relationType == "" {
		relationType = "business_partner"
	}

	var existing CompanyRelation
	err = db.Where("from_id = ? AND to_id = ? AND relation_type = ?", fromID, toID, relationType).
		First(&existing).Error
	if err == nil {
		existing.Description = description
		existing.IsActive = true
		return db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return db.Create(&CompanyRelation{
		FromID:       &fromID,
		ToID:         &toID,
		RelationType: relationType,
		Description:  description,
		IsActive:     true,
	}).Error
}

func ensureCompanyByCorporateNumber(db *gorm.DB, corpNum, name string) (uint, error) {
	var company Company
	err := db.Where("corporate_number = ?", corpNum).First(&company).Error
	if err == nil {
		return company.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	// 法人番号が未設定の既存行（L1カタログ等）を名称で補完
	if err := db.Where("corporate_number = '' AND name = ?", name).First(&company).Error; err == nil {
		company.CorporateNumber = corpNum
		if saveErr := db.Save(&company).Error; saveErr != nil {
			return 0, saveErr
		}
		return company.ID, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	company = Company{
		Name:            name,
		CorporateNumber: corpNum,
		IsActive:        true,
		DataStatus:      "published",
		SourceType:      "public_registry",
		IsProvisional:   false,
	}
	if err := db.Create(&company).Error; err != nil {
		return 0, err
	}
	return company.ID, nil
}
