// Package ability は能力検査（言語・非言語）の出題と採点（SOCAIAGENT-433）。
//
// 性格検査（services/assessment）と分けている。能力検査の正答率は 10カテゴリ
// （user_weight_scores）へ変換する方法が無く、マッチングに載せない。型を共有しない
// ことで、将来の変更で混入するのを防ぐ。
package ability

import (
	"encoding/json"
	"fmt"

	"Backend/internal/models"
)

// SectionScore は section ごとの正答数と出題数。
//
// 正答率・偏差値・順位は持たない。1桁の分母で割った率や、母集団の無い偏差値は
// 実力の差より揺れの方が大きく出る。学生が自分の数字として受け取る画面なので、
// 数えた事実（何問中何問）だけを返す。
type SectionScore struct {
	Correct int `json:"correct"`
	Total   int `json:"total"`
}

// ScoreAnswers は選択式の回答を section 別の正答数へ変換する。
//
// 採点はここだけで行う（クライアントへ正解を渡さない）。無回答は誤答と同じ扱いで、
// 出題数には数える（受検しなかった問を分母から外すと、飛ばした学生が有利になる）。
// 選択肢の添字の範囲外は誤答として扱う（入口で弾くのは呼び出し側の責務）。
// section が正典外の問があればエラーにする。
func ScoreAnswers(questions []models.AbilityQuestion, answers map[uint]int) (map[string]SectionScore, error) {
	out := map[string]SectionScore{}
	for _, s := range models.AbilitySections {
		out[s] = SectionScore{}
	}
	for _, q := range questions {
		if _, ok := out[q.Section]; !ok {
			return nil, fmt.Errorf("section が正典外です: question=%d section=%q", q.ID, q.Section)
		}
		sc := out[q.Section]
		sc.Total++
		if v, ok := answers[q.ID]; ok && v == q.CorrectChoice {
			sc.Correct++
		}
		out[q.Section] = sc
	}
	// 出題が無い section は返さない（0/0 を見せると「受けたのに全問不正解」と読める）。
	for s, sc := range out {
		if sc.Total == 0 {
			delete(out, s)
		}
	}
	return out, nil
}

// ChoicesOf は問題の選択肢を配列へ戻す。
func ChoicesOf(q models.AbilityQuestion) ([]string, error) {
	var cs []string
	if err := json.Unmarshal([]byte(q.Choices), &cs); err != nil {
		return nil, fmt.Errorf("選択肢の形式が不正です: question=%d: %w", q.ID, err)
	}
	return cs, nil
}

// ValidChoice は選んだ添字が選択肢の範囲内かを返す。
func ValidChoice(q models.AbilityQuestion, choice int) (bool, error) {
	cs, err := ChoicesOf(q)
	if err != nil {
		return false, err
	}
	return choice >= 0 && choice < len(cs), nil
}
