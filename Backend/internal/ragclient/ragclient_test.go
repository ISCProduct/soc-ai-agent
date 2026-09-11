package ragclient

import (
	"net/http"
	"testing"
)

func TestSetAuthHeader(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantHeader string
	}{
		{name: "トークン設定時はヘッダーを付与する", token: "secret-token", wantHeader: "secret-token"},
		{name: "空白のみのトークンは付与しない", token: "   ", wantHeader: ""},
		{name: "未設定時は付与しない", token: "", wantHeader: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RAG_INTERNAL_TOKEN", tt.token)

			req, err := http.NewRequest(http.MethodGet, "http://rag-review:9000/health", nil)
			if err != nil {
				t.Fatalf("リクエスト生成に失敗: %v", err)
			}
			SetAuthHeader(req)

			if got := req.Header.Get(InternalTokenHeader); got != tt.wantHeader {
				t.Errorf("ヘッダー = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}
