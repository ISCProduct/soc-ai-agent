package chat_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chatcontrollers "Backend/internal/controllers/chat"
	"Backend/internal/controllers/mocks"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/services/chat"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
)

// チャット本文は文字数（ルーン）で上限を見て、超えたら AI を呼ぶ前に 400 で返す（SOCAIAGENT-702）。
func TestChatController_Chat_MessageLengthLimit(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		wantStatus int
	}{
		{"上限ちょうどは通す（日本語はルーン数で数える）", strings.Repeat("あ", chat.MaxChatMessageRunes), http.StatusOK},
		{"上限を1字超えたら400", strings.Repeat("あ", chat.MaxChatMessageRunes+1), http.StatusBadRequest},
		{"ASCII も同じ上限", strings.Repeat("a", chat.MaxChatMessageRunes+1), http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatSvc := &mocks.ChatServiceMock{}
			chatSvc.On("ClaimSessionOwnership", "s1", uint(1)).Return(nil).Maybe()
			chatSvc.On("SessionHasOtherUserMessages", "s1", uint(1)).Return(false, nil).Maybe()
			chatSvc.On("ProcessChat", mock.Anything, mock.Anything).Return(&chat.ChatResponse{Response: "ok"}, nil).Maybe()

			body, _ := json.Marshal(map[string]string{"session_id": "s1", "message": tt.message})
			req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req = testsupport.WithUserID(req, 1)
			rec := httptest.NewRecorder()
			err := chatcontrollers.NewChatController(chatSvc, nil, nil, nil, nil).Chat(testsupport.NewCtx(req, rec))

			status := rec.Code
			var he *echo.HTTPError
			if err != nil {
				if h, ok := err.(*echo.HTTPError); ok {
					he = h
					status = h.Code
				}
			}
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d (err=%v)", status, tt.wantStatus, err)
			}
			if tt.wantStatus == http.StatusBadRequest {
				if he == nil || he.Message != "文字数が上限を超えています" {
					t.Errorf("message = %v, want 文字数が上限を超えています", he)
				}
				chatSvc.AssertNotCalled(t, "ProcessChat", mock.Anything, mock.Anything)
			}
		})
	}
}
