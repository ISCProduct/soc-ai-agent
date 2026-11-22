package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	authsvc "Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

type fakeOAuthService struct{}

func (fakeOAuthService) GetGoogleAuthURL(state string) string {
	return "https://accounts.example/?state=" + state
}
func (fakeOAuthService) GetGitHubAuthURL(state string) string {
	return "https://github.example/?state=" + state
}
func (fakeOAuthService) HandleGoogleCallback(context.Context, string, uint) (*authsvc.AuthResponse, error) {
	return &authsvc.AuthResponse{UserID: 7, UserToken: "user-jwt", RefreshToken: "refresh"}, nil
}
func (fakeOAuthService) HandleGitHubCallback(context.Context, string, uint) (*authsvc.AuthResponse, error) {
	return &authsvc.AuthResponse{UserID: 7, UserToken: "user-jwt", RefreshToken: "refresh"}, nil
}

const testVerifier = "verifier-made-in-this-browser"

// googleCallback は開始（bind 付き）→ プロバイダから戻る、を1往復して 302 の Location を返す。
func googleCallback(t *testing.T, c *OAuthController, bind string) *url.URL {
	t.Helper()
	e := echo.New()

	startReq := httptest.NewRequest(http.MethodGet, "/api/auth/google?bind="+url.QueryEscape(bind), nil)
	startRec := httptest.NewRecorder()
	if err := c.GoogleLogin(e.NewContext(startReq, startRec)); err != nil {
		t.Fatal(err)
	}
	loc, _ := url.Parse(startRec.Header().Get("Location"))

	cbReq := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=provider-code&state="+url.QueryEscape(loc.Query().Get("state")), nil)
	for _, ck := range startRec.Result().Cookies() {
		cbReq.AddCookie(ck)
	}
	cbRec := httptest.NewRecorder()
	if err := c.GoogleCallback(e.NewContext(cbReq, cbRec)); err != nil {
		t.Fatal(err)
	}
	if cbRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d", cbRec.Code)
	}
	got, _ := url.Parse(cbRec.Header().Get("Location"))
	return got
}

func TestOAuthCallback_CodeExchange(t *testing.T) {
	t.Setenv("OAUTH_STATE_SECRET", "test-secret")
	t.Setenv("APP_URL", "https://app.example")
	bind := authsvc.OAuthBindHash(testVerifier)

	cases := []struct {
		name     string
		bind     string
		wantPath string
		wantKeys []string // Location のクエリに必ずあるもの
		noKeys   []string // Location のクエリに無いこと
	}{
		{"トークンを載せずコードだけ渡す", bind, "/auth/callback", []string{"code"}, []string{"user"}},
		{"照合値が無ければログインを止める", "", "", []string{"error"}, []string{"user", "code"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// 旧方式のフラグが残っていても、旧経路（user=）に戻らないこと (SOCAIAGENT-707)
			t.Setenv("OAUTH_CODE_EXCHANGE", "false")
			c := NewOAuthController(fakeOAuthService{}, nil, authsvc.NewOAuthCodeStore(nil))
			loc := googleCallback(t, c, tt.bind)
			if loc.Path != tt.wantPath {
				t.Fatalf("path = %q want %q", loc.Path, tt.wantPath)
			}
			for _, k := range tt.wantKeys {
				if loc.Query().Get(k) == "" {
					t.Fatalf("query %q が無い: %s", k, loc)
				}
			}
			for _, k := range tt.noKeys {
				if loc.Query().Has(k) {
					t.Fatalf("query %q があってはいけない: %s", k, loc)
				}
			}
			if strings.Contains(loc.String(), "user-jwt") {
				t.Fatal("URL にトークンが載っている")
			}
		})
	}
}

func TestOAuthExchangeCode(t *testing.T) {
	t.Setenv("OAUTH_STATE_SECRET", "test-secret")
	t.Setenv("APP_URL", "https://app.example")

	exchange := func(c *OAuthController, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/exchange", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		if err := c.ExchangeCode(echo.New().NewContext(req, rec)); err != nil {
			t.Fatal(err)
		}
		return rec
	}
	body := func(code, verifier string) string {
		b, _ := json.Marshal(map[string]string{"code": code, "verifier": verifier})
		return string(b)
	}

	t.Run("正しい verifier なら認証情報を返し、二度目は拒否する", func(t *testing.T) {
		c := NewOAuthController(fakeOAuthService{}, nil, authsvc.NewOAuthCodeStore(nil))
		code := googleCallback(t, c, authsvc.OAuthBindHash(testVerifier)).Query().Get("code")

		rec := exchange(c, body(code, testVerifier))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body)
		}
		var got authsvc.AuthResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.UserToken != "user-jwt" || got.RefreshToken != "refresh" {
			t.Fatalf("body = %s", rec.Body)
		}
		if rec := exchange(c, body(code, testVerifier)); rec.Code != http.StatusBadRequest {
			t.Fatalf("再利用 status = %d", rec.Code)
		}
	})

	t.Run("別のブラウザの verifier では交換できない", func(t *testing.T) {
		c := NewOAuthController(fakeOAuthService{}, nil, authsvc.NewOAuthCodeStore(nil))
		code := googleCallback(t, c, authsvc.OAuthBindHash(testVerifier)).Query().Get("code")
		rec := exchange(c, body(code, "victim-browser-verifier"))
		if rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), "user-jwt") {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body)
		}
	})

	for _, tt := range []struct{ name, body string }{
		{"code なしは 400", body("", testVerifier)},
		{"verifier なしは 400", body("x", "")},
		{"壊れた JSON は 400", "{"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := NewOAuthController(fakeOAuthService{}, nil, authsvc.NewOAuthCodeStore(nil))
			if rec := exchange(c, tt.body); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d", rec.Code)
			}
		})
	}
}
