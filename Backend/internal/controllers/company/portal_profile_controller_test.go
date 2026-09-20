package company

// 企業ポータルのプロフィール編集・担当者管理のテスト（#1322）。
// 実行: cd Backend && go test ./internal/controllers/ -run CompanyPortalProfile -v
//
// 見たいのは「更新系が owner に限られること」と
// 「自分自身を無効化できないこと」。後者を許すと owner が居なくなり、
// 復旧が運営対応になる。

import (
	"net/http"
	"testing"

	"Backend/internal/models"
	companyauth "Backend/internal/services/companyauth"
	"Backend/internal/services/companyportal"

	"github.com/labstack/echo/v4"
)

type fakeProfileService struct {
	updatedCompanyID uint
	updatedInput     companyportal.ProfileInput
}

func (f *fakeProfileService) Get(companyID uint) (*models.Company, error) {
	return &models.Company{ID: companyID, Name: "テスト株式会社"}, nil
}

func (f *fakeProfileService) Update(companyID uint, in companyportal.ProfileInput) (*models.Company, error) {
	f.updatedCompanyID = companyID
	f.updatedInput = in
	return &models.Company{ID: companyID, Name: "テスト株式会社"}, nil
}

type fakeMemberService struct {
	invitedCompanyID uint
	disabledTarget   uint
	disabledValue    bool
	listed           bool
}

func (f *fakeMemberService) Invite(companyID uint, _ companyauth.InviteRequest) (*models.CompanyUser, error) {
	f.invitedCompanyID = companyID
	return &models.CompanyUser{ID: 20, CompanyID: companyID, Email: "new@example.com"}, nil
}

func (f *fakeMemberService) ListByCompany(uint) ([]models.CompanyUser, error) {
	f.listed = true
	return []models.CompanyUser{}, nil
}

func (f *fakeMemberService) SetDisabled(companyID, companyUserID uint, disabled bool) (*models.CompanyUser, error) {
	f.disabledTarget, f.disabledValue = companyUserID, disabled
	return &models.CompanyUser{ID: companyUserID, CompanyID: companyID}, nil
}

func TestCompanyPortalProfile_更新系はownerのみ(t *testing.T) {
	tests := []struct {
		name string
		call func(ctrl *CompanyPortalProfileController, c echo.Context) error
	}{
		{"プロフィール更新", func(ctrl *CompanyPortalProfileController, c echo.Context) error {
			return ctrl.UpdateCompany(c)
		}},
		{"担当者の招待", func(ctrl *CompanyPortalProfileController, c echo.Context) error {
			return ctrl.InviteMember(c)
		}},
		{"担当者の無効化", func(ctrl *CompanyPortalProfileController, c echo.Context) error {
			c.SetParamNames("userID")
			c.SetParamValues("20")
			return ctrl.SetMemberDisabled(c)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles, members := &fakeProfileService{}, &fakeMemberService{}
			ctrl := NewCompanyPortalProfileController(profiles, members)
			c, _ := portalRequestAs(t, http.MethodPatch, "/company", `{}`, 5, 10, "member")

			err := tt.call(ctrl, c)
			assertAPIStatus(t, err, http.StatusForbidden)

			if profiles.updatedCompanyID != 0 || members.invitedCompanyID != 0 || members.disabledTarget != 0 {
				t.Error("member なのにサービスが呼ばれた")
			}
		})
	}
}

func TestCompanyPortalProfile_参照はmemberもできる(t *testing.T) {
	profiles, members := &fakeProfileService{}, &fakeMemberService{}
	ctrl := NewCompanyPortalProfileController(profiles, members)
	c, _ := portalRequestAs(t, http.MethodGet, "/members", "", 5, 10, "member")

	if err := ctrl.ListMembers(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if !members.listed {
		t.Error("一覧が取得されていない")
	}
}

func TestCompanyPortalProfile_自分自身は無効化できない(t *testing.T) {
	// 許すと owner が居なくなり、復旧が運営対応になる。
	profiles, members := &fakeProfileService{}, &fakeMemberService{}
	ctrl := NewCompanyPortalProfileController(profiles, members)
	c, _ := portalRequestAs(t, http.MethodPatch, "/members/10", `{"disabled":true}`, 5, 10, models.CompanyUserRoleOwner)
	c.SetParamNames("userID")
	c.SetParamValues("10") // 自分自身

	err := ctrl.SetMemberDisabled(c)
	assertAPIStatus(t, err, http.StatusBadRequest)

	if members.disabledTarget != 0 {
		t.Error("自己無効化が実行された")
	}
}

func TestCompanyPortalProfile_他人は無効化できる(t *testing.T) {
	profiles, members := &fakeProfileService{}, &fakeMemberService{}
	ctrl := NewCompanyPortalProfileController(profiles, members)
	c, _ := portalRequestAs(t, http.MethodPatch, "/members/20", `{"disabled":true}`, 5, 10, models.CompanyUserRoleOwner)
	c.SetParamNames("userID")
	c.SetParamValues("20")

	if err := ctrl.SetMemberDisabled(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if members.disabledTarget != 20 || !members.disabledValue {
		t.Errorf("無効化されていない: target=%d value=%v", members.disabledTarget, members.disabledValue)
	}
}

func TestCompanyPortalProfile_JWT由来の企業を使う(t *testing.T) {
	// URL にもボディにも企業を指定させない。JWT 由来だけを使う。
	profiles, members := &fakeProfileService{}, &fakeMemberService{}
	ctrl := NewCompanyPortalProfileController(profiles, members)
	c, _ := portalRequestAs(t, http.MethodPatch, "/company",
		`{"description":"新しい概要","company_id":999}`, 5, 10, models.CompanyUserRoleOwner)

	if err := ctrl.UpdateCompany(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if profiles.updatedCompanyID != 5 {
		t.Errorf("ボディの company_id が使われた: %d", profiles.updatedCompanyID)
	}
	if profiles.updatedInput.Description == nil || *profiles.updatedInput.Description != "新しい概要" {
		t.Error("更新内容が渡っていない")
	}
}

func TestCompanyPortalProfile_未指定の項目はnilで渡る(t *testing.T) {
	// nil で渡らないと、サービス側が「未指定」と「空にする」を区別できない。
	profiles, members := &fakeProfileService{}, &fakeMemberService{}
	ctrl := NewCompanyPortalProfileController(profiles, members)
	c, _ := portalRequestAs(t, http.MethodPatch, "/company",
		`{"description":"概要のみ"}`, 5, 10, models.CompanyUserRoleOwner)

	if err := ctrl.UpdateCompany(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if profiles.updatedInput.Industry != nil {
		t.Errorf("未指定の項目が nil でない: %v", *profiles.updatedInput.Industry)
	}
	if profiles.updatedInput.Location != nil {
		t.Errorf("未指定の項目が nil でない: %v", *profiles.updatedInput.Location)
	}
}
