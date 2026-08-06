package services

import (
	"errors"
	"fmt"
	"strings"

	"Backend/domain/valueobject"

	"gorm.io/gorm"
)

// LocalEvaluationResult はローカル評価の差分を表す（weight_category -> delta）
type LocalEvaluationResult map[string]int

// EvaluateSpeechHeuristic は簡易ルールベースで発話からカテゴリ差分を算出する。
// LLM 非依存の暫定実装。永続化キーは user_weight_scores.weight_category（10カテゴリ）に揃える。
// 注: 4フェーズは analysis progress 側で管理し、スコア行自体はセッション×カテゴリ単位。
func EvaluateSpeechHeuristic(speech string) LocalEvaluationResult {
	low := strings.ToLower(speech)
	res := LocalEvaluationResult{}

	// 10カテゴリ（CompanyWeightProfile / マッチングと同一名称）へのキーワード対応
	keywords := map[string][]string{
		string(valueobject.CategoryTechnical):     {"技術", "プログラミング", "コード", "システム", "エンジニア", "開発"},
		string(valueobject.CategoryTeamwork):      {"チーム", "協力", "一緒に", "メンバー", "連携"},
		string(valueobject.CategoryLeadership):    {"リーダー", "引っ張", "主導", "まとめ", "マネジメント"},
		string(valueobject.CategoryCreativity):    {"アイデア", "創造", "工夫", "発想", "新しい"},
		string(valueobject.CategoryStability):     {"安定", "確実", "計画的", "継続", "コツコツ"},
		string(valueobject.CategoryGrowth):        {"成長", "学び", "学習", "スキルアップ", "向上"},
		string(valueobject.CategoryWorkLife):      {"ワークライフ", "プライベート", "残業", "バランス", "働き方"},
		string(valueobject.CategoryChallenge):     {"挑戦", "チャレンジ", "難しい", "新規", "未知"},
		string(valueobject.CategoryDetail):        {"具体的", "例えば", "詳細", "丁寧", "正確", "実績", "回"},
		string(valueobject.CategoryCommunication): {"わかりやす", "明確", "説明", "伝え", "プレゼン", "対話", "コミュニケーション"},
	}

	for cat, kws := range keywords {
		delta := 0
		for _, kw := range kws {
			if strings.Contains(low, strings.ToLower(kw)) {
				delta += 10
			}
		}
		if delta != 0 {
			res[cat] = delta
		}
	}
	return res
}

// EvaluateAndPersist 発話を評価して user_weight_scores リポジトリへ反映する。
// 無ければ SetScore、既存なら AddScore を使って差分を適用する。
func (s *InterviewService) EvaluateAndPersist(userID uint, sessionID string, speech string) error {
	if s.userWeightScoreRepo == nil {
		// リポジトリ未注入の場合は noop
		return nil
	}
	deltas := EvaluateSpeechHeuristic(speech)
	for cat, delta := range deltas {
		if delta == 0 {
			continue
		}
		// 既存スコアの有無を確認
		_, err := s.userWeightScoreRepo.FindByUserSessionAndCategory(userID, sessionID, cat)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("find score failed: %w", err)
			}
			// 未登録なら基準点50に差分を加えた絶対値で新規作成
			abs := clampInt(50+delta, 0, 100)
			if err2 := s.userWeightScoreRepo.SetScore(userID, sessionID, cat, abs); err2 != nil {
				return fmt.Errorf("set score failed: %w", err2)
			}
			continue
		}
		if err2 := s.userWeightScoreRepo.AddScore(userID, sessionID, cat, delta); err2 != nil {
			return fmt.Errorf("add score failed: %w", err2)
		}
	}
	return nil
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
