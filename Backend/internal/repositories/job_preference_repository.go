package repositories

import "gorm.io/gorm"

// JobPreferenceRepository は、本人が選んだ職種と、その職種の募集がある企業を読む（SOCAIAGENT-670。読み取り専用）。
type JobPreferenceRepository struct {
	db *gorm.DB
}

func NewJobPreferenceRepository(db *gorm.DB) *JobPreferenceRepository {
	return &JobPreferenceRepository{db: db}
}

// SessionJobCategoryID は、セッションで本人が選んだ職種。選んでいなければ 0。
func (r *JobPreferenceRepository) SessionJobCategoryID(sessionID string) (uint, error) {
	return NewConversationContextRepository(r.db).GetJobCategoryID(sessionID)
}

// CompanyIDsHiringCategory は、companyIDs のうち、その職種の募集がある企業。
// 選んだ職種が大分類・小分類のどちらでも当たるよう、同じ職種・その子・その親の募集を対象にする。
func (r *JobPreferenceRepository) CompanyIDsHiringCategory(companyIDs []uint, jobCategoryID uint) (map[uint]bool, error) {
	out := map[uint]bool{}
	if len(companyIDs) == 0 || jobCategoryID == 0 {
		return out, nil
	}
	var ids []uint
	err := r.db.Raw(`SELECT DISTINCT p.company_id
		FROM company_job_positions p
		LEFT JOIN job_categories jc ON jc.id = p.job_category_id
		WHERE p.company_id IN ?
		  AND p.job_category_id > 0
		  AND (p.job_category_id = ?
		       OR jc.parent_id = ?
		       OR p.job_category_id = (SELECT parent_id FROM job_categories WHERE id = ?))`,
		companyIDs, jobCategoryID, jobCategoryID, jobCategoryID).Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
