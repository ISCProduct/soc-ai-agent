package matching

import (
	"Backend/domain/entity"
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const aiReasonText = "AI が生成したマッチング理由"

// capturingMatchRepo は保存されたマッチ結果を検査するために CreateOrUpdateBatch だけ差し替える。
type capturingMatchRepo struct {
	matchingMatchRepo
	captured []*entity.UserCompanyMatch
}

func (s *capturingMatchRepo) CreateOrUpdateBatch(matches []*entity.UserCompanyMatch) (int, error) {
	s.captured = matches
	return len(matches), nil
}

// newCountingLLMServer は /responses を返し、呼び出し回数を数えるモックサーバを返す。
func newCountingLLMServer(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": aiReasonText,
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// buildScoredCompanies は技術志向の重みを1社ずつ変えて、スコアが一意に定まる企業群を作る。
func buildScoredCompanies(n int) (*matchingCompanyRepo, *matchingScoreRepo) {
	companies := make([]models.Company, n)
	profiles := make(map[uint]*models.CompanyWeightProfile, n)
	for i := 0; i < n; i++ {
		id := uint(i + 1)
		companies[i] = models.Company{ID: id, Name: "会社" + string(rune('A'+i%26)), Industry: "IT"}
		// TechnicalOrientation を 1 ずつ変えることで match_score に差をつける
		profiles[id] = &models.CompanyWeightProfile{CompanyID: id, TechnicalOrientation: i + 1}
	}
	return &matchingCompanyRepo{companies: companies, profiles: profiles},
		&matchingScoreRepo{scores: []entity.UserWeightScore{{WeightCategory: "技術志向", Score: 70}}}
}

// TestCalculateMatching_AIReasonOnlyForTopN は #1061 の受け入れ条件。
// 表示される上位N件だけ AI 理由を生成し、残りはテンプレのままであることを検証する。
func TestCalculateMatching_AIReasonOnlyForTopN(t *testing.T) {
	const companyCount = 30
	const topN = 5

	t.Setenv("MATCHING_REASON_USE_AI", "true")
	t.Setenv("MATCHING_REASON_AI_TOP_N", "5")

	var calls atomic.Int32
	srv := newCountingLLMServer(t, &calls)

	companyRepo, scoreRepo := buildScoredCompanies(companyCount)
	matchRepo := &capturingMatchRepo{}
	svc := NewMatchingService(scoreRepo, companyRepo, matchRepo, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))

	if err := svc.CalculateMatching(context.Background(), 10, "sess-topn"); err != nil {
		t.Fatalf("CalculateMatching: %v", err)
	}

	if got := int(calls.Load()); got != topN {
		t.Fatalf("LLM 呼び出し回数=%d want %d（全%d社ぶん呼んでいないか）", got, topN, companyCount)
	}
	if len(matchRepo.captured) != companyCount {
		t.Fatalf("保存件数=%d want %d（AI対象外も保存されるべき）", len(matchRepo.captured), companyCount)
	}

	aiCount := 0
	for _, m := range matchRepo.captured {
		if m.MatchReason == aiReasonText {
			aiCount++
			continue
		}
		if strings.TrimSpace(m.MatchReason) == "" {
			t.Fatalf("company %d の理由が空。AI対象外はテンプレ理由が入るべき", m.CompanyID)
		}
	}
	if aiCount != topN {
		t.Fatalf("AI理由が入ったマッチ数=%d want %d", aiCount, topN)
	}

	// AI 理由が入ったのはスコア上位N件であること
	top := topMatchesByScore(matchRepo.captured, topN)
	for _, m := range top {
		if m.MatchReason != aiReasonText {
			t.Fatalf("上位マッチ(company=%d, score=%.2f)にAI理由が入っていない", m.CompanyID, m.MatchScore)
		}
	}
}

// TestCalculateMatching_NoAICallsWhenFlagOff は #588 の受け入れ条件
// 「CalculateMatching 中に外部LLMが走らないこと」を引き継ぐ。
func TestCalculateMatching_NoAICallsWhenFlagOff(t *testing.T) {
	t.Setenv("MATCHING_REASON_USE_AI", "")

	var calls atomic.Int32
	srv := newCountingLLMServer(t, &calls)

	companyRepo, scoreRepo := buildScoredCompanies(10)
	matchRepo := &capturingMatchRepo{}
	svc := NewMatchingService(scoreRepo, companyRepo, matchRepo, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))

	if err := svc.CalculateMatching(context.Background(), 10, "sess-off"); err != nil {
		t.Fatalf("CalculateMatching: %v", err)
	}

	if got := calls.Load(); got != 0 {
		t.Fatalf("既定オフなのに LLM を %d 回呼んでいる", got)
	}
	for _, m := range matchRepo.captured {
		if strings.TrimSpace(m.MatchReason) == "" {
			t.Fatalf("company %d の理由が空。テンプレ理由が入るべき", m.CompanyID)
		}
	}
}

func TestTopMatchesByScore(t *testing.T) {
	mk := func(id uint, score float64) *entity.UserCompanyMatch {
		return &entity.UserCompanyMatch{CompanyID: id, MatchScore: score}
	}
	pending := []*entity.UserCompanyMatch{mk(1, 10), mk(2, 50), mk(3, 30), mk(4, 50)}

	tests := []struct {
		name  string
		limit int
		want  []uint
	}{
		{"上位2件をスコア降順で返す", 2, []uint{2, 4}},
		{"同点は元の順序を保つ(SliceStable)", 3, []uint{2, 4, 3}},
		{"limitが件数を超えたら全件", 99, []uint{2, 4, 3, 1}},
		{"limit=0は空", 0, nil},
		{"負のlimitは空", -1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topMatchesByScore(pending, tt.limit)
			if len(got) != len(tt.want) {
				t.Fatalf("件数=%d want %d", len(got), len(tt.want))
			}
			for i, m := range got {
				if m.CompanyID != tt.want[i] {
					t.Fatalf("[%d] CompanyID=%d want %d", i, m.CompanyID, tt.want[i])
				}
			}
		})
	}
}

func TestTopMatchesByScore_DoesNotReorderInput(t *testing.T) {
	pending := []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 10},
		{CompanyID: 2, MatchScore: 90},
	}
	_ = topMatchesByScore(pending, 2)

	if pending[0].CompanyID != 1 || pending[1].CompanyID != 2 {
		t.Fatalf("入力スライスの順序が変わっている: %d, %d（保存順に影響する）",
			pending[0].CompanyID, pending[1].CompanyID)
	}
}

func TestMatchingReasonAITopN(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{"未指定は既定20", "", 20},
		{"正常値はそのまま", "5", 5},
		{"0以下は既定", "0", 20},
		{"負値は既定", "-3", 20},
		{"数値でなければ既定", "abc", 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MATCHING_REASON_AI_TOP_N", tt.env)
			if got := config.MatchingReasonAITopN(); got != tt.want {
				t.Fatalf("MatchingReasonAITopN()=%d want %d", got, tt.want)
			}
		})
	}
}
