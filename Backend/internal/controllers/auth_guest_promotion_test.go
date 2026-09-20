package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/services/auth"
)

func promotionTestService() *mockAuthService {
	return &mockAuthService{
		registerFn: func(_ auth.RegisterRequest, _ uint) (*auth.AuthResponse, error) {
			return &auth.AuthResponse{UserID: 42, Email: "s@example.com"}, nil
		},
	}
}

func postRegister(t *testing.T, svc *mockAuthService, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	e, ctrl := newAuthTestServer(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-User-Token", token)
	}
	rec := httptest.NewRecorder()
	e.POST("/api/auth/register", ctrl.Register)
	e.ServeHTTP(rec, req)
	return rec
}

const regBody = `{"email":"s@example.com","password":"password123","name":"山田太郎"`

// TestRegister_PromotionIdentityComesFromToken は #1374 の安全側の要。
//
// 昇格対象をボディで指定できると、他人のゲストアカウントを奪える。
// 対象は必ず X-User-Token から決めること。
func TestRegister_PromotionIdentityComesFromToken(t *testing.T) {
	token, err := middleware.GenerateJWT(7, "guest@temp.local", authTestUserSecret)
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	svc := promotionTestService()
	// ボディで別人(999)を指定しても無視されること
	rec := postRegister(t, svc, regBody+`,"promote_guest":true,"user_id":999}`, token)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.promotedGuestUserID != 7 {
		t.Errorf("昇格対象 = %d, want 7（トークンの主体）", svc.promotedGuestUserID)
	}
	if svc.promotedGuestUserID == 999 {
		t.Error("ボディの値で昇格対象を決めている（他人のアカウントを奪える）")
	}
}

// 引き継ぎを希望していないときは、トークンがあっても昇格しない。
// 既存のログイン状態のまま別アカウントを作る経路を壊さないため。
func TestRegister_NoPromotionWithoutFlag(t *testing.T) {
	token, _ := middleware.GenerateJWT(7, "guest@temp.local", authTestUserSecret)
	svc := promotionTestService()

	rec := postRegister(t, svc, regBody+`}`, token)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.promotedGuestUserID != 0 {
		t.Errorf("フラグ無しで昇格している: %d", svc.promotedGuestUserID)
	}
}

// 引き継ぎを希望しているのにトークンが無い／無効なら、黙って新規作成しない。
// 新規作成すると学生は診断をやり直すことになるのに、画面上は成功に見える。
func TestRegister_PromotionWithoutValidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"トークン無し", ""},
		{"壊れたトークン", "not-a-jwt"},
		{"別のシークレットで署名", mustJWT(t, 7, "other-secret")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := promotionTestService()
			rec := postRegister(t, svc, regBody+`,"promote_guest":true}`, tt.token)

			if rec.Code == http.StatusCreated {
				t.Fatal("黙って新規作成している（診断結果が失われるのに成功に見える）")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if svc.promotedGuestUserID != 0 {
				t.Errorf("昇格対象が設定されている: %d", svc.promotedGuestUserID)
			}
		})
	}
}

func mustJWT(t *testing.T, userID uint, secret string) string {
	t.Helper()
	tok, err := middleware.GenerateJWT(userID, "x@example.com", secret)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return tok
}
