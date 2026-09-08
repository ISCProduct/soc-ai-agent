package repositories

import (
	"fmt"
	"strings"

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

// guestEntryVisibilityGuard は「ゲストが投稿し、まだ公開されていない企業」を
// 指す行を除く SQL 条件を組み立てる（#1203）。
// cols には企業IDを持つ列を並べる（relations なら4つの端点、market_info なら1つ）。
// いずれかの列が非公開のゲスト投稿企業を指していれば、その行ごと返さない。
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
// 管理者が公開すると data_status='published' になり、以後は表示される。
// 注意: company_entry_submissions の行が消えるとガードは fail-open する
// （投稿履歴が無い＝ゲスト投稿ではない、と判定される）。現状この行を削除する
// コードは無いが、データ整理や削除要求で消すときは data_status も合わせること。
// 逆に公開後に却下されると is_active=false になる（data_status は published のまま）ため、
// そちらも非表示に含める。これが無いと相関図にノードだけ出て詳細が404になる。
//
// 走査対象は companies(全社) ではなく company_entry_submissions(ゲスト投稿のみ) 側で、
// 端点4本を1つの NOT EXISTS にまとめてある。NULL 端点は IN が一致しないため自然に通る
// （資本関係と取引関係で使う列が異なる）。
func guestEntryVisibilityGuard(cols ...string) string {
	return fmt.Sprintf(`NOT EXISTS (
	SELECT 1 FROM company_entry_submissions s
	JOIN companies c ON c.id = s.company_id
	WHERE (c.data_status <> 'published' OR c.is_active = false)
	  AND s.company_id IN (%s)
)`, strings.Join(cols, ", "))
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
		Where(guestEntryVisibilityGuard("parent_id", "child_id", "from_id", "to_id")).
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
		Where(guestEntryVisibilityGuard("parent_id", "child_id", "from_id", "to_id")).
		Find(&relations).Error
	return relations, err
}

// GetMarketInfoByCompanyID 指定企業の市場情報を取得
func (r *CompanyQueryRepository) GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error) {
	var marketInfo models.CompanyMarketInfo
	err := r.db.
		Preload("Company").
		Where("company_id = ?", companyID).
		Where(guestEntryVisibilityGuard("company_id")).
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
		Where(guestEntryVisibilityGuard("company_id")).
		Find(&marketInfos).Error
	return marketInfos, err
}

// GetJobPositionsByCompany 企業の公開済み求人一覧を取得
func (r *CompanyQueryRepository) GetJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error) {
	var positions []models.CompanyJobPosition
	err := r.db.
		Where("company_id = ? AND is_active = ? AND data_status = ?", companyID, true, "published").
		// 求人側の data_status だけでなく企業側も見る（#1203）。
		// 現状ゲスト投稿の求人は draft で作られるが、その前提が変わると素通りする。
		Where(guestEntryVisibilityGuard("company_id")).
		Preload("JobCategory").
		Order("created_at desc").
		Find(&positions).Error
	return positions, err
}

// GetCompanyByID 指定IDの企業を取得。
// 審査前のゲスト投稿は返さない（#1203）。
func (r *CompanyQueryRepository) GetCompanyByID(id uint) (*models.Company, error) {
	var company models.Company
	err := r.db.
		Where("id = ? AND is_active = ?", id, true).
		Where(guestEntryVisibilityGuard("companies.id")).
		First(&company).Error
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
	db = db.Where("is_active = ?", true).Where(guestEntryVisibilityGuard("companies.id"))
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
