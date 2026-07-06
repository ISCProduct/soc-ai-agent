package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func makeWebSearchServer(t *testing.T, responseText string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		resp := map[string]any{
			"output_text": responseText,
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func validCompanyInfoJSON() string {
	return `{"description":"テスト企業の概要","industry":"IT・ソフトウェア","location":"東京都渋谷区","website_url":"https://example.com","founded_year":2010,"employee_count":500,"main_business":"クラウドサービスの開発","culture":"フラットな組織文化","work_style":"ハイブリッド"}`
}

func TestCompanyInfoFetcher_FetchAndSave(t *testing.T) {
	baseCompany := func() *models.Company {
		return &models.Company{
			ID:          1,
			Name:        "テスト株式会社",
			Description: "旧概要",
			Industry:    "製造業",
		}
	}

	tests := []struct {
		name           string
		setupMock      func(*mocks.CompanyRepositoryMock)
		serverResponse string
		serverStatus   int
		wantErr        bool
		wantErrContain string
		checkResult    func(*testing.T, *services.CompanyInfoResult)
	}{
		{
			name: "正常系: 全フィールドが更新される",
			setupMock: func(m *mocks.CompanyRepositoryMock) {
				m.On("FindByID", uint(1)).Return(baseCompany(), nil)
				m.On("Update", mock.AnythingOfType("*models.Company")).Return(nil)
			},
			serverResponse: validCompanyInfoJSON(),
			serverStatus:   http.StatusOK,
			wantErr:        false,
			checkResult: func(t *testing.T, r *services.CompanyInfoResult) {
				assert.Equal(t, "テスト企業の概要", r.Description)
				assert.Equal(t, "IT・ソフトウェア", r.Industry)
				assert.Equal(t, "東京都渋谷区", r.Location)
				assert.Equal(t, "https://example.com", r.WebsiteURL)
				assert.Equal(t, 2010, r.FoundedYear)
				assert.Equal(t, 500, r.EmployeeCount)
				assert.Equal(t, "クラウドサービスの開発", r.MainBusiness)
				assert.Equal(t, "フラットな組織文化", r.Culture)
				assert.Equal(t, "ハイブリッド", r.WorkStyle)
			},
		},
		{
			name: "正常系: レスポンスに余分なテキストが含まれていてもJSONを抽出できる",
			setupMock: func(m *mocks.CompanyRepositoryMock) {
				m.On("FindByID", uint(1)).Return(baseCompany(), nil)
				m.On("Update", mock.AnythingOfType("*models.Company")).Return(nil)
			},
			serverResponse: "こちらが企業情報です：\n" + validCompanyInfoJSON() + "\n以上です。",
			serverStatus:   http.StatusOK,
			wantErr:        false,
			checkResult: func(t *testing.T, r *services.CompanyInfoResult) {
				assert.Equal(t, "テスト企業の概要", r.Description)
				assert.Equal(t, "IT・ソフトウェア", r.Industry)
			},
		},
		{
			name: "正常系: 空文字フィールドは既存データを上書きしない",
			setupMock: func(m *mocks.CompanyRepositoryMock) {
				c := baseCompany()
				c.Industry = "既存業種"
				m.On("FindByID", uint(1)).Return(c, nil)
				m.On("Update", mock.AnythingOfType("*models.Company")).Return(nil)
			},
			serverResponse: `{"description":"新概要","industry":"","location":"","website_url":"","founded_year":0,"employee_count":0,"main_business":"","culture":"","work_style":""}`,
			serverStatus:   http.StatusOK,
			wantErr:        false,
			checkResult: func(t *testing.T, r *services.CompanyInfoResult) {
				assert.Equal(t, "新概要", r.Description)
				assert.Equal(t, "", r.Industry)
			},
		},
		{
			name: "異常系: 企業が見つからない場合はエラー",
			setupMock: func(m *mocks.CompanyRepositoryMock) {
				m.On("FindByID", uint(99)).Return(nil, errors.New("record not found"))
			},
			serverResponse: validCompanyInfoJSON(),
			serverStatus:   http.StatusOK,
			wantErr:        true,
			wantErrContain: "company not found",
		},
		{
			name: "異常系: WebSearchが不正なJSONを返した場合はエラー",
			setupMock: func(m *mocks.CompanyRepositoryMock) {
				m.On("FindByID", uint(1)).Return(baseCompany(), nil)
			},
			serverResponse: "JSONではないテキスト",
			serverStatus:   http.StatusOK,
			wantErr:        true,
			wantErrContain: "failed to parse web search response",
		},
		{
			name: "異常系: DB更新が失敗した場合はエラー",
			setupMock: func(m *mocks.CompanyRepositoryMock) {
				m.On("FindByID", uint(1)).Return(baseCompany(), nil)
				m.On("Update", mock.AnythingOfType("*models.Company")).Return(errors.New("db error"))
			},
			serverResponse: validCompanyInfoJSON(),
			serverStatus:   http.StatusOK,
			wantErr:        true,
			wantErrContain: "failed to update company",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := makeWebSearchServer(t, tc.serverResponse, tc.serverStatus)
			defer srv.Close()

			repo := &mocks.CompanyRepositoryMock{}
			tc.setupMock(repo)

			client := openai.NewWithBaseURL(srv.URL, "gpt-4o-search-preview")
			fetcher := services.NewCompanyInfoFetcher(repo, client)

			companyID := uint(1)
			if tc.wantErrContain == "company not found" {
				companyID = 99
			}

			result, err := fetcher.FetchAndSave(context.Background(), companyID, true)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrContain != "" {
					assert.Contains(t, err.Error(), tc.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			if tc.checkResult != nil {
				tc.checkResult(t, result)
			}
		})
	}
}
