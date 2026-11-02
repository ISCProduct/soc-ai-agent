// Package esreview は ES添削結果の永続化と、前回との差分・スコア連携を扱う（#1531）。
package esreview

import (
	"log"
	"strings"
	"time"

	"Backend/internal/models"
)

// Repository は ES添削履歴の永続化面。
type Repository interface {
	Create(review *models.ESReview) error
	FindPreviousByQuestion(userID uint, questionKey string, excludeID uint) (*models.ESReview, error)
	// HasScoredSameText は本人が同じ本文で既にスコアへ反映済みか。
	HasScoredSameText(userID uint, textHash string, excludeID uint) (bool, error)
	// LastScoredAt は本人が直近で反映した時刻。まだ無ければ nil。
	LastScoredAt(userID, excludeID uint) (*time.Time, error)
	// MarkScored は反映したことを記録する。
	MarkScored(id uint, at time.Time) error
}

// scoreInterval は ES添削がスコアを動かせる間隔。
//
// 本文の正規化をどれだけ強めても、一語でも書き換えられれば別の本文になる。
// 内容の判定だけでは積み上げを止められないので、反映の頻度そのものに上限を置く。
// 移動平均（既存0.7/新規0.3）で上限へ寄せるには10回ほど要るため、1日1回なら
// 意図的に積んでも2週間近くかかり、正当な推敲（1日1回でも十分に反映される）は妨げない。
const scoreInterval = 24 * time.Hour

// ScoreUpdater は user_weight_scores への反映面。
type ScoreUpdater interface {
	// 戻り値の bool は「実際に反映したか」。診断セッションが無いなどで反映しなかった
	// 場合に false を返す。true のときだけ反映済みとして記録する。
	UpdateScoresFromESReview(userID uint, review *models.ESReview) (bool, error)
}

// Input は1回の添削の入力と結果。コントローラが RAG の応答から詰める。
type Input struct {
	UserID         uint
	OrganizationID uint
	Question       string
	QuestionType   string
	CompanyName    string
	ESText         string
	CharLimit      *uint
	CharLimitMode  string

	SpecificityScore   int
	StarScore          int
	LengthBalanceScore int
	CompanyFitScore    *int

	Feedback     string
	ImprovedText string
}

type Service struct {
	repo   Repository
	scores ScoreUpdater
}

func NewService(repo Repository, scores ScoreUpdater) *Service {
	return &Service{repo: repo, scores: scores}
}

// Save は添削結果を保存し、同一設問の前回結果を返す（無ければ nil）。
//
// 保存に失敗しても添削そのものは成立しているので、呼び出し元はエラーを握って
// 結果を返してよい。履歴が残らないことと、添削が使えないことでは後者の方が重い。
// ただし黙って落とすと気づけないため、ここでログに残す。
func (s *Service) Save(in Input) (*models.ESReview, *models.ESReview, error) {
	question := strings.TrimSpace(in.Question)
	review := &models.ESReview{
		UserID:         in.UserID,
		OrganizationID: in.OrganizationID,
		Question:       truncateRunes(question, 500),
		QuestionKey:    models.NormalizeESQuestion(question),
		// 列長を超えると INSERT が落ち、添削は返るのに履歴だけ黙って残らない。
		// 利用者の入力で履歴機能だけが死ぬ経路を作らない。
		QuestionType:       truncateRunes(in.QuestionType, 50),
		ESText:             in.ESText,
		ESTextHash:         models.HashESText(in.ESText),
		CharLimit:          in.CharLimit,
		SpecificityScore:   clampScore(in.SpecificityScore),
		StarScore:          clampScore(in.StarScore),
		LengthBalanceScore: clampScore(in.LengthBalanceScore),
		Feedback:           in.Feedback,
		ImprovedText:       in.ImprovedText,
	}
	if name := truncateRunes(strings.TrimSpace(in.CompanyName), 255); name != "" {
		review.CompanyName = &name
	}
	if mode := strings.TrimSpace(in.CharLimitMode); mode != "" {
		review.CharLimitMode = &mode
	}
	if in.CompanyFitScore != nil {
		v := clampScore(*in.CompanyFitScore)
		review.CompanyFitScore = &v
	}

	if err := s.repo.Create(review); err != nil {
		return nil, nil, err
	}

	// 前回は保存の後に引く。先に引くと、同一設問を続けて出したときに
	// 自分自身が「前回」になって差分が常にゼロになる。
	var previous *models.ESReview
	if review.QuestionKey != "" {
		p, err := s.repo.FindPreviousByQuestion(in.UserID, review.QuestionKey, review.ID)
		if err != nil {
			log.Printf("[ESReview] 前回結果の取得に失敗 user=%d review=%d: %v", in.UserID, review.ID, err)
		} else {
			previous = p
		}
	}

	// スコアへ反映してよいかを2段で判定する（#1531）。
	//
	// 移動平均（既存0.7/新規0.3）なので、ESを繰り返し出すだけで対象カテゴリを
	// 上限へ寄せられる。ES添削はテキストボックスで何度でも押せるため、
	// 履歴書（PDFアップロードが要る）より摩擦がない。
	//
	//  1. 同じ本文で既に反映済みなら動かさない（同じ文章の出し直しに新しい情報は無い）
	//  2. 直近の反映から scoreInterval 以内なら動かさない
	//
	// 2 が要るのは、1 だけでは止まらないから。本文を正規化しても一語書き換えれば
	// 別の本文になる。実際に、不可視文字を1つ足す・全角半角を変える・NFD にするだけで
	// 何度でも積める経路がテストで再現した。内容の判定は「同じ文章」を弾く層にとどめ、
	// 積み上げの速度は時間で縛る。
	if s.scores != nil && s.shouldScore(in.UserID, review) {
		applied, err := s.scores.UpdateScoresFromESReview(in.UserID, review)
		switch {
		case err != nil:
			log.Printf("[ESReview] スコア反映に失敗 user=%d review=%d: %v", in.UserID, review.ID, err)
		case !applied:
			// 反映しなかった行を「反映済み」にすると、その本文が以後永久に乗らない。
			// 診断セッションができてから同じ本文を出し直せば反映される。
		default:
			if err := s.repo.MarkScored(review.ID, time.Now()); err != nil {
				// 記録できないと次回も反映できてしまう。気づけるようにログへ残す。
				log.Printf("[ESReview] 反映時刻の記録に失敗 user=%d review=%d: %v", in.UserID, review.ID, err)
			}
		}
	}
	return review, previous, nil
}

// shouldScore はこの添削をスコアへ反映してよいかを返す。
// 判定できないときは false（二重に積む方が戻せない）。
func (s *Service) shouldScore(userID uint, review *models.ESReview) bool {
	scored, err := s.repo.HasScoredSameText(userID, review.ESTextHash, review.ID)
	if err != nil {
		log.Printf("[ESReview] 本文の重複判定に失敗 user=%d review=%d: %v", userID, review.ID, err)
		return false
	}
	if scored {
		return false
	}
	last, err := s.repo.LastScoredAt(userID, review.ID)
	if err != nil {
		log.Printf("[ESReview] 直近の反映時刻の取得に失敗 user=%d review=%d: %v", userID, review.ID, err)
		return false
	}
	if last != nil && time.Since(*last) < scoreInterval {
		return false
	}
	return true
}

// clampScore は RAG の 1〜10 評価を範囲内に収める。
// 範囲外は RAG 側の異常なので、丸めた値で保存して記録自体は残す。
func clampScore(v int) uint8 {
	if v < 1 {
		return 1
	}
	if v > 10 {
		return 10
	}
	return uint8(v)
}

// truncateRunes は多バイト文字の途中で切らずに丸める。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
