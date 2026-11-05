package assessment

import (
	"fmt"

	"Backend/domain/valueobject"
	"Backend/internal/models"
)

// LikertMin / LikertMax はリッカート5択の範囲（1=あてはまらない … 5=あてはまる）。
const (
	LikertMin = 1
	LikertMax = 5
)

// ChatWeight はチャット由来スコアと検査スコアを合成するときのチャット側の比率（#1678）。
//
// 暫定値で根拠は無い。score_calibration_weights と同じく、選考通過率との相関を見てから
// 調整する。それまでは固定値として運用する（docs/wiki に明記）。
const ChatWeight = 0.5

// ScoreAnswers はリッカート回答をカテゴリ別 0〜100 に変換する。
//
// 1問の得点 = (値-1)×25（1→0, 5→100）。逆転項目は 6-値 にしてから換算する。
// カテゴリの得点はそのカテゴリに答えた問の平均。答えの無いカテゴリは結果に含めない
// （0点で埋めると「その傾向が全く無い」と誤読され、マッチングを歪める）。
// 正典外のカテゴリを持つ問があればエラーにする（user_weight_scores へ流さない）。
func ScoreAnswers(questions []models.AssessmentQuestion, answers map[uint]int) (map[string]float64, error) {
	sum := map[string]float64{}
	n := map[string]int{}
	for _, q := range questions {
		v, ok := answers[q.ID]
		if !ok {
			continue
		}
		if v < LikertMin || v > LikertMax {
			return nil, fmt.Errorf("回答値が範囲外です: question=%d value=%d", q.ID, v)
		}
		cat, err := valueobject.ParseWeightCategory(q.WeightCategory)
		if err != nil {
			return nil, err
		}
		if q.ReverseScored {
			v = LikertMin + LikertMax - v
		}
		sum[string(cat)] += float64(v-LikertMin) * 100 / float64(LikertMax-LikertMin)
		n[string(cat)]++
	}
	out := make(map[string]float64, len(sum))
	for c, s := range sum {
		out[c] = s / float64(n[c])
	}
	return out, nil
}

// MergeScores はチャットと検査のカテゴリ別スコアを chatWeight : (1-chatWeight) で合成する。
//
// 片側にしか無いカテゴリはある側の値をそのまま使い、どちらにも無いカテゴリは出さない。
// 欠損を0や中立50で埋めると、受検していない側の「無回答」がスコアを引き下げる。
func MergeScores(chat, assessment map[string]float64, chatWeight float64) map[string]float64 {
	out := make(map[string]float64, len(chat)+len(assessment))
	for c, v := range chat {
		out[c] = v
	}
	for c, a := range assessment {
		if v, ok := chat[c]; ok {
			out[c] = v*chatWeight + a*(1-chatWeight)
		} else {
			out[c] = a
		}
	}
	return out
}
