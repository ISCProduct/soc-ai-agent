package auth_test

// AuthControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./internal/controllers/auth/... -run Auth -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	authcontrollers "Backend/internal/controllers/auth"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/services/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthController(svc *mocks.AuthServiceMock) *authcontrollers.AuthController {
	return authcontrollers.NewAuthController(svc, "test-user-secret")
}

// ---- Register ----

func TestAuthController_Register_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, authcontrollers.NewAuthController(nil, "test-user-secret").Register, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAuthController_Register_EmailExists(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	regReq := auth.RegisterRequest{Email: "dup@example.com", Password: "pass1234", Name: "テスト"}
	svc.On("Register", regReq, uint(0), uint(0)).Return(nil, errors.New("email already exists"))

	body, _ := json.Marshal(regReq)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).Register, testsupport.NewCtx(req, rec), http.StatusConflict)
	svc.AssertExpectations(t)
}

func TestAuthController_Register_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	regReq := auth.RegisterRequest{Email: "new@example.com", Password: "pass1234", Name: "テスト"}
	resp := &auth.AuthResponse{Token: "tok"}
	svc.On("Register", regReq, uint(0), uint(0)).Return(resp, nil)

	body, _ := json.Marshal(regReq)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).Register, testsupport.NewCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

// ---- Login ----

func TestAuthController_Login_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, authcontrollers.NewAuthController(nil, "test-user-secret").Login, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAuthController_Login_InvalidCredentials(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	loginReq := auth.LoginRequest{Email: "a@example.com", Password: "wrong"}
	svc.On("Login", loginReq, uint(0)).Return(nil, errors.New("invalid email or password"))

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).Login, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
	svc.AssertExpectations(t)
}

func TestAuthController_Login_EmailNotVerified(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	loginReq := auth.LoginRequest{Email: "a@example.com", Password: "pass1234"}
	svc.On("Login", loginReq, uint(0)).Return(nil, errors.New("email_not_verified"))

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).Login, testsupport.NewCtx(req, rec), http.StatusForbidden)
	svc.AssertExpectations(t)
}

func TestAuthController_Login_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	loginReq := auth.LoginRequest{Email: "a@example.com", Password: "pass1234"}
	resp := &auth.AuthResponse{Token: "jwt-token"}
	svc.On("Login", loginReq, uint(0)).Return(resp, nil)

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).Login, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- CreateGuest ----

func TestAuthController_CreateGuest_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("CreateGuestUser", uint(0)).Return(&auth.AuthResponse{Token: "guest-tok"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).CreateGuest, testsupport.NewCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

// ---- GetUser ----

func TestAuthController_GetUser_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, authcontrollers.NewAuthController(nil, "test-user-secret").GetUser, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestAuthController_GetUser_NotFound(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("GetUser", uint(1)).Return(nil, errors.New("user not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).GetUser, testsupport.NewCtx(req, rec), http.StatusNotFound)
	svc.AssertExpectations(t)
}

func TestAuthController_GetUser_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("GetUser", uint(1)).Return(&auth.AuthResponse{Token: "tok"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).GetUser, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- RequestRegistration ----

func TestAuthController_RequestRegistration_Conflict(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("RequestRegistration", "dup@example.com").Return(errors.New("email already exists"))

	body, _ := json.Marshal(map[string]string{"email": "dup@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-registration", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).RequestRegistration, testsupport.NewCtx(req, rec), http.StatusConflict)
	svc.AssertExpectations(t)
}

func TestAuthController_RequestRegistration_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("RequestRegistration", "new@example.com").Return(nil)

	body, _ := json.Marshal(map[string]string{"email": "new@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-registration", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).RequestRegistration, testsupport.NewCtx(req, rec), http.StatusOK)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "confirmation email sent", resp["message"])
	svc.AssertExpectations(t)
}

// ---- VerifyRegistration ----

func TestAuthController_VerifyRegistration_MissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-registration", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, authcontrollers.NewAuthController(nil, "test-user-secret").VerifyRegistration, testsupport.NewCtx(req, rec), http.StatusBadRequest)
}

func TestAuthController_VerifyRegistration_InvalidToken(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ValidateRegistrationToken", "bad-token").Return("", errors.New("invalid token"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-registration?token=bad-token", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).VerifyRegistration, testsupport.NewCtx(req, rec), http.StatusBadRequest)
	svc.AssertExpectations(t)
}

func TestAuthController_VerifyRegistration_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ValidateRegistrationToken", "valid-token").Return("user@example.com", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-registration?token=valid-token", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).VerifyRegistration, testsupport.NewCtx(req, rec), http.StatusOK)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "user@example.com", resp["email"])
	svc.AssertExpectations(t)
}

// ---- UpdateProfile ----

func TestAuthController_UpdateProfile_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, authcontrollers.NewAuthController(nil, "test-user-secret").UpdateProfile, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestAuthController_UpdateProfile_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	updateReq := auth.UpdateProfileRequest{UserID: 1, Name: "更新太郎"}
	svc.On("UpdateProfile", updateReq).Return(&auth.AuthResponse{Token: "tok"}, nil)

	body, _ := json.Marshal(map[string]any{"name": "更新太郎"})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).UpdateProfile, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- RequestPasswordReset ----

func TestAuthController_RequestPasswordReset_AlwaysOK(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("RequestPasswordReset", "any@example.com").Return(errors.New("not found"))

	body, _ := json.Marshal(map[string]string{"email": "any@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	// エラーがあっても常に200（情報漏洩防止）
	testsupport.AssertStatus(t, newAuthController(svc).RequestPasswordReset, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- ResetPassword ----

func TestAuthController_ResetPassword_InvalidToken(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ResetPassword", "bad-tok", "newpass").Return(errors.New("invalid token"))

	body, _ := json.Marshal(map[string]string{"token": "bad-tok", "password": "newpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).ResetPassword, testsupport.NewCtx(req, rec), http.StatusBadRequest)
	svc.AssertExpectations(t)
}

func TestAuthController_ResetPassword_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ResetPassword", "valid-tok", "newpass").Return(nil)

	body, _ := json.Marshal(map[string]string{"token": "valid-tok", "password": "newpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).ResetPassword, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- VerifyEmail ----

func TestAuthController_VerifyEmail_InvalidToken(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("VerifyEmail", "bad").Return(errors.New("invalid token"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=bad", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).VerifyEmail, testsupport.NewCtx(req, rec), http.StatusBadRequest)
	svc.AssertExpectations(t)
}

func TestAuthController_VerifyEmail_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("VerifyEmail", "valid").Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=valid", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).VerifyEmail, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

// ---- DeleteAccount ----

func TestAuthController_DeleteAccount_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, authcontrollers.NewAuthController(nil, "test-user-secret").DeleteAccount, testsupport.NewCtx(req, rec), http.StatusUnauthorized)
}

func TestAuthController_DeleteAccount_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("DeleteAccount", uint(1)).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
	req = testsupport.WithUserID(req, 1)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, newAuthController(svc).DeleteAccount, testsupport.NewCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}
