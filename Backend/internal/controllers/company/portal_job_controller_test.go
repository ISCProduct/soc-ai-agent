package company

// 企業ポータル求人管理のロール境界テスト（#1321）。
// 実行: cd Backend && go test ./internal/controllers/ -run CompanyPortalJob -v
//
// 一覧は member も見られるが、作成・編集・公開は owner のみ。
// サーバー側で弾けていないと、UIを直接叩かれたときに通ってしまう。

import (
	"encoding/json"
	"net/http"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/companyportal"
)

type fakePortalJobService struct {
	createdCompanyID uint
	updatedID        uint
	publishedID      uint
	publishedValue   bool
	listCalled       bool
	companyPublished bool
}

func (f *fakePortalJobService) List(uint) ([]models.CompanyJobPosition, error) {
	f.listCalled = true
	return []models.CompanyJobPosition{}, nil
}

func (f *fakePortalJobService) Create(companyID uint, _ companyportal.JobInput) (*models.CompanyJobPosition, error) {
	f.createdCompanyID = companyID
	return &models.CompanyJobPosition{ID: 1, CompanyID: companyID, DataStatus: companyportal.JobStatusDraft}, nil
}

func (f *fakePortalJobService) Update(jobID, companyID uint, _ companyportal.JobInput) (*models.CompanyJobPosition, error) {
	f.updatedID = jobID
	return &models.CompanyJobPosition{ID: jobID, CompanyID: companyID}, nil
}

func (f *fakePortalJobService) SetPublished(jobID, companyID uint, published bool) (*companyportal.JobVisibility, error) {
	f.publishedID, f.publishedValue = jobID, published
	status := companyportal.JobStatusDraft
	if published {
		status = companyportal.JobStatusPublished
	}
	return &companyportal.JobVisibility{
		Job:              &models.CompanyJobPosition{ID: jobID, CompanyID: companyID, DataStatus: status},
		CompanyPublished: f.companyPublished,
	}, nil
}

func (f *fakePortalJobService) CompanyPublished(uint) bool { return f.companyPublished }

func TestCompanyPortalJob_List_memberも見られる(t *testing.T) {
	svc := &fakePortalJobService{}
	ctrl := NewCompanyPortalJobController(svc)

	c, rec := portalRequest(t, http.MethodGet, "/jobs", "", 5, "member")
	if err := ctrl.List(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !svc.listCalled {
		t.Error("一覧が取得されていない")
	}
}

func TestCompanyPortalJob_破壊的操作はownerのみ(t *testing.T) {
	tests := []struct {
		name string
		call func(ctrl *CompanyPortalJobController) error
	}{
		{"作成", func(ctrl *CompanyPortalJobController) error {
			c, _ := portalRequest(t, http.MethodPost, "/jobs", `{"title":"エンジニア"}`, 5, "member")
			return ctrl.Create(c)
		}},
		{"編集", func(ctrl *CompanyPortalJobController) error {
			c, _ := portalRequest(t, http.MethodPatch, "/jobs/1", `{"title":"エンジニア"}`, 5, "member")
			c.SetParamNames("id")
			c.SetParamValues("1")
			return ctrl.Update(c)
		}},
		{"公開", func(ctrl *CompanyPortalJobController) error {
			c, _ := portalRequest(t, http.MethodPost, "/jobs/1/publish", `{"published":true}`, 5, "member")
			c.SetParamNames("id")
			c.SetParamValues("1")
			return ctrl.Publish(c)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePortalJobService{}
			ctrl := NewCompanyPortalJobController(svc)

			err := tt.call(ctrl)
			assertAPIStatus(t, err, http.StatusForbidden)

			// 権限が無いなら、サービスまで到達してはいけない。
			if svc.createdCompanyID != 0 || svc.updatedID != 0 || svc.publishedID != 0 {
				t.Error("member なのにサービスが呼ばれた")
			}
		})
	}
}

func TestCompanyPortalJob_Create_JWT由来の企業で作る(t *testing.T) {
	svc := &fakePortalJobService{}
	ctrl := NewCompanyPortalJobController(svc)

	// ボディに company_id を入れても無視されること
	c, rec := portalRequest(t, http.MethodPost, "/jobs",
		`{"title":"エンジニア","company_id":999}`, 5, models.CompanyUserRoleOwner)
	if err := ctrl.Create(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.createdCompanyID != 5 {
		t.Errorf("ボディの company_id が使われた: %d", svc.createdCompanyID)
	}
}

func TestCompanyPortalJob_Publish_既定は公開(t *testing.T) {
	// published を省略したときに非公開になると、公開ボタンが無反応に見える。
	svc := &fakePortalJobService{}
	ctrl := NewCompanyPortalJobController(svc)

	c, _ := portalRequest(t, http.MethodPost, "/jobs/3/publish", `{}`, 5, models.CompanyUserRoleOwner)
	c.SetParamNames("id")
	c.SetParamValues("3")

	if err := ctrl.Publish(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if svc.publishedID != 3 || !svc.publishedValue {
		t.Errorf("既定が公開になっていない: id=%d published=%v", svc.publishedID, svc.publishedValue)
	}
}

func TestCompanyPortalJob_Publish_明示的に非公開にできる(t *testing.T) {
	svc := &fakePortalJobService{}
	ctrl := NewCompanyPortalJobController(svc)

	c, _ := portalRequest(t, http.MethodPost, "/jobs/3/publish", `{"published":false}`, 5, models.CompanyUserRoleOwner)
	c.SetParamNames("id")
	c.SetParamValues("3")

	if err := ctrl.Publish(c); err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if svc.publishedValue {
		t.Error("非公開の指定が効いていない")
	}
}

func TestCompanyPortalJob_未認証は401(t *testing.T) {
	ctrl := NewCompanyPortalJobController(&fakePortalJobService{})
	c, _ := portalRequest(t, http.MethodGet, "/jobs", "", 0, "")

	err := ctrl.List(c)
	assertAPIStatus(t, err, http.StatusUnauthorized)
}

// 企業本体が未公開のとき、公開は通すが「学生に見えない」ことを返す。
//
// 現状 842社すべてが draft / provisional のため、ここで拒否すると
// 求人機能がまったく使えない。代わりに UI が警告を出せる情報を返す(#1321)。
func TestCompanyPortalJob_Publish_企業未公開なら見えないことを返す(t *testing.T) {
	svc := &fakePortalJobService{companyPublished: false}
	ctrl := NewCompanyPortalJobController(svc)

	c, rec := portalRequest(t, http.MethodPost, "/jobs/3/publish", `{"published":true}`, 5, models.CompanyUserRoleOwner)
	c.SetParamNames("id")
	c.SetParamValues("3")

	if err := ctrl.Publish(c); err != nil {
		t.Fatalf("公開は通すべき: %v", err)
	}

	var got struct {
		Job              map[string]any `json:"job"`
		CompanyPublished bool           `json:"company_published"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSONを読めない: %v (%s)", err, rec.Body.String())
	}
	if got.CompanyPublished {
		t.Error("企業が未公開なのに公開済みとして返している")
	}
	if got.Job["data_status"] != companyportal.JobStatusPublished {
		t.Errorf("求人が公開になっていない: %v", got.Job["data_status"])
	}
}
