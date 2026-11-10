package models

import "time"

// ResumeLatestFact は教員一覧向けの「最新履歴書＋レビュー」要約（#1030）。
// ドキュメント無しのユーザーは map に載らない（呼び出し側で未提出扱い）。
type ResumeLatestFact struct {
	HasDocument bool
	LatestScore *int // レビュー未生成なら nil
}

type ResumeDocument struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	OrganizationID   uint      `gorm:"not null;index;column:organization_id" json:"organization_id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	SessionID        string    `gorm:"size:100;index" json:"session_id"`
	SourceType       string    `gorm:"size:50;not null" json:"source_type"` // pdf, docx, google_docs
	SourceURL        string    `gorm:"type:text" json:"source_url,omitempty"`
	OriginalFilename string    `gorm:"size:255" json:"original_filename"`
	StoredPath       string    `gorm:"type:text;not null" json:"stored_path"`
	NormalizedPath   string    `gorm:"type:text" json:"normalized_path,omitempty"`
	AnnotatedPath    string    `gorm:"type:text" json:"annotated_path,omitempty"`
	Status           string    `gorm:"size:50;default:'uploaded'" json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ResumeTextBlock struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	DocumentID uint   `gorm:"not null;index" json:"document_id"`
	PageNumber int    `gorm:"not null" json:"page_number"`
	BlockIndex int    `gorm:"not null" json:"block_index"`
	Text       string `gorm:"type:text;not null" json:"text"`
	BBox       string `gorm:"type:json;not null" json:"bbox"`
	// Confidence は PaddleOCR の行認識スコア（0〜1）。
	// NULL は信頼度不明（#1942 以前の行、平文入力）。不明な行は書き換えを禁止しない。
	// 下限未満の行は誤認識として、書き換え案を出さない。
	Confidence *float64  `gorm:"column:confidence" json:"confidence,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ResumeReview struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	DocumentID uint `gorm:"not null;index" json:"document_id"`
	// Score は総合スコア(0-100)。ルーブリックの項目スコアからサーバー側で算出する。
	// 生成に失敗した・ルーブリック違反だったときは nil（スコア無し）。
	// 固定値70を入れない。良かった70と失敗した70が区別できなくなる（#1529）。
	Score *int `gorm:"column:score" json:"score"`
	// ItemScoresJSON は項目別スコア(0-5)のJSON。#1529 以前の行とスコア無しの行は nil。
	// 画面は nil を「内訳なし」に倒す。
	ItemScoresJSON *string   `gorm:"type:json;column:item_scores_json" json:"item_scores_json,omitempty"`
	Summary        string    `gorm:"type:text" json:"summary"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ResumeReviewItem struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ReviewID   uint   `gorm:"not null;index" json:"review_id"`
	PageNumber int    `gorm:"not null" json:"page_number"`
	BBox       string `gorm:"type:json;not null" json:"bbox"`
	Severity   string `gorm:"size:20;not null" json:"severity"` // info, warning, critical
	Message    string `gorm:"type:text;not null" json:"message"`
	// Quote は指摘が指している原文。注釈PDFと画面の「原文」に使う。
	Quote      string `gorm:"type:text" json:"quote,omitempty"`
	Suggestion string `gorm:"type:text" json:"suggestion,omitempty"`
	// Rewrite は Quote をそのまま置き換えられる一文。説明文は Suggestion に残す。
	// 空は「書き換え案なし」（読み取り不確か、または #1942 以前の行）。
	Rewrite string `gorm:"type:text" json:"rewrite,omitempty"`
	// RewriteWithheld は Rewrite を空にした理由。low_ocr_confidence のとき画面は
	// 「読み取りが不確かだから出していない」と出す。空は理由なし。
	RewriteWithheld string    `gorm:"size:32" json:"rewrite_withheld,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
