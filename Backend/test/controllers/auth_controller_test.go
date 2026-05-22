package controllers_test

// AuthControllerのHTTPハンドラーテスト (Issue #397)
//
// 実行: cd Backend && go test ./test/controllers/... -run Auth -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"

	"github.com/stretchr/testify/assert"
)

func newAuthController() *controllers.AuthController {
	return controllers.NewAuthController(nil)
}

// TestAuthController_Register_MethodNotAllowed はPOST以外に405を返すことを検証
func TestAuthController_Register_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/register", nil)
			w := httptest.NewRecorder()
			newAuthController().Register(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_Register_InvalidBody はJSONパースエラーに400を返すことを検証
func TestAuthController_Register_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	newAuthController().Register(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAuthController_Login_MethodNotAllowed はPOST以外に405を返すことを検証
func TestAuthController_Login_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/login", nil)
			w := httptest.NewRecorder()
			newAuthController().Login(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_Login_InvalidBody はJSONパースエラーに400を返すことを検証
func TestAuthController_Login_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	newAuthController().Login(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAuthController_CreateGuest_MethodNotAllowed はPOST以外に405を返すことを検証
func TestAuthController_CreateGuest_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/guest", nil)
			w := httptest.NewRecorder()
			newAuthController().CreateGuest(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_GetUser_MethodNotAllowed はGET以外に405を返すことを検証
func TestAuthController_GetUser_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/me", nil)
			w := httptest.NewRecorder()
			newAuthController().GetUser(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_GetUser_Unauthorized はuserIDがコンテキストにない場合に401を返すことを検証
func TestAuthController_GetUser_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()
	newAuthController().GetUser(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAuthController_RequestRegistration_MethodNotAllowed はPOST以外に405を返すことを検証
func TestAuthController_RequestRegistration_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/request-registration", nil)
			w := httptest.NewRecorder()
			newAuthController().RequestRegistration(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_RequestRegistration_InvalidBody はJSONパースエラーに400を返すことを検証
func TestAuthController_RequestRegistration_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/request-registration", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	newAuthController().RequestRegistration(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAuthController_VerifyRegistration_MethodNotAllowed はGET以外に405を返すことを検証
func TestAuthController_VerifyRegistration_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/verify-registration", nil)
			w := httptest.NewRecorder()
			newAuthController().VerifyRegistration(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_VerifyRegistration_MissingToken はtokenクエリパラメータなしに400を返すことを検証
func TestAuthController_VerifyRegistration_MissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-registration", nil)
	w := httptest.NewRecorder()
	newAuthController().VerifyRegistration(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAuthController_RequestPasswordReset_MethodNotAllowed はPOST以外に405を返すことを検証
func TestAuthController_RequestPasswordReset_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/forgot-password", nil)
			w := httptest.NewRecorder()
			newAuthController().RequestPasswordReset(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_RequestPasswordReset_InvalidBody はJSONパースエラーに400を返すことを検証
func TestAuthController_RequestPasswordReset_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	newAuthController().RequestPasswordReset(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAuthController_ResetPassword_MethodNotAllowed はPOST以外に405を返すことを検証
func TestAuthController_ResetPassword_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/reset-password", nil)
			w := httptest.NewRecorder()
			newAuthController().ResetPassword(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_ResetPassword_InvalidBody はJSONパースエラーに400を返すことを検証
func TestAuthController_ResetPassword_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	newAuthController().ResetPassword(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAuthController_VerifyEmail_MethodNotAllowed はGET/POST以外に405を返すことを検証
func TestAuthController_VerifyEmail_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/verify-email", nil)
			w := httptest.NewRecorder()
			newAuthController().VerifyEmail(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_DeleteAccount_MethodNotAllowed はDELETE以外に405を返すことを検証
func TestAuthController_DeleteAccount_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/account", nil)
			w := httptest.NewRecorder()
			newAuthController().DeleteAccount(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_DeleteAccount_Unauthorized はuserIDがコンテキストにない場合に401を返すことを検証
func TestAuthController_DeleteAccount_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
	w := httptest.NewRecorder()
	newAuthController().DeleteAccount(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAuthController_UpdateProfile_MethodNotAllowed はPOST以外に405を返すことを検証
func TestAuthController_UpdateProfile_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/profile", nil)
			w := httptest.NewRecorder()
			newAuthController().UpdateProfile(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestAuthController_UpdateProfile_Unauthorized はuserIDがコンテキストにない場合に401を返すことを検証
func TestAuthController_UpdateProfile_Unauthorized(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{"name": "テスト"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/profile", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	newAuthController().UpdateProfile(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
