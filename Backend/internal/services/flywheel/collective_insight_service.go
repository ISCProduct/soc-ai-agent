package flywheel

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// CollectiveInsightService 集合知レコメンドサービス
type CollectiveInsightService struct {
	repo            *repositories.CollectiveInsightRepository
	weightScoreRepo *repositories.UserWeightScoreRepository
}

func NewCollectiveInsightService(
	repo *repositories.CollectiveInsightRepository,
	weightScoreRepo *repositories.UserWeightScoreRepository,
) *CollectiveInsightService {
	return &CollectiveInsightService{repo: repo, weightScoreRepo: weightScoreRepo}
}

// ── 行動ログ記録 ─────────────────────────────────────────────────────────────

// RecordAction ユーザー行動を匿名ログとして記録する
// 同意していないユーザーの行動は記録しない
func (s *CollectiveInsightService) RecordAction(userID uint, sessionID string, companyID uint, actionType string) error {
	consented, err := s.repo.IsConsentGiven(userID)
	if err != nil || !consented {
		return nil // 未同意は黙って無視
	}

	// スコアスナップショット取得
	scores, _ := s.weightScoreRepo.FindByUserAndSession(userID, sessionID)
	snapshot := map[string]int{}
	for _, sc := range scores {
		snapshot[sc.WeightCategory] = sc.Score
	}
	snapJSON, _ := json.Marshal(snapshot)

	log := &models.CollectiveInsightLog{
		AnonymousUserID: anonymizeUserID(userID),
		CompanyID:       companyID,
		ActionType:      actionType,
		ScoreSnapshot:   string(snapJSON),
	}
	return s.repo.LogAction(log)
}

// UpdateConsent ユーザーの集合知参加同意を更新する
func (s *CollectiveInsightService) UpdateConsent(userID uint, allow bool) error {
	return s.repo.UpdateConsent(userID, allow)
}

// ── 類似ユーザー検索 ─────────────────────────────────────────────────────────

// CollectiveRecommendItem 集合知レコメンドアイテム
type CollectiveRecommendItem struct {
	CompanyID       uint    `json:"company_id"`
	CompanyName     string  `json:"company_name"`
	PassCount       int     `json:"pass_count"`        // 類似ユーザーの通過人数
	SimilarUsers    int     `json:"similar_users"`     // 類似ユーザー数
	CollectiveScore float64 `json:"collective_score"`  // 集合知スコア（0-100）
	Reason          string  `json:"reason"`
}

// GetCollectiveRecommendations 類似スコアプロファイルを持つユーザーの通過企業をレコメンドする
func (s *CollectiveInsightService) GetCollectiveRecommendations(
	userID uint,
	sessionID string,
	excludeCompanyIDs []uint,
) ([]CollectiveRecommendItem, error) {
	// 自分のスコアを取得
	myScores, err := s.weightScoreRepo.FindByUserAndSession(userID, sessionID)
	if err != nil || len(myScores) == 0 {
		return nil, nil
	}

	// スコアマップに変換
	myScoreMap := map[string]float64{}
	for _, sc := range myScores {
		myScoreMap[sc.WeightCategory] = float64(sc.Score)
	}

	// 過去の行動ログからスナップショットを取得して類似ユーザーを特定
	logs, err := s.repo.GetUserScoreSnapshots(500)
	if err != nil {
		return nil, err
	}

	similarHashes := findSimilarUsers(myScoreMap, anonymizeUserID(userID), logs, 0.85, 20)
	if len(similarHashes) == 0 {
		return nil, nil
	}

	rows, err := s.repo.GetPassedCompaniesBySimilarUsers(similarHashes, excludeCompanyIDs)
	if err != nil {
		return nil, err
	}

	items := make([]CollectiveRecommendItem, 0, len(rows))
	for _, row := range rows {
		score := math.Min(row.CollectiveScore*100, 100)
		score = math.Round(score*10) / 10
		items = append(items, CollectiveRecommendItem{
			CompanyID:       row.CompanyID,
			CompanyName:     row.CompanyName,
			PassCount:       row.PassCount,
			SimilarUsers:    len(similarHashes),
			CollectiveScore: score,
			Reason:          fmt.Sprintf("あなたと似たスコアプロファイルの%d人が通過・応募した企業です", row.PassCount),
		})
	}
	return items, nil
}

// RebuildSummaries 企業別行動サマリーを再集計する（管理バッチ用）
func (s *CollectiveInsightService) RebuildSummaries() error {
	return s.repo.RebuildSummaries()
}

// GetTopPassRateCompanies 全ユーザー通過率上位の企業を返す
func (s *CollectiveInsightService) GetTopPassRateCompanies(limit int) ([]models.AnonymizedBehaviorSummary, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.GetTopPassRateCompanies(limit)
}

// ── ヘルパー ─────────────────────────────────────────────────────────────────

// anonymizeUserID userIDをSHA-256でハッシュ化する
func anonymizeUserID(userID uint) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("user:%d:collective", userID)))
	return fmt.Sprintf("%x", h)
}

// cosineSimMap 2つのスコアマップのコサイン類似度を計算する
func cosineSimMap(a, b map[string]float64) float64 {
	dot, normA, normB := 0.0, 0.0, 0.0
	for k, va := range a {
		vb := b[k]
		dot += va * vb
		normA += va * va
	}
	for _, vb := range b {
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// findSimilarUsers 類似スコアプロファイルを持つ匿名ユーザーIDを返す
func findSimilarUsers(
	myScores map[string]float64,
	myHash string,
	logs []models.CollectiveInsightLog,
	threshold float64,
	maxUsers int,
) []string {
	type candidate struct {
		hash       string
		similarity float64
	}

	seen := map[string]float64{}
	for _, log := range logs {
		if log.AnonymousUserID == myHash {
			continue // 自分自身は除外
		}
		if log.ScoreSnapshot == "" {
			continue
		}
		var scoreMap map[string]float64
		if err := json.Unmarshal([]byte(log.ScoreSnapshot), &scoreMap); err != nil {
			continue
		}
		// int→float64 変換が必要な場合のフォールバック
		if len(scoreMap) == 0 {
			var intMap map[string]int
			if err := json.Unmarshal([]byte(log.ScoreSnapshot), &intMap); err == nil {
				for k, v := range intMap {
					scoreMap[k] = float64(v)
				}
			}
		}

		sim := cosineSimMap(myScores, scoreMap)
		if sim >= threshold {
			if prev, ok := seen[log.AnonymousUserID]; !ok || sim > prev {
				seen[log.AnonymousUserID] = sim
			}
		}
	}

	candidates := make([]candidate, 0, len(seen))
	for hash, sim := range seen {
		candidates = append(candidates, candidate{hash, sim})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].similarity > candidates[j].similarity
	})

	result := make([]string, 0, maxUsers)
	for i, c := range candidates {
		if i >= maxUsers {
			break
		}
		result = append(result, c.hash)
	}
	return result
}
