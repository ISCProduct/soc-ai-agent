package controllers_test

// 残りのコントローラーのHTTPハンドラーテスト (Issue #397)
//
// 実行: cd Backend && go test ./test/controllers/... -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"

	"github.com/stretchr/testify/assert"
)

// ---- AdminCrawlController ----

func TestAdminCrawlController_Sources_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminCrawlController(nil, nil)
	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/crawl/sources", nil)
			w := httptest.NewRecorder()
			c.Sources(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminCrawlController_Runs_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminCrawlController(nil, nil)
	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/crawl/runs", nil)
			w := httptest.NewRecorder()
			c.Runs(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminCrawlController_SourceDetail_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminCrawlController(nil, nil)
	// PUTのみ許可。GET/POST/DELETEは405
	methods := []string{http.MethodGet, http.MethodPost, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			// 正しいパスプレフィックス /api/admin/crawl-sources/ を使用
			req := httptest.NewRequest(method, "/api/admin/crawl-sources/1", nil)
			w := httptest.NewRecorder()
			c.SourceDetail(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- AdminInterviewController ----

func TestAdminInterviewController_ListSessions_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminInterviewController(nil, nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/interview/sessions", nil)
			w := httptest.NewRecorder()
			c.ListSessions(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminInterviewController_ListVideos_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminInterviewController(nil, nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/interview/videos", nil)
			w := httptest.NewRecorder()
			c.ListVideos(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAdminInterviewController_VideoURL_MethodNotAllowed(t *testing.T) {
	c := controllers.NewAdminInterviewController(nil, nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/admin/interview/videos/1/url", nil)
			w := httptest.NewRecorder()
			c.VideoURL(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- QuestionController ----

func TestQuestionController_GenerateQuestions_MethodNotAllowed(t *testing.T) {
	c := controllers.NewQuestionController(nil)
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/questions/generate", nil)
			w := httptest.NewRecorder()
			c.GenerateQuestions(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestQuestionController_CreateQuestion_MethodNotAllowed(t *testing.T) {
	c := controllers.NewQuestionController(nil)
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/questions", nil)
			w := httptest.NewRecorder()
			c.CreateQuestion(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestQuestionController_GetQuestionsByCategory_MethodNotAllowed(t *testing.T) {
	c := controllers.NewQuestionController(nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/questions", nil)
			w := httptest.NewRecorder()
			c.GetQuestionsByCategory(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- ScheduleController ----

func TestScheduleController_List_MethodNotAllowed(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/schedules", nil)
			w := httptest.NewRecorder()
			c.List(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestScheduleController_Create_MethodNotAllowed(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/schedules", nil)
			w := httptest.NewRecorder()
			c.Create(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestScheduleController_Get_MethodNotAllowed(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/schedules/1", nil)
			w := httptest.NewRecorder()
			c.Get(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestScheduleController_Update_MethodNotAllowed(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	methods := []string{http.MethodGet, http.MethodPost, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/schedules/1", nil)
			w := httptest.NewRecorder()
			c.Update(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestScheduleController_Delete_MethodNotAllowed(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/schedules/1", nil)
			w := httptest.NewRecorder()
			c.Delete(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestScheduleController_ExportICS_MethodNotAllowed(t *testing.T) {
	c := controllers.NewScheduleController(nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/schedules/export.ics", nil)
			w := httptest.NewRecorder()
			c.ExportICS(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- CompanyEntryController ----

func TestCompanyEntryController_Submit_MethodNotAllowed(t *testing.T) {
	c := controllers.NewCompanyEntryController(nil, nil)
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/companies/entry", nil)
			w := httptest.NewRecorder()
			c.Submit(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- CompanyRelationController ----

// TestCompanyRelationController_GetCompanyByID_MethodNotAllowed はGET以外に405を返すことを検証
func TestCompanyRelationController_GetCompanyByID_MethodNotAllowed(t *testing.T) {
	c := controllers.NewCompanyRelationController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/companies/1", nil)
			w := httptest.NewRecorder()
			c.GetCompanyByID(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestCompanyRelationController_GetCompanyJobPositions_MethodNotAllowed はGET以外に405を返すことを検証
func TestCompanyRelationController_GetCompanyJobPositions_MethodNotAllowed(t *testing.T) {
	c := controllers.NewCompanyRelationController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/companies/1/job-positions", nil)
			w := httptest.NewRecorder()
			c.GetCompanyJobPositions(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestCompanyRelationController_WebSearchCompanies_MethodNotAllowed はGET以外に405を返すことを検証
func TestCompanyRelationController_WebSearchCompanies_MethodNotAllowed(t *testing.T) {
	c := controllers.NewCompanyRelationController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/companies/search", nil)
			w := httptest.NewRecorder()
			c.WebSearchCompanies(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- ESReviewController ----

func TestESReviewController_Review_MethodNotAllowed(t *testing.T) {
	c := controllers.NewESReviewController()
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/es/review", nil)
			w := httptest.NewRecorder()
			c.Review(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- ESRewriteController ----

func TestESRewriteController_Rewrite_MethodNotAllowed(t *testing.T) {
	c := controllers.NewESRewriteController(nil)
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/es/rewrite", nil)
			w := httptest.NewRecorder()
			c.Rewrite(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// ---- GitHubController ----

func TestGitHubController_GetProfile_MethodNotAllowed(t *testing.T) {
	c := controllers.NewGitHubController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/github/profile", nil)
			req = withUserID(req, 1)
			w := httptest.NewRecorder()
			c.GetProfile(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestGitHubController_Sync_MethodNotAllowed(t *testing.T) {
	c := controllers.NewGitHubController(nil, nil)
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/github/sync", nil)
			req = withUserID(req, 1)
			w := httptest.NewRecorder()
			c.Sync(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestGitHubController_GetSkills_MethodNotAllowed(t *testing.T) {
	c := controllers.NewGitHubController(nil, nil)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/github/skills", nil)
			req = withUserID(req, 1)
			w := httptest.NewRecorder()
			c.GetSkills(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}
