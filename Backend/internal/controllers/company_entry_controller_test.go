package controllers_test

import (
	"Backend/internal/controllers"
	"Backend/internal/services"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestCompanyEntryController_Submit_ValidationErrors(t *testing.T) {
	svc := &services.CompanyEntryService{}
	ctrl := controllers.NewCompanyEntryController(svc)
	e := echo.New()

	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
	}{
		{
			name:       "name missing",
			body:       map[string]any{"contact_email": "hr@example.com", "privacy_consent": true},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "email missing",
			body:       map[string]any{"name": "テスト株式会社", "privacy_consent": true},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "honeypot accepted as fake success",
			body: map[string]any{
				"name": "テスト株式会社", "contact_email": "hr@example.com",
				"privacy_consent": true, "company_fax": "spam",
			},
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/company-entry", bytes.NewReader(b))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)
			err := ctrl.Submit(ctx)
			if tt.wantStatus >= 400 {
				if err == nil {
					t.Fatalf("expected error status %d", tt.wantStatus)
				}
				he, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("expected HTTPError, got %T %v", err, err)
				}
				if he.Code != tt.wantStatus {
					t.Fatalf("status %d, want %d", he.Code, tt.wantStatus)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
