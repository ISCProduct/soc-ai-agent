package repositories

import (
	"errors"

	"Backend/internal/models"
	"Backend/internal/util"

	"gorm.io/gorm"
)

type SchoolRepository struct {
	db *gorm.DB
}

func NewSchoolRepository(db *gorm.DB) *SchoolRepository {
	return &SchoolRepository{db: db}
}

func (r *SchoolRepository) Create(school *models.School) error {
	return r.db.Create(school).Error
}

func (r *SchoolRepository) Update(school *models.School) error {
	return r.db.Save(school).Error
}

func (r *SchoolRepository) FindByID(id uint) (*models.School, error) {
	var school models.School
	if err := r.db.First(&school, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &school, nil
}

// FindByName は学園内の個別校を名前で引く。
//
// organizationID は必須。学校名は本人の自己申告（新規登録・プロフィール更新）から来るため、
// 学園で絞らないと他学園の school_id を自分に付けられる（#1675 のテナント越境）。
// 0 は既定組織として扱う。users の保存側（user_repository.go の CreateUser）と同じ正規化に
// しておかないと、解決した学校と実際に保存される organization_id が食い違う。
func (r *SchoolRepository) FindByName(name string, organizationID uint) (*models.School, error) {
	if organizationID == 0 {
		organizationID = models.DefaultOrganizationID
	}
	var school models.School
	if err := r.db.Where("organization_id = ? AND name = ?", organizationID, name).First(&school).Error; err == nil {
		return &school, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	normalized := util.NormalizeSchoolName(name)
	if normalized == "" {
		return nil, nil
	}

	var schools []models.School
	if err := r.db.Where("organization_id = ? AND status = ?", organizationID, models.SchoolStatusActive).Find(&schools).Error; err != nil {
		return nil, err
	}
	for i := range schools {
		if util.NormalizeSchoolName(schools[i].Name) == normalized {
			return &schools[i], nil
		}
	}
	return nil, nil
}

func (r *SchoolRepository) List(limit, offset int) ([]models.School, int64, error) {
	if limit <= 0 {
		limit = 25
	}
	var total int64
	if err := r.db.Model(&models.School{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var schools []models.School
	if err := r.db.Order("id ASC").Limit(limit).Offset(offset).Find(&schools).Error; err != nil {
		return nil, 0, err
	}
	return schools, total, nil
}

func (r *SchoolRepository) AddMember(m *models.AdminSchoolMembership) error {
	return r.db.Create(m).Error
}

func (r *SchoolRepository) RemoveMember(userID, schoolID uint) error {
	res := r.db.Where("user_id = ? AND school_id = ?", userID, schoolID).Delete(&models.AdminSchoolMembership{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListSchoolsForAdmin は管理者・職員の担当校。自分の学園の学校だけを返す（#1675）。
//
// 認可には脚が2本ある。「対象ユーザーの学校が本人の学園のものか」と
// 「呼び出し元の担当校が自分の学園のものか」。前者だけ閉じても、越境した
// admin_school_memberships の行が1つあれば他学園の学校が担当校として通り、
// その学校の学生は（学校と学生の学園が揃っているので）すべての判定を通過してしまう。
//
// ここは ResolveAccess / CanAdminAccessSchool / EchoAdminSchoolScope /
// EnsureAdminSchoolAccessForUser の共通の土台なので、1箇所で全経路が閉じる。
// 割当側（AddMember）のチェックは「これから作る行」しか守らないため、
// 読み取り側でも学園一致を要求してデータの綺麗さに依存させない。
func (r *SchoolRepository) ListSchoolsForAdmin(userID uint) ([]models.School, error) {
	var schools []models.School
	err := r.db.Joins("JOIN admin_school_memberships ON admin_school_memberships.school_id = schools.id").
		Where("admin_school_memberships.user_id = ?", userID).
		Where("schools.organization_id = (SELECT organization_id FROM users WHERE id = ?)", userID).
		Order("schools.id ASC").
		Find(&schools).Error
	if err != nil {
		return nil, err
	}
	return schools, nil
}

func (r *SchoolRepository) AddCompanyApproval(a *models.SchoolCompanyApproval) error {
	return r.db.Create(a).Error
}

func (r *SchoolRepository) RemoveCompanyApproval(schoolID, companyID uint) error {
	res := r.db.Where("school_id = ? AND company_id = ?", schoolID, companyID).Delete(&models.SchoolCompanyApproval{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *SchoolRepository) IsCompanyApproved(schoolID, companyID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.SchoolCompanyApproval{}).
		Where("school_id = ? AND company_id = ?", schoolID, companyID).
		Count(&count).Error
	return count > 0, err
}

func (r *SchoolRepository) ListApprovedCompanyIDs(schoolID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&models.SchoolCompanyApproval{}).
		Where("school_id = ?", schoolID).
		Pluck("company_id", &ids).Error
	return ids, err
}
