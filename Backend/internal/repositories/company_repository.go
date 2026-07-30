package repositories

import (
	"Backend/internal/companyfetch"
	"Backend/internal/companyfields"
	"Backend/internal/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CompanyRepository struct {
	db *gorm.DB
}

func NewCompanyRepository(db *gorm.DB) *CompanyRepository {
	return &CompanyRepository{db: db}
}

func (r *CompanyRepository) DB() *gorm.DB {
	return r.db
}

// FindAllActive アクティブな企業をページネーション付きで取得
func (r *CompanyRepository) FindAllActive(limit, offset int) ([]models.Company, error) {
	var companies []models.Company
	err := r.db.Where("is_active = ?", true).
		Order("id desc").
		Limit(limit).Offset(offset).
		Find(&companies).Error
	return companies, err
}

// CountActive アクティブ企業数を取得
func (r *CompanyRepository) CountActive() (int64, error) {
	var count int64
	err := r.db.Model(&models.Company{}).
		Where("is_active = ?", true).
		Count(&count).Error
	return count, err
}

// companyListSelectColumns は管理画面一覧向けの軽量カラム（text 過取得を避ける）。
// description / tech_stack は不足判定に必要なため残す。
const companyListSelectColumns = "id, name, industry, location, source_type, is_provisional, data_status, " +
	"info_fetched_at, jobs_fetched_at, tech_fetched_at, relations_fetched_at, " +
	"website_url, description, tech_stack, created_at, updated_at"

// ListActiveFiltered は名前・公開ステータス・業界・情報充足で絞り込んだアクティブ企業を返す。
// status は "draft" / "published" / ""（指定なし=すべて）。
// industry が "__unset__" のときは業界未設定のみ。
// readiness は "ready"（主3種そろい）/ "missing"（主3種不足）/ ""。
// orderBy が "industry" のときは業界順、それ以外は id desc。
func (r *CompanyRepository) ListActiveFiltered(limit, offset int, name, status, industry, readiness, orderBy string) ([]models.Company, int64, error) {
	q := r.db.Model(&models.Company{}).Where("is_active = ?", true).Session(&gorm.Session{})
	if name = strings.TrimSpace(name); name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	switch strings.TrimSpace(status) {
	case "draft", "published":
		q = q.Where("data_status = ?", status)
	}
	switch industry = strings.TrimSpace(industry); industry {
	case "":
		// no-op
	case "__unset__":
		q = q.Where("(industry IS NULL OR industry = '')")
	default:
		q = q.Where("industry = ?", industry)
	}
	const (
		infoReady = `(info_fetched_at IS NOT NULL AND COALESCE(description, '') <> '' AND COALESCE(website_url, '') <> '')`
		techReady = `(tech_fetched_at IS NOT NULL AND TRIM(COALESCE(tech_stack, '')) <> '' AND TRIM(COALESCE(tech_stack, '')) NOT IN ('[]', 'null', '{}'))`
		relReady  = `(relations_fetched_at IS NOT NULL)`
	)
	techRequiredSQL, techRequiredArgs := companyfields.TechRequiredIndustrySQL("industry")
	// 技術必須業界のみ techReady を要求。それ以外は会社概要+関連企業で充足とみなす。
	primaryReady := "(" + infoReady + " AND " + relReady + " AND (" + techReady + " OR NOT " + techRequiredSQL + "))"
	switch strings.TrimSpace(readiness) {
	case "ready":
		q = q.Where(primaryReady, techRequiredArgs...)
	case "missing":
		q = q.Where("NOT "+primaryReady, techRequiredArgs...)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "id desc"
	if strings.TrimSpace(orderBy) == "industry" {
		order = "CASE WHEN industry IS NULL OR industry = '' THEN 1 ELSE 0 END ASC, industry ASC, id DESC"
	}

	var companies []models.Company
	err := q.Session(&gorm.Session{}).Select(companyListSelectColumns).
		Order(order).Limit(limit).Offset(offset).Find(&companies).Error
	return companies, total, err
}

// ListActiveIndustries はアクティブ企業に設定されている業界名の重複なし一覧を返す。
func (r *CompanyRepository) ListActiveIndustries() ([]string, error) {
	var industries []string
	err := r.db.Model(&models.Company{}).
		Where("is_active = ? AND industry IS NOT NULL AND industry <> ''", true).
		Distinct().
		Order("industry ASC").
		Pluck("industry", &industries).Error
	return industries, err
}

// FindAllActiveNames アクティブ企業のIDと名前を取得（フィルタ q で部分一致）
func (r *CompanyRepository) FindAllActiveNames(q string) ([]models.CompanyName, error) {
	var names []models.CompanyName
	query := r.db.Model(&models.Company{}).Select("id, name").Where("is_active = ?", true)
	if q = strings.TrimSpace(q); q != "" {
		query = query.Where("name LIKE ?", "%"+q+"%")
	}
	err := query.Order("name asc").Find(&names).Error
	return names, err
}

// FindAllPublished 公開済み企業をページネーション付きで取得（マッチング用）
func (r *CompanyRepository) FindAllPublished(limit, offset int) ([]models.Company, error) {
	var companies []models.Company
	err := r.db.Where("is_active = ? AND data_status = ?", true, "published").
		Order("id desc").
		Limit(limit).Offset(offset).
		Find(&companies).Error
	return companies, err
}

// CountPublished 公開済み企業数を取得
func (r *CompanyRepository) CountPublished() (int64, error) {
	var count int64
	err := r.db.Model(&models.Company{}).
		Where("is_active = ? AND data_status = ?", true, "published").
		Count(&count).Error
	return count, err
}

// FindByID IDで企業を取得
func (r *CompanyRepository) FindByID(id uint) (*models.Company, error) {
	var company models.Company
	err := r.db.First(&company, id).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// FindByIDs は複数IDの企業をバッチ取得する。
func (r *CompanyRepository) FindByIDs(ids []uint) ([]models.Company, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var companies []models.Company
	err := r.db.Where("id IN ?", ids).Find(&companies).Error
	return companies, err
}

// FindByName 企業名で取得
func (r *CompanyRepository) FindByName(name string) (*models.Company, error) {
	var company models.Company
	err := r.db.Where("name = ?", name).First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// FindByCorporateNumber 法人番号で企業を取得
func (r *CompanyRepository) FindByCorporateNumber(corporateNumber string) (*models.Company, error) {
	var company models.Company
	err := r.db.Where("corporate_number = ?", corporateNumber).First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// GetWeightProfile 企業の重視度プロファイルを取得
func (r *CompanyRepository) GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error) {
	var profile models.CompanyWeightProfile
	query := r.db.Where("company_id = ?", companyID)

	if jobPositionID != nil {
		query = query.Where("job_position_id = ?", *jobPositionID)
	} else {
		query = query.Where("job_position_id IS NULL")
	}

	err := query.First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetWeightProfilesByCompanyIDs は会社単位プロファイル（job_position_id IS NULL）を一括取得する。
func (r *CompanyRepository) GetWeightProfilesByCompanyIDs(companyIDs []uint) (map[uint]*models.CompanyWeightProfile, error) {
	out := make(map[uint]*models.CompanyWeightProfile, len(companyIDs))
	if len(companyIDs) == 0 {
		return out, nil
	}
	var profiles []models.CompanyWeightProfile
	err := r.db.Where("company_id IN ? AND job_position_id IS NULL", companyIDs).Find(&profiles).Error
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		out[profiles[i].CompanyID] = &profiles[i]
	}
	return out, nil
}

// Create 企業を作成
func (r *CompanyRepository) Create(company *models.Company) error {
	return r.db.Create(company).Error
}

// Update 企業情報を更新
func (r *CompanyRepository) Update(company *models.Company) error {
	return r.db.Save(company).Error
}

// FindJobPositionByCompanyAndTitle 企業IDと職種タイトルで募集職種を取得
func (r *CompanyRepository) FindJobPositionByCompanyAndTitle(companyID uint, title string) (*models.CompanyJobPosition, error) {
	var position models.CompanyJobPosition
	err := r.db.Where("company_id = ? AND title = ?", companyID, title).First(&position).Error
	if err != nil {
		return nil, err
	}
	return &position, nil
}

// FindJobPositionByID 募集職種をIDで取得
func (r *CompanyRepository) FindJobPositionByID(id uint) (*models.CompanyJobPosition, error) {
	var position models.CompanyJobPosition
	err := r.db.First(&position, id).Error
	if err != nil {
		return nil, err
	}
	return &position, nil
}

// CreateJobPosition 募集職種を作成
func (r *CompanyRepository) CreateJobPosition(position *models.CompanyJobPosition) error {
	db := r.db
	if position.JobCategoryID == 0 {
		db = db.Omit("JobCategoryID")
	}
	return db.Create(position).Error
}

// UpdateJobPosition 募集職種を更新
func (r *CompanyRepository) UpdateJobPosition(position *models.CompanyJobPosition) error {
	db := r.db.Omit("Company", "JobCategory")
	if position.JobCategoryID == 0 {
		// FK: job_category_id=0 は不正。未設定は NULL のまま維持する
		db = db.Omit("JobCategoryID")
	}
	return db.Save(position).Error
}

// FindJobPositionsByCompany 企業の公開済み募集職種を取得（公開ユーザー向け）
func (r *CompanyRepository) FindJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error) {
	var positions []models.CompanyJobPosition
	err := r.db.Where("company_id = ? AND is_active = ? AND data_status = ?", companyID, true, "published").
		Preload("JobCategory").
		Find(&positions).Error
	return positions, err
}

// ListJobPositions 募集職種を一覧取得
func (r *CompanyRepository) ListJobPositions(companyID *uint, limit int) ([]models.CompanyJobPosition, error) {
	if limit <= 0 {
		limit = 50
	}
	var positions []models.CompanyJobPosition
	query := r.db.Preload("JobCategory").Preload("Company")
	if companyID != nil {
		query = query.Where("company_id = ?", *companyID)
	}
	err := query.Order("created_at desc").Limit(limit).Find(&positions).Error
	return positions, err
}

// CreateOrUpdateWeightProfile 重視度プロファイルを作成または更新
func (r *CompanyRepository) CreateOrUpdateWeightProfile(profile *models.CompanyWeightProfile) error {
	var existing models.CompanyWeightProfile
	query := r.db.Where("company_id = ?", profile.CompanyID)

	if profile.JobPositionID != nil {
		query = query.Where("job_position_id = ?", *profile.JobPositionID)
	} else {
		query = query.Where("job_position_id IS NULL")
	}

	err := query.First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		// 新規作成
		return r.db.Create(profile).Error
	} else if err != nil {
		return err
	}

	// 更新（created_atはDBの値を引き継ぐ）
	profile.ID = existing.ID
	profile.CreatedAt = existing.CreatedAt
	return r.db.Save(profile).Error
}

// CountWeightProfiles 企業の重視度プロファイル件数を取得
func (r *CompanyRepository) CountWeightProfiles() (int64, error) {
	var count int64
	err := r.db.Model(&models.CompanyWeightProfile{}).Count(&count).Error
	return count, err
}

// ListPublishedL1WarmCandidates は L1 未充足の公開企業を返す。
func (r *CompanyRepository) ListPublishedL1WarmCandidates(limit int, infoTTL time.Duration) ([]models.CompanyL1WarmRow, error) {
	if limit <= 0 {
		limit = 100
	}
	cutoff := time.Now().Add(-infoTTL)
	var rows []models.CompanyL1WarmRow
	err := r.db.Raw(`
SELECT
  c.*,
  CASE WHEN p.id IS NULL THEN 0 ELSE 1 END AS has_weight_profile
FROM companies c
LEFT JOIN company_weight_profiles p
  ON p.company_id = c.id AND p.job_position_id IS NULL
WHERE c.is_active = ?
  AND c.data_status = ?
  AND (
    c.info_fetched_at IS NULL
    OR c.info_fetched_at < ?
    OR p.id IS NULL
  )
ORDER BY c.id ASC
LIMIT ?
`, true, "published", cutoff, limit).Scan(&rows).Error
	return rows, err
}

// CountL1Coverage は公開カタログの L1 充足統計を返す。
func (r *CompanyRepository) CountL1Coverage(infoTTL time.Duration) (*models.L1CoverageStats, error) {
	cutoff := time.Now().Add(-infoTTL)
	stats := &models.L1CoverageStats{}
	if err := r.db.Model(&models.Company{}).
		Where("is_active = ? AND data_status = ?", true, "published").
		Count(&stats.PublishedTotal).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.Company{}).
		Where("is_active = ? AND data_status = ? AND info_fetched_at IS NOT NULL AND info_fetched_at >= ?", true, "published", cutoff).
		Count(&stats.InfoFresh).Error; err != nil {
		return nil, err
	}
	if err := r.db.Raw(`
SELECT COUNT(DISTINCT c.id)
FROM companies c
INNER JOIN company_weight_profiles p
  ON p.company_id = c.id AND p.job_position_id IS NULL
WHERE c.is_active = ? AND c.data_status = ?
`, true, "published").Scan(&stats.HasProfile).Error; err != nil {
		return nil, err
	}
	if err := r.db.Raw(`
SELECT COUNT(*)
FROM companies c
LEFT JOIN company_weight_profiles p
  ON p.company_id = c.id AND p.job_position_id IS NULL
WHERE c.is_active = ?
  AND c.data_status = ?
  AND (
    c.info_fetched_at IS NULL
    OR c.info_fetched_at < ?
    OR p.id IS NULL
  )
`, true, "published", cutoff).Scan(&stats.NeedsWarm).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

// ListActiveMissingFetchCandidates は info/jobs/tech/relations のいずれかが未取得または TTL 切れのアクティブ企業を返す。
// primaryOnly=true のときは求人不足だけを理由に候補へ入れない（まとめて取得の主3種モード用）。
// 公開企業を優先し、不足埋めバッチ（#633）で使う。
func (r *CompanyRepository) ListActiveMissingFetchCandidates(limit int, primaryOnly bool) ([]models.Company, error) {
	if limit <= 0 {
		limit = 60
	}
	now := time.Now()
	infoCutoff := now.Add(-companyfetch.TTLInfo)
	jobsCutoff := now.Add(-companyfetch.TTLJobs)
	techCutoff := now.Add(-companyfetch.TTLTech)
	relCutoff := now.Add(-companyfetch.TTLRelations)

	techRequiredSQL, techRequiredArgs := companyfields.TechRequiredIndustrySQL("industry")
	primaryCond := `
		info_fetched_at IS NULL OR info_fetched_at < ? OR TRIM(COALESCE(description, '')) = '' OR TRIM(COALESCE(website_url, '')) = ''
		OR (
			(` + techRequiredSQL + `)
			AND (
				tech_fetched_at IS NULL OR tech_fetched_at < ?
				OR TRIM(COALESCE(tech_stack, '')) = ''
				OR TRIM(COALESCE(tech_stack, '')) IN ('[]', 'null', '{}')
			)
		)
		OR relations_fetched_at IS NULL OR relations_fetched_at < ?
		OR (
			NOT EXISTS (
				SELECT 1 FROM company_relations cr
				WHERE cr.deleted_at IS NULL AND cr.is_active = 1 AND (
					cr.parent_id = companies.id OR cr.child_id = companies.id
					OR cr.from_id = companies.id OR cr.to_id = companies.id
				)
			)
			AND NOT EXISTS (
				SELECT 1 FROM company_market_info mi
				WHERE mi.company_id = companies.id AND mi.deleted_at IS NULL
				AND (
					mi.is_listed = 1
					OR TRIM(COALESCE(mi.stock_code, '')) <> ''
					OR LOWER(TRIM(COALESCE(mi.market_type, ''))) IN ('prime', 'standard', 'growth')
				)
			)
		)
	`
	args := []any{infoCutoff}
	args = append(args, techRequiredArgs...)
	args = append(args, techCutoff, relCutoff)
	whereSQL := primaryCond
	if !primaryOnly {
		whereSQL = primaryCond + `
			OR jobs_fetched_at IS NULL OR jobs_fetched_at < ?
			OR NOT EXISTS (
				SELECT 1 FROM company_job_positions jp
				WHERE jp.company_id = companies.id AND jp.deleted_at IS NULL
			)
		`
		args = append(args, jobsCutoff)
	}

	var companies []models.Company
	err := r.db.Where("is_active = ?", true).
		Where(whereSQL, args...).
		Order("CASE WHEN data_status = 'published' THEN 0 ELSE 1 END, id ASC").
		Limit(limit).
		Find(&companies).Error
	return companies, err
}
