package models

import "time"

// UserWeightScoreResponse は API 応答に出すスコアの形。
//
// json タグを付けないので、キーは Go のフィールド名（ID / UserID / WeightCategory …）になる。
// これは従来の応答の形で、api/openapi.yaml の UserWeightScore もこのキーで書いてある。
// UserWeightScore 自体には json タグ（id / user_id / category …）があり、再計算の控えの
// 保存にも使うので、応答には使わずこの型へ変換して返す。OrganizationID と Source は出さない。
type UserWeightScoreResponse struct {
	ID                 uint
	UserID             uint
	SessionID          string
	WeightCategory     string
	Score              int
	CalculationVersion string
	PromptVersion      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewUserWeightScoreResponses は応答用の形に変換する。nil は nil のまま（JSON では null）、
// 空は空のまま（JSON では []）にして、従来の応答と同じ形を保つ。
func NewUserWeightScoreResponses(scores []UserWeightScore) []UserWeightScoreResponse {
	if scores == nil {
		return nil
	}
	out := make([]UserWeightScoreResponse, len(scores))
	for i, s := range scores {
		out[i] = UserWeightScoreResponse{
			ID:                 s.ID,
			UserID:             s.UserID,
			SessionID:          s.SessionID,
			WeightCategory:     s.WeightCategory,
			Score:              s.Score,
			CalculationVersion: s.CalculationVersion,
			PromptVersion:      s.PromptVersion,
			CreatedAt:          s.CreatedAt,
			UpdatedAt:          s.UpdatedAt,
		}
	}
	return out
}
