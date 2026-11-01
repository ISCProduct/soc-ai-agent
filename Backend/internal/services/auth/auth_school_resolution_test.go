package auth

import (
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/models"
)

// orgScopedSchoolRepo は学園で絞って学校名を引く最小実装。
// 本番の SchoolRepository.FindByName と同じ契約（organization_id 必須、0 は既定組織）。
type orgScopedSchoolRepo struct {
	schools  []models.School
	lastOrg  uint
	lastName string
}

func (r *orgScopedSchoolRepo) FindByName(name string, organizationID uint) (*models.School, error) {
	r.lastName, r.lastOrg = name, organizationID
	if organizationID == 0 {
		organizationID = models.DefaultOrganizationID
	}
	for i := range r.schools {
		if r.schools[i].Name == name && r.schools[i].OrganizationID == organizationID {
			return &r.schools[i], nil
		}
	}
	return nil, nil
}

func (r *orgScopedSchoolRepo) Create(*models.School) error                   { return nil }
func (r *orgScopedSchoolRepo) Update(*models.School) error                   { return nil }
func (r *orgScopedSchoolRepo) FindByID(uint) (*models.School, error)         { return nil, nil }
func (r *orgScopedSchoolRepo) List(int, int) ([]models.School, int64, error) { return nil, 0, nil }
func (r *orgScopedSchoolRepo) AddMember(*models.AdminSchoolMembership) error { return nil }
func (r *orgScopedSchoolRepo) RemoveMember(uint, uint) error                 { return nil }
func (r *orgScopedSchoolRepo) ListSchoolsForAdmin(uint) ([]models.School, error) {
	return nil, nil
}
func (r *orgScopedSchoolRepo) AddCompanyApproval(*models.SchoolCompanyApproval) error { return nil }
func (r *orgScopedSchoolRepo) RemoveCompanyApproval(uint, uint) error                 { return nil }
func (r *orgScopedSchoolRepo) IsCompanyApproved(uint, uint) (bool, error)             { return false, nil }
func (r *orgScopedSchoolRepo) ListApprovedCompanyIDs(uint) ([]uint, error)            { return nil, nil }

// 学生がプロフィールの学校名を他学園の校名に書き替えても、その学校の school_id は付かない。
// 付いてしまうと、先生向けの学生一覧など users.school_id を信頼する経路に他学園の学生が
// 混ざる（#1675 のテナント越境）。学園を跨いだ解決は許さない。
func TestUpdateProfile_SchoolNameDoesNotResolveAcrossOrganizations(t *testing.T) {
	now := time.Now()
	user := &entity.User{
		ID:              1,
		Email:           "student@example.com",
		Name:            "学生",
		OrganizationID:  1, // 自分は学園1
		EmailVerifiedAt: &now,
	}
	schools := &orgScopedSchoolRepo{schools: []models.School{
		{ID: 101, Name: "自学園の校", OrganizationID: 1},
		{ID: 202, Name: "他学園の校", OrganizationID: 9},
	}}
	service := NewAuthService(&userRepoAuthStub{user: user}, &pendingRepoAuthStub{}, nil)
	service.SetSchoolRepo(schools)

	if _, err := service.UpdateProfile(UpdateProfileRequest{UserID: 1, SchoolName: "他学園の校"}); err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if schools.lastOrg != 1 {
		t.Fatalf("解決時に本人の学園が渡っていない: organizationID=%d", schools.lastOrg)
	}
	if user.SchoolID != nil {
		t.Fatalf("別学園の校名が解決されてしまった: school_id=%d", *user.SchoolID)
	}

	// 自学園の校名は従来どおり解決できる（過度に閉じて既存機能を壊していないこと）。
	if _, err := service.UpdateProfile(UpdateProfileRequest{UserID: 1, SchoolName: "自学園の校"}); err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if user.SchoolID == nil || *user.SchoolID != 101 {
		t.Fatalf("自学園の校名が解決されていない: %v", user.SchoolID)
	}
}
