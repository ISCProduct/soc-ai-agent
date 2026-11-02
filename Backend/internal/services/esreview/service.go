// Package esreview は ES添削結果の永続化と、前回との差分・スコア連携を扱う（#1531）。
package esreview

import (
	"log"
	"strings"

	"Backend/internal/models"
)

// Repository は ES添削履歴の永続化面。
type Repository interface {
	Create(review *models.ESReview) error
	FindPreviousByQuestion(userID uint, questionKey string, excludeID uint) (*models.ESReview, error)
}

// ScoreUpdater は user_weight_scores への反映面。
type ScoreUpdater interface {
	UpdateScoresFromESReview(userID uint, review *models.ESReview) error
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
		UserID:             in.UserID,
		OrganizationID:     in.OrganizationID,
		Question:           truncateRunes(question, 500),
		QuestionKey:        models.NormalizeESQuestion(question),
		QuestionType:       in.QuestionType,
		ESText:             in.ESText,
		CharLimit:          in.CharLimit,
		SpecificityScore:   clampScore(in.SpecificityScore),
		StarScore:          clampScore(in.StarScore),
		LengthBalanceScore: clampScore(in.LengthBalanceScore),
		Feedback:           in.Feedback,
		ImprovedText:       in.ImprovedText,
	}
	if name := strings.TrimSpace(in.CompanyName); name != "" {
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

	if s.scores != nil {
		if err := s.scores.UpdateScoresFromESReview(in.UserID, review); err != nil {
			log.Printf("[ESReview] スコア反映に失敗 user=%d review=%d: %v", in.UserID, review.ID, err)
		}
	}
	return review, previous, nil
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
