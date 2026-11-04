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
		OrganizationID:  3, // 既定組織(1)だと DefaultOrganizationID への定数置換を検出できない
		EmailVerifiedAt: &now,
	}
	schools := &orgScopedSchoolRepo{schools: []models.School{
		{ID: 101, Name: "自学園の校", OrganizationID: 3},
		{ID: 202, Name: "他学園の校", OrganizationID: 9},
	}}
	service := NewAuthService(&userRepoAuthStub{user: user}, &pendingRepoAuthStub{}, nil)
	service.SetSchoolRepo(schools)

	if _, err := service.UpdateProfile(UpdateProfileRequest{UserID: 1, SchoolName: "他学園の校"}); err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if schools.lastOrg != 3 {
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

// 学園を渡す経路は3つある（新規登録・ゲスト昇格・プロフィール更新）。
// 登録と昇格で引数を取り違えても落ちるように、渡った学園を固定する。
func TestRegister_PassesTenantOrganizationToSchoolResolution(t *testing.T) {
	schools := &orgScopedSchoolRepo{schools: []models.School{
		{ID: 101, Name: "自学園の校", OrganizationID: 3},
		{ID: 202, Name: "他学園の校", OrganizationID: 9},
	}}
	service := NewAuthService(&userRepoAuthStub{}, &pendingRepoAuthStub{}, nil)
	service.SetSchoolRepo(schools)

	// サブドメインの学園は 3。他学園の校名を入れても解決されない。
	_, _ = service.Register(RegisterRequest{
		Email: "new@example.com", Password: "password123", Name: "新規",
		TargetLevel: "新卒", SchoolName: "他学園の校",
	}, 3, 0)

	if schools.lastOrg != 3 {
		t.Fatalf("登録時にサブドメインの学園が渡っていない: organizationID=%d", schools.lastOrg)
	}
	if schools.lastName != "他学園の校" {
		t.Fatalf("解決対象の学校名が渡っていない: %q", schools.lastName)
	}
}

// ゲスト昇格はゲスト自身の学園で解決する（tenantOrgID ではない）。
// 引数を取り違えると越境が戻るため、渡った学園と既存値の保持を固定する。
func TestRegister_GuestPromotionResolvesWithinGuestOrganization(t *testing.T) {
	now := time.Now()
	existing := uint(101)
	// ゲストはメール未設定（既存ユーザー扱いにならないようにする）。
	guest := &entity.User{
		ID:              5,
		Name:            "ゲスト",
		IsGuest:         true,
		OrganizationID:  3,
		SchoolID:        &existing,
		EmailVerifiedAt: &now,
	}
	schools := &orgScopedSchoolRepo{schools: []models.School{
		{ID: 101, Name: "自学園の校", OrganizationID: 3},
		{ID: 202, Name: "他学園の校", OrganizationID: 9},
	}}
	service := NewAuthService(&userRepoAuthStub{user: guest}, &pendingRepoAuthStub{}, nil)
	service.SetSchoolRepo(schools)

	// サブドメインが学園9でも、昇格はゲストの学園(3)で解決する。
	_, _ = service.Register(RegisterRequest{
		Email: "promoted@example.com", Password: "password123", Name: "昇格後",
		TargetLevel: "新卒", SchoolName: "他学園の校",
	}, 9, guest.ID)

	if schools.lastOrg != 3 {
		t.Fatalf("昇格でゲストの学園が渡っていない: organizationID=%d", schools.lastOrg)
	}
	if guest.SchoolID == nil || *guest.SchoolID != existing {
		t.Fatalf("解決できないときは既存の school_id を保つべき: %v", guest.SchoolID)
	}
}
