package controllers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/services/matching"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/mock"
)

// TestGetRecommendations_ReasonIsBuiltWhenNotStored は、match_reason が
// 保存されていなくても推薦一覧に理由が載ることを検証する。
//
// CalculateMatching は全公開企業ぶんのテンプレ理由を保存しなくなった。
// 表示されるのは上位10件だけなのに、本番想定の4,000社では1回の診断で
// 67MB を確保して TEXT 列を4,000行書いていたため（確保メモリの96%）。
//
// 保存をやめられる根拠が「読み出し側が BuildMatchReason を呼び直すこと」なので、
// その前提が崩れたら気付けるようにここで固定する。崩れると利用者には
// 「おすすめ理由が空欄」として出る。
func TestGetRecommendations_ReasonIsBuiltWhenNotStored(t *testing.T) {
	const sessionID = "s1"
	const userID = uint(1)

	scores := []entity.UserWeightScore{
		{WeightCategory: "technical_orientation", Score: 80},
		{WeightCategory: "growth_orientation", Score: 70},
	}
	// 保存済みの理由は空。CalculateMatching が保存しなくなった状態を再現する。
	matches := []*entity.UserCompanyMatch{{
		ID:             11,
		UserID:         userID,
		SessionID:      sessionID,
		CompanyID:      7,
		Company:        &entity.Company{ID: 7, Name: "株式会社テスト", Industry: "情報通信業", MainBusiness: "BtoB SaaSの開発"},
		MatchScore:     72,
		TechnicalMatch: 85,
		GrowthMatch:    60,
		MatchReason:    "",
	}}

	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetUserScores", userID, sessionID).Return(scores, nil)
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("GetTopMatches", mock.Anything, userID, sessionID, 10).Return(matches, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id="+sessionID, nil)
	req = withUserID(req, userID)
	rec := httptest.NewRecorder()

	ctl := newChatController(chatSvc, matchSvc, nil, nil, nil)
	if err := ctl.GetRecommendations(newCtx(req, rec)); err != nil {
		t.Fatalf("GetRecommendations: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Recommendations []struct {
			CategoryName string `json:"category_name"`
			Reason       string `json:"reason"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Recommendations) != 1 {
		t.Fatalf("推薦件数 = %d, want 1", len(resp.Recommendations))
	}

	got := resp.Recommendations[0].Reason
	if strings.TrimSpace(got) == "" {
		t.Fatal("理由が空。保存をやめた前提（読み出し側が生成し直す）が崩れている")
	}
	// 保存していた場合と同じ文面になること。
	want := matching.BuildMatchReason(&entity.UserCompanyMatch{
		CompanyID:      7,
		Company:        &entity.Company{ID: 7, Name: "株式会社テスト", Industry: "情報通信業", MainBusiness: "BtoB SaaSの開発"},
		MatchScore:     72,
		TechnicalMatch: 85,
		GrowthMatch:    60,
	}, scores)
	if got != want {
		t.Errorf("生成された理由が保存時と異なる\n got: %s\nwant: %s", got, want)
	}
	// スコア帯に合った言い回しが出ていること（72% は「おおむね」帯）
	if !strings.Contains(got, "おおむね一致しています") {
		t.Errorf("マッチ度に応じた言い回しが出ていない:\n%s", got)
	}
}

// 保存済みの理由（AI生成）がある場合はそれを優先する。
// 上位N件の AI 理由は高価なので保存しており、読み出し側で作り直してはいけない。
func TestGetRecommendations_StoredAIReasonWins(t *testing.T) {
	const sessionID = "s1"
	const userID = uint(1)
	const aiReason = "AIが生成した固有の理由文です。"

	scores := []entity.UserWeightScore{{WeightCategory: "technical_orientation", Score: 80}}
	matches := []*entity.UserCompanyMatch{{
		ID:          11,
		UserID:      userID,
		SessionID:   sessionID,
		CompanyID:   7,
		Company:     &entity.Company{ID: 7, Name: "株式会社テスト", Industry: "情報通信業"},
		MatchScore:  91,
		MatchReason: aiReason,
	}}

	chatSvc := &mocks.ChatServiceMock{}
	chatSvc.On("GetUserScores", userID, sessionID).Return(scores, nil)
	matchSvc := &mocks.MatchingServiceMock{}
	matchSvc.On("GetTopMatches", mock.Anything, userID, sessionID, 10).Return(matches, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/recommendations?session_id="+sessionID, nil)
	req = withUserID(req, userID)
	rec := httptest.NewRecorder()

	ctl := newChatController(chatSvc, matchSvc, nil, nil, nil)
	if err := ctl.GetRecommendations(newCtx(req, rec)); err != nil {
		t.Fatalf("GetRecommendations: %v", err)
	}

	var resp struct {
		Recommendations []struct {
			Reason string `json:"reason"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Recommendations) != 1 || resp.Recommendations[0].Reason != aiReason {
		t.Errorf("保存済みのAI理由が使われていない: %+v", resp.Recommendations)
	}
}
