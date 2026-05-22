package controllers_test

// AuthControllerのHTTPハンドラーテスト (Issue #397/#422)
//
// 実行: cd Backend && go test ./test/controllers/... -run Auth -v

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthControllerWithMock(svc *mocks.AuthServiceMock) *controllers.AuthController {
	return controllers.NewAuthController(svc)
}

// ---- Register ----

func TestAuthController_Register_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/register", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).Register(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_Register_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).Register(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_Register_EmailAlreadyExists(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("Register", services.RegisterRequest{Email: "test@example.com", Password: "pass"}).
		Return(nil, errors.New("email already exists"))

	body, _ := json.Marshal(map[string]string{"email": "test@example.com", "password": "pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).Register(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_Register_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	resp := &services.AuthResponse{Token: "jwt-token"}
	svc.On("Register", services.RegisterRequest{Email: "new@example.com", Password: "pass123"}).
		Return(resp, nil)

	body, _ := json.Marshal(map[string]string{"email": "new@example.com", "password": "pass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).Register(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got services.AuthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "jwt-token", got.Token)
	svc.AssertExpectations(t)
}

// ---- Login ----

func TestAuthController_Login_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/login", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).Login(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_Login_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).Login(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_Login_InvalidCredentials(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("Login", services.LoginRequest{Email: "bad@example.com", Password: "wrong"}).
		Return(nil, errors.New("invalid email or password"))

	body, _ := json.Marshal(map[string]string{"email": "bad@example.com", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).Login(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_Login_EmailNotVerified(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("Login", services.LoginRequest{Email: "unverified@example.com", Password: "pass"}).
		Return(nil, errors.New("email_not_verified"))

	body, _ := json.Marshal(map[string]string{"email": "unverified@example.com", "password": "pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).Login(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_Login_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	resp := &services.AuthResponse{Token: "valid-token"}
	svc.On("Login", services.LoginRequest{Email: "user@example.com", Password: "correct"}).
		Return(resp, nil)

	body, _ := json.Marshal(map[string]string{"email": "user@example.com", "password": "correct"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).Login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got services.AuthResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "valid-token", got.Token)
	svc.AssertExpectations(t)
}

// ---- CreateGuest ----

func TestAuthController_CreateGuest_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/guest", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).CreateGuest(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_CreateGuest_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	resp := &services.AuthResponse{Token: "guest-token"}
	svc.On("CreateGuestUser").Return(resp, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest", nil)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).CreateGuest(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

// ---- GetUser ----

func TestAuthController_GetUser_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/me", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).GetUser(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_GetUser_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).GetUser(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthController_GetUser_NotFound(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("GetUser", uint(99)).Return(nil, errors.New("user not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = withUserID(req, 99)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).GetUser(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_GetUser_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	resp := &services.AuthResponse{Token: "tok", UserID: 1}
	svc.On("GetUser", uint(1)).Return(resp, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).GetUser(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- VerifyRegistration ----

func TestAuthController_VerifyRegistration_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/verify-registration", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).VerifyRegistration(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_VerifyRegistration_MissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-registration", nil)
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).VerifyRegistration(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_VerifyRegistration_InvalidToken(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ValidateRegistrationToken", "bad-token").Return("", errors.New("invalid token"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-registration?token=bad-token", nil)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).VerifyRegistration(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_VerifyRegistration_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ValidateRegistrationToken", "valid-token").Return("user@example.com", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-registration?token=valid-token", nil)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).VerifyRegistration(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "user@example.com", resp["email"])
	svc.AssertExpectations(t)
}

// ---- RequestPasswordReset ----

func TestAuthController_RequestPasswordReset_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/forgot-password", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).RequestPasswordReset(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_RequestPasswordReset_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).RequestPasswordReset(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_RequestPasswordReset_AlwaysOK(t *testing.T) {
	// 情報漏洩防止のため、エラー時も200を返す仕様
	svc := &mocks.AuthServiceMock{}
	svc.On("RequestPasswordReset", "any@example.com").Return(nil)

	body, _ := json.Marshal(map[string]string{"email": "any@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).RequestPasswordReset(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- ResetPassword ----

func TestAuthController_ResetPassword_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/reset-password", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).ResetPassword(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_ResetPassword_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).ResetPassword(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_ResetPassword_InvalidToken(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ResetPassword", "invalid", "newpass").Return(errors.New("invalid token"))

	body, _ := json.Marshal(map[string]string{"token": "invalid", "password": "newpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).ResetPassword(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_ResetPassword_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("ResetPassword", "valid-token", "newpassword").Return(nil)

	body, _ := json.Marshal(map[string]string{"token": "valid-token", "password": "newpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).ResetPassword(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- DeleteAccount ----

func TestAuthController_DeleteAccount_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/account", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).DeleteAccount(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_DeleteAccount_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).DeleteAccount(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthController_DeleteAccount_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("DeleteAccount", uint(1)).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).DeleteAccount(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- UpdateProfile ----

func TestAuthController_UpdateProfile_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/profile", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).UpdateProfile(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_UpdateProfile_Unauthorized(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{"name": "テスト"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/profile", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).UpdateProfile(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthController_UpdateProfile_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	resp := &services.AuthResponse{UserID: 1}
	svc.On("UpdateProfile", services.UpdateProfileRequest{UserID: 1, Name: "山田太郎"}).
		Return(resp, nil)

	body, _ := json.Marshal(map[string]interface{}{"name": "山田太郎"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/profile", bytes.NewBuffer(body))
	req = withUserID(req, 1)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).UpdateProfile(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- VerifyEmail ----

func TestAuthController_VerifyEmail_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/verify-email", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).VerifyEmail(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_VerifyEmail_InvalidToken(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("VerifyEmail", "bad").Return(errors.New("invalid token"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=bad", nil)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).VerifyEmail(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_VerifyEmail_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("VerifyEmail", "valid-token").Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email?token=valid-token", nil)
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).VerifyEmail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---- RequestRegistration ----

func TestAuthController_RequestRegistration_MethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/request-registration", nil)
			w := httptest.NewRecorder()
			controllers.NewAuthController(nil).RequestRegistration(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestAuthController_RequestRegistration_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-registration", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewAuthController(nil).RequestRegistration(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthController_RequestRegistration_EmailAlreadyExists(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("RequestRegistration", "dup@example.com").Return(errors.New("email already exists"))

	body, _ := json.Marshal(map[string]string{"email": "dup@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-registration", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).RequestRegistration(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	svc.AssertExpectations(t)
}

func TestAuthController_RequestRegistration_Success(t *testing.T) {
	svc := &mocks.AuthServiceMock{}
	svc.On("RequestRegistration", "new@example.com").Return(nil)

	body, _ := json.Marshal(map[string]string{"email": "new@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-registration", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthControllerWithMock(svc).RequestRegistration(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
