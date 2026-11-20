package repositories

import (
	"context"
	"errors"
	"strings"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// CompanySearchRepository は学生向けの企業検索（SOCAIAGENT-612 / #2024）。
//
// 学生の企業探しは AI の推薦が中心で、自分の条件で探す導線が無かった。業界・地域（都道府県）・職種・
// 名前で絞り込める中立な検索を出す。並びは企業名順（推薦や課金で上位に固定しない）。
// 表示する企業は既存の企業一覧（GET /api/companies）と同じく、有効で、審査前のゲスト投稿でないもの。
type CompanySearchRepository struct {
	db *gorm.DB
}

func NewCompanySearchRepository(db *gorm.DB) *CompanySearchRepository {
	return &CompanySearchRepository{db: db}
}

// Prefectures は地域の絞り込みで使う都道府県。companies.location（自由記述の所在地）に名前が含まれるかで絞る。
// 一覧の外の値は受け付けない（LIKE に任意の文字列を渡さない）。
var Prefectures = []string{
	"北海道", "青森県", "岩手県", "宮城県", "秋田県", "山形県", "福島県",
	"茨城県", "栃木県", "群馬県", "埼玉県", "千葉県", "東京都", "神奈川県",
	"新潟県", "富山県", "石川県", "福井県", "山梨県", "長野県", "岐阜県", "静岡県", "愛知県",
	"三重県", "滋賀県", "京都府", "大阪府", "兵庫県", "奈良県", "和歌山県",
	"鳥取県", "島根県", "岡山県", "広島県", "山口県",
	"徳島県", "香川県", "愛媛県", "高知県",
	"福岡県", "佐賀県", "長崎県", "熊本県", "大分県", "宮崎県", "鹿児島県", "沖縄県",
}

// IsPrefecture は都道府県の一覧にある名前か。
func IsPrefecture(name string) bool {
	for _, p := range Prefectures {
		if p == name {
			return true
		}
	}
	return false
}

// CompanySearchFilter は検索の条件。空・0 は絞らない。
type CompanySearchFilter struct {
	Name          string
	Industry      string
	Prefecture    string
	JobCategoryID uint
	Limit         int
	Offset        int
}

// CompanySearchRow は検索結果の1社（学生に見せる項目だけ）。
type CompanySearchRow struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Industry    string `json:"industry"`
	Location    string `json:"location"`
	Description string `json:"description"`
	LogoURL     string `json:"logo_url"`
	// 出どころ（詳細画面と同じ判定で「AI推定」を出すため。AI 補完の値を確認済みの事実に見せない）
	SourceType          string `json:"source_type"`
	SourceURL           string `json:"source_url"`
	LastFetchConfidence string `json:"last_fetch_confidence"`
	LastModelUsed       string `json:"last_model_used"`
}

// visibleCompanyForSearch は学生に見せてよい企業（applyCompanyFilters と同じ判定）。
func (r *CompanySearchRepository) visible(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&models.Company{}).
		Where("companies.is_active = ? AND companies.deleted_at IS NULL", true).
		Where(guestEntryVisibilityGuard("companies.id"))
}

// jobCategoryCondition は、その職種（または子の職種）の公開中の求人を持つ企業に絞る条件。
// 企業本体が公開中であることも求める。下書きの企業の求人は、学生向けの求人一覧
// （publishedCompanyExists）と同じく根拠に数えない。
var jobCategoryCondition = `EXISTS (
	SELECT 1 FROM company_job_positions jp
	JOIN job_categories jc ON jc.id = jp.job_category_id
	WHERE jp.company_id = companies.id
	  AND jp.is_active = true AND jp.data_status = 'published' AND jp.deleted_at IS NULL
	  AND ` + publishedCompanyExists("jp.company_id") + `
	  AND (jc.id = ? OR jc.parent_id = ?)
)`

// Search は条件に合う企業を企業名順に返す（総件数つき）。
func (r *CompanySearchRepository) Search(ctx context.Context, f CompanySearchFilter) ([]CompanySearchRow, int64, error) {
	q := r.visible(ctx)
	if name := strings.TrimSpace(f.Name); name != "" {
		q = q.Where("companies.name LIKE ?", "%"+escapeLike(name)+"%")
	}
	if f.Industry != "" {
		q = q.Where("companies.industry = ?", f.Industry)
	}
	if f.Prefecture != "" {
		q = q.Where("companies.location LIKE ?", "%"+f.Prefecture+"%")
	}
	if f.JobCategoryID > 0 {
		q = q.Where(jobCategoryCondition, f.JobCategoryID, f.JobCategoryID)
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []CompanySearchRow
	err := q.Session(&gorm.Session{}).
		Select("companies.id, companies.name, COALESCE(companies.industry, '') AS industry, COALESCE(companies.location, '') AS location, COALESCE(companies.description, '') AS description, COALESCE(companies.logo_url, '') AS logo_url, " +
			"COALESCE(companies.source_type, '') AS source_type, COALESCE(companies.source_url, '') AS source_url, " +
			"COALESCE(companies.last_fetch_confidence, '') AS last_fetch_confidence, COALESCE(companies.last_model_used, '') AS last_model_used").
		Order("companies.name ASC, companies.id ASC").
		Limit(f.Limit).Offset(f.Offset).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// JobCategoryOption は職種の絞り込みの選択肢。
type JobCategoryOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// SearchOptions は絞り込みの選択肢を返す。業界は学生に見せる企業に付いているもの、
// 職種は公開中の企業の公開中の求人がある職種（その職種か、parent_id で子にあたる職種の求人）。
// 標準のシードは小分類（Level 1）に parent_id を設定していないため、大分類だけに絞ると
// 小分類の求人しか無い企業を選べなくなる。そのため階層で絞らず、求人のある職種をすべて出す。
func (r *CompanySearchRepository) SearchOptions(ctx context.Context) ([]string, []JobCategoryOption, error) {
	var industries []string
	if err := r.visible(ctx).Where("companies.industry IS NOT NULL AND companies.industry <> ''").
		Distinct().Order("companies.industry ASC").Pluck("companies.industry", &industries).Error; err != nil {
		return nil, nil, err
	}
	var cats []JobCategoryOption
	err := r.db.WithContext(ctx).Table("job_categories top").
		Select("top.id AS id, top.name AS name").
		Where("top.is_active = ?", true).
		Where(`EXISTS (
			SELECT 1 FROM company_job_positions jp
			JOIN job_categories jc ON jc.id = jp.job_category_id
			JOIN companies ON companies.id = jp.company_id
			WHERE (jc.id = top.id OR jc.parent_id = top.id)
			  AND jp.is_active = true AND jp.data_status = 'published' AND jp.deleted_at IS NULL
			  AND companies.is_active = true AND companies.deleted_at IS NULL
			  AND companies.data_status = 'published'
			  AND ` + guestEntryVisibilityGuard("companies.id") + `
		)`).
		Order("top.level ASC, top.display_order ASC, top.id ASC").
		Scan(&cats).Error
	if err != nil {
		return nil, nil, err
	}
	return industries, cats, nil
}

// FavoriteState は、その企業が本人の最新の診断の推薦にあるときの推薦 ID とお気に入りの状態。
type FavoriteState struct {
	CompanyID   uint `json:"company_id"`
	MatchID     uint `json:"match_id"`
	IsFavorited bool `json:"is_favorited"`
}

// FavoriteStates は、本人の最新の診断（面接スナップショット・適性検査を除くチャット診断）の推薦のうち、
// 指定した企業のものを返す。お気に入りは推薦（user_company_matches）に付くので、推薦に無い企業は含まれない。
// 診断がまだ無ければ空。
func (r *CompanySearchRepository) FavoriteStates(ctx context.Context, userID uint, companyIDs []uint) ([]FavoriteState, error) {
	out := []FavoriteState{}
	if len(companyIDs) == 0 {
		return out, nil
	}
	var latest models.UserWeightScore
	err := r.db.WithContext(ctx).Where("user_id = ? AND session_id NOT LIKE ? AND source <> ?", userID, "interview-%", models.ScoreSourceAssessment).
		Order("updated_at DESC, id DESC").Select("session_id").First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	err = r.db.WithContext(ctx).Model(&models.UserCompanyMatch{}).
		Select("company_id, id AS match_id, is_favorited").
		Where("user_id = ? AND session_id = ? AND company_id IN ?", userID, latest.SessionID, companyIDs).
		Scan(&out).Error
	return out, err
}
