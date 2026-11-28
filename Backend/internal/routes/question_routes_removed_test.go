package routes

import (
	"net/http"
	"testing"
)

// 呼び出し元の無い質問の生成・作成・一覧の入口は削除した。再び登録されたら落ちる(SOCAIAGENT-694)。
func TestQuestionRoutes_AreRemoved(t *testing.T) {
	removed := []Route{
		{Method: http.MethodPost, Path: "/api/questions/generate"},
		{Method: http.MethodPost, Path: "/api/questions/create"},
		{Method: http.MethodGet, Path: "/api/questions/list"},
	}
	registered := Inventory()
	for _, r := range removed {
		t.Run(r.Method+" "+r.Path, func(t *testing.T) {
			for _, got := range registered {
				if got.Method == r.Method && got.Path == r.Path {
					t.Fatalf("%s %s が登録されている", r.Method, r.Path)
				}
			}
		})
	}
}
