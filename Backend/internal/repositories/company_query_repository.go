package repositories

import (
	"fmt"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// CompanyQueryRepository は CompanyRelationQueryRepository インターフェースの実装。
type CompanyQueryRepository struct {
	db *gorm.DB
}

func NewCompanyQueryRepository(db *gorm.DB) *CompanyQueryRepository {
	return &CompanyQueryRepository{db: db}
}

// unreviewedGuestEntrySubQuery は「ゲストが投稿し、まだ管理者に公開されていない企業」を
// 選ぶサブクエリ条件（#1203）。
//
// /company-entry は無認証で投稿でき（honeypot とレート制限のみ）、
// 作られる企業は data_status='draft' で始まる。審査前にそのまま公開APIへ出ると、
// 任意の内容の企業を作って即座に学生へ露出させられる。
//
// data_status だけで弾けない理由:
// draft は自動収集した全企業の既定状態でもあり（crawl_*/gbizinfo 等）、
// 実データでは 842社中 752社が draft。data_status='published' で絞ると
// 公開企業一覧が9割減し、学生の企業検索・面接の企業選択・履歴書サジェストが
// 軒並み空になる。source_type='manual' も crawl_service/l1_seed_service が使うため
// 識別子にならない。
//
// ゲスト投稿は company_entry_submissions に必ず行が作られる
// （company_entry_service.go）ので、これを唯一の識別子とする。
// 管理者が公開した時点で data_status='published' になり、以後は表示される。
const unreviewedGuestEntryCondition = `EXISTS (
	SELECT 1 FROM company_entry_submissions s
	WHERE s.company_id = %s AND %s <> 'published'
)`

// excludeUnreviewedGuestEntries は無認証APIから審査前のゲスト投稿企業を除く。
// companiesAlias は companies テーブル（または結合先）の別名。
func excludeUnreviewedGuestEntries(db *gorm.DB, companiesAlias string) *gorm.DB {
	idCol := companiesAlias + ".id"
	statusCol := companiesAlias + ".data_status"
	return db.Where("NOT " + fmt.Sprintf(unreviewedGuestEntryCondition, idCol, statusCol))
}

// visibleCompanyIDsSubQuery は公開してよい企業IDのサブクエリ。
// relations / market_info のように companies を直接 FROM に持たない
// クエリで、関連先の企業を絞るのに使う。
func visibleCompanyIDsSubQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&models.Company{}).
		Select("id").
		Where("NOT " + fmt.Sprintf(unreviewedGuestEntryCondition, "companies.id", "companies.data_status"))
}

// relationEndpointsVisible は関係の端点(parent/child/from/to)がすべて
// 公開してよい企業であることを要求する条件を返す（#1203）。
// NULL の端点は条件を通す（資本関係と取引関係で使う列が異なるため）。
func relationEndpointsVisible(db *gorm.DB) *gorm.DB {
	visible := visibleCompanyIDsSubQuery(db)
	cond := db.Session(&gorm.Session{NewDB: true})
	for _, col := range []string{"parent_id", "child_id", "from_id", "to_id"} {
		cond = cond.Where(col+" IS NULL OR "+col+" IN (?)", visible)
	}
	return cond
}

// GetByCompanyID 指定企業IDに関連する企業関係を取得
func (r *CompanyQueryRepository) GetByCompanyID(companyID uint) ([]models.CompanyRelation, error) {
	var relations []models.CompanyRelation
	err := r.db.
		Preload("Parent").
		Preload("Child").
		Preload("From").
		Preload("To").
		Where("parent_id = ? OR child_id = ? OR from_id = ? OR to_id = ?",
			companyID, companyID, companyID, companyID).
		Where("is_active = ?", true).
		// 端点のいずれかが審査前のゲスト投稿なら、その関係ごと返さない（#1203）。
		// Preload("Parent"/"Child"/"From"/"To") が models.Company を丸ごと返すため、
		// ここを塞がないと企業一覧を絞っても隣から読めてしまう。
		Where(relationEndpointsVisible(r.db)).
		Find(&relations).Error
	return relations, err
}

// GetAll 全企業関係を取得
func (r *CompanyQueryRepository) GetAll() ([]models.CompanyRelation, error) {
	var relations []models.CompanyRelation
	err := r.db.
		Preload("Parent").
		Preload("Child").
		Preload("From").
		Preload("To").
		Where("is_active = ?", true).
		Where(relationEndpointsVisible(r.db)).
		Find(&relations).Error
	return relations, err
}

// GetMarketInfoByCompanyID 指定企業の市場情報を取得
func (r *CompanyQueryRepository) GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error) {
	var marketInfo models.CompanyMarketInfo
	err := r.db.
		Preload("Company").
		Where("company_id = ?", companyID).
		Where("company_id IN (?)", visibleCompanyIDsSubQuery(r.db)).
		First(&marketInfo).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &marketInfo, nil
}

// GetAllMarketInfo 全企業の市場情報を取得
func (r *CompanyQueryRepository) GetAllMarketInfo() ([]models.CompanyMarketInfo, error) {
	var marketInfos []models.CompanyMarketInfo
	err := r.db.
		Preload("Company").
		Where("company_id IN (?)", visibleCompanyIDsSubQuery(r.db)).
		Find(&marketInfos).Error
	return marketInfos, err
}

// GetJobPositionsByCompany 企業の公開済み求人一覧を取得
func (r *CompanyQueryRepository) GetJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error) {
	var positions []models.CompanyJobPosition
	err := r.db.
		Where("company_id = ? AND is_active = ? AND data_status = ?", companyID, true, "published").
		Preload("JobCategory").
		Order("created_at desc").
		Find(&positions).Error
	return positions, err
}

// GetCompanyByID 指定IDの企業を取得。
// 審査前のゲスト投稿は返さない（#1203）。
func (r *CompanyQueryRepository) GetCompanyByID(id uint) (*models.Company, error) {
	var company models.Company
	err := excludeUnreviewedGuestEntries(
		r.db.Where("id = ? AND is_active = ?", id, true), "companies",
	).First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// GetCompaniesFiltered フィルタリングされた企業一覧と総件数を取得
func (r *CompanyQueryRepository) GetCompaniesFiltered(limit, offset int, industry, name, tech string) ([]models.Company, int64, error) {
	var total int64
	if err := applyCompanyFilters(r.db.Model(&models.Company{}), industry, name, tech).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "updated_at DESC, id ASC"
	if name != "" {
		order = "name ASC"
	}

	var companies []models.Company
	err := applyCompanyFilters(r.db, industry, name, tech).
		Limit(limit).
		Offset(offset).
		Order(order).
		Find(&companies).Error
	return companies, total, err
}

// applyCompanyFilters は無認証の企業検索に共通の絞り込みを適用する。
// 審査前のゲスト投稿を除く理由は unreviewedGuestEntryCondition のコメント参照（#1203）。
func applyCompanyFilters(db *gorm.DB, industry, name, tech string) *gorm.DB {
	db = excludeUnreviewedGuestEntries(db.Where("is_active = ?", true), "companies")
	if industry != "" {
		db = db.Where("industry = ?", industry)
	}
	if name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if tech != "" {
		like := "%" + tech + "%"
		db = db.Where("tech_stack LIKE ? OR infra_stack LIKE ? OR cicd_tools LIKE ?", like, like, like)
	}
	return db
}
