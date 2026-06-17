package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/services"
	"Backend/internal/services/interfaces"

	"github.com/labstack/echo/v4"
)

// ── モック ────────────────────────────────────────────────────────────────────

type mockAuthService struct {
	interfaces.AuthService
	registerFn            func(req services.RegisterRequest) (*services.AuthResponse, error)
	loginFn               func(req services.LoginRequest) (*services.AuthResponse, error)
	getUserFn             func(userID uint) (*services.AuthResponse, error)
	requestRegistrationFn func(email string) error
	validateRegTokenFn    func(token string) (string, error)
	updateProfileFn       func(req services.UpdateProfileRequest) (*services.AuthResponse, error)
	resetPasswordFn       func(token, pw string) error
	verifyEmailFn         func(token string) error
	createGuestFn         func() (*services.AuthResponse, error)
	deleteAccountFn       func(userID uint) error
}

func (m *mockAuthService) Register(req services.RegisterRequest) (*services.AuthResponse, error) {
	return m.registerFn(req)
}
func (m *mockAuthService) Login(req services.LoginRequest) (*services.AuthResponse, error) {
	return m.loginFn(req)
}
func (m *mockAuthService) GetUser(userID uint) (*services.AuthResponse, error) {
	return m.getUserFn(userID)
}
func (m *mockAuthService) RequestRegistration(email string) error {
	return m.requestRegistrationFn(email)
}
func (m *mockAuthService) ValidateRegistrationToken(token string) (string, error) {
	return m.validateRegTokenFn(token)
}
func (m *mockAuthService) UpdateProfile(req services.UpdateProfileRequest) (*services.AuthResponse, error) {
	return m.updateProfileFn(req)
}
func (m *mockAuthService) ResetPassword(token, pw string) error {
	return m.resetPasswordFn(token, pw)
}
func (m *mockAuthService) VerifyEmail(token string) error {
	return m.verifyEmailFn(token)
}
func (m *mockAuthService) CreateGuestUser() (*services.AuthResponse, error) {
	return m.createGuestFn()
}
func (m *mockAuthService) DeleteAccount(userID uint) error {
	return m.deleteAccountFn(userID)
}
func (m *mockAuthService) RequestPasswordReset(_ string) error { return nil }

// ── テスト共通セットアップ ─────────────────────────────────────────────────────

// newAuthTestServer はルート登録済みのEchoと記録済みレスポンスを返すヘルパー。
// e.ServeHTTP 経由で呼ぶことでカスタムエラーハンドラーが必ず通る。
func newAuthTestServer(svc interfaces.AuthService) (*echo.Echo, *AuthController) {
	e := echo.New()
	e.HTTPErrorHandler = middleware.CustomHTTPErrorHandler
	ctrl := NewAuthController(svc)
	return e, ctrl
}

func servePost(e *echo.Echo, path, body string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	e.POST(path, handler)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func serveGet(e *echo.Echo, path string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	e.GET(path, handler)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func serveDelete(e *echo.Echo, path string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	e.DELETE(path, handler)
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeAuthErrResp(t *testing.T, rec *httptest.ResponseRecorder) middleware.ErrorResponse {
	t.Helper()
	var resp middleware.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSONデコード失敗: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_DuplicateEmail_ReturnsDuplicateEmailCode(t *testing.T) {
	svc := &mockAuthService{
		registerFn: func(_ services.RegisterRequest) (*services.AuthResponse, error) {
			return nil, errors.New("email already exists")
		},
	}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/register", `{"email":"a@example.com","password":"pass"}`, ctrl.Register)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeDuplicateEmail {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeDuplicateEmail)
	}
}

func TestRegister_InvalidBody_ReturnsValidationError(t *testing.T) {
	svc := &mockAuthService{}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/register", `{invalid json`, ctrl.Register)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeValidationError {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeValidationError)
	}
}

func TestRegister_OtherError_ReturnsValidationError(t *testing.T) {
	svc := &mockAuthService{
		registerFn: func(_ services.RegisterRequest) (*services.AuthResponse, error) {
			return nil, errors.New("weak password")
		},
	}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/register", `{"email":"a@example.com","password":"pw"}`, ctrl.Register)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeValidationError {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeValidationError)
	}
}

func TestRegister_Success(t *testing.T) {
	svc := &mockAuthService{
		registerFn: func(_ services.RegisterRequest) (*services.AuthResponse, error) {
			return &services.AuthResponse{}, nil
		},
	}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/register", `{"email":"a@example.com","password":"pass"}`, ctrl.Register)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_InvalidCredentials_ReturnsUnauthorized(t *testing.T) {
	svc := &mockAuthService{
		loginFn: func(_ services.LoginRequest) (*services.AuthResponse, error) {
			return nil, errors.New("invalid email or password")
		},
	}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/login", `{"email":"a@example.com","password":"wrong"}`, ctrl.Login)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeUnauthorized)
	}
}

func TestLogin_EmailNotVerified_ReturnsForbidden(t *testing.T) {
	for _, msg := range []string{"email_not_verified", "re_verification_required"} {
		t.Run(msg, func(t *testing.T) {
			svc := &mockAuthService{
				loginFn: func(_ services.LoginRequest) (*services.AuthResponse, error) {
					return nil, errors.New(msg)
				},
			}
			e, ctrl := newAuthTestServer(svc)
			rec := servePost(e, "/login", `{"email":"a@example.com","password":"pass"}`, ctrl.Login)

			if rec.Code != http.StatusForbidden {
				t.Errorf("msg=%q: status = %d, want %d", msg, rec.Code, http.StatusForbidden)
			}
			resp := decodeAuthErrResp(t, rec)
			if resp.Code != ErrCodeForbidden {
				t.Errorf("msg=%q: code = %q, want %q", msg, resp.Code, ErrCodeForbidden)
			}
		})
	}
}

func TestLogin_Success(t *testing.T) {
	svc := &mockAuthService{
		loginFn: func(_ services.LoginRequest) (*services.AuthResponse, error) {
			return &services.AuthResponse{}, nil
		},
	}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/login", `{"email":"a@example.com","password":"pass"}`, ctrl.Login)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// ── GetUser ───────────────────────────────────────────────────────────────────

func TestGetUser_Unauthorized_WhenNoUserIDInContext(t *testing.T) {
	svc := &mockAuthService{}
	e, ctrl := newAuthTestServer(svc)
	rec := serveGet(e, "/user", ctrl.GetUser)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeUnauthorized)
	}
}

// ── RequestRegistration ───────────────────────────────────────────────────────

func TestRequestRegistration_DuplicateEmail(t *testing.T) {
	svc := &mockAuthService{
		requestRegistrationFn: func(_ string) error {
			return errors.New("email already exists")
		},
	}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/registration", `{"email":"a@example.com"}`, ctrl.RequestRegistration)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeDuplicateEmail {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeDuplicateEmail)
	}
}

func TestRequestRegistration_InvalidBody(t *testing.T) {
	svc := &mockAuthService{}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/registration", `{bad`, ctrl.RequestRegistration)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeValidationError {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeValidationError)
	}
}

// ── VerifyRegistration ────────────────────────────────────────────────────────

func TestVerifyRegistration_MissingToken(t *testing.T) {
	svc := &mockAuthService{}
	e, ctrl := newAuthTestServer(svc)
	rec := serveGet(e, "/verify", ctrl.VerifyRegistration)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeValidationError {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeValidationError)
	}
}

// ── ResetPassword ─────────────────────────────────────────────────────────────

func TestResetPassword_InvalidToken_ReturnsValidationError(t *testing.T) {
	svc := &mockAuthService{
		resetPasswordFn: func(_, _ string) error {
			return errors.New("invalid or expired token")
		},
	}
	e, ctrl := newAuthTestServer(svc)
	rec := servePost(e, "/reset", `{"token":"bad","password":"newpass"}`, ctrl.ResetPassword)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeValidationError {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeValidationError)
	}
}

// ── DeleteAccount ─────────────────────────────────────────────────────────────

func TestDeleteAccount_Unauthorized_WhenNoUserID(t *testing.T) {
	svc := &mockAuthService{}
	e, ctrl := newAuthTestServer(svc)
	rec := serveDelete(e, "/account", ctrl.DeleteAccount)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	resp := decodeAuthErrResp(t, rec)
	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("code = %q, want %q", resp.Code, ErrCodeUnauthorized)
	}
}
