package hr

import (
	"context"
	"log"
	"strings"
)

// scoutProfileReader は公開可否と公開テキストの取得（repositories.UserPreferenceRepository）。
// 同意フラグの生値ではなく実効的な公開可否を見るため IsScoutVisible を使う
// （退会済みユーザーを再インデックスしないため）。
type scoutProfileReader interface {
	IsScoutVisible(userID uint) (bool, error)
	ScoutProfileText(userID uint) (string, error)
}

// studentVectorIndexer は学生プロフィールのベクトル登録・削除（RAG）。
type studentVectorIndexer interface {
	Index(ctx context.Context, userID uint, text string) error
	Delete(ctx context.Context, userID uint) error
}

// StudentIndexSyncer は学生プロフィールのベクトルを最新状態へ同期する（#1094）。
//
// 同期契機は「公開テキストの元になるデータが変わったとき」で、具体的には
// 希望条件の保存（/api/user/preferences）とプロフィール更新（/api/auth/profile）の両方。
// 資格情報はプロフィール側で更新されるため、片方だけに置くとインデックスが古くなる。
type StudentIndexSyncer struct {
	profiles scoutProfileReader
	indexer  studentVectorIndexer
}

func NewStudentIndexSyncer(profiles scoutProfileReader, indexer studentVectorIndexer) *StudentIndexSyncer {
	return &StudentIndexSyncer{profiles: profiles, indexer: indexer}
}

// Sync は同意状態に合わせてベクトルを登録または削除する。
// RAGが落ちていても呼び出し元の保存処理は成功させたいので、失敗はログのみに留める。
func (s *StudentIndexSyncer) Sync(ctx context.Context, userID uint) {
	if s == nil || s.indexer == nil || s.profiles == nil {
		return
	}
	allow, err := s.profiles.IsScoutVisible(userID)
	if err != nil {
		log.Printf("[WARN] scout visibility lookup failed user_id=%d: %v", userID, err)
		return
	}
	if !allow {
		s.delete(ctx, userID)
		return
	}
	text, err := s.profiles.ScoutProfileText(userID)
	if err != nil {
		log.Printf("[WARN] student profile text build failed user_id=%d: %v", userID, err)
		return
	}
	if strings.TrimSpace(text) == "" {
		// 公開できる情報が無い状態でベクトルだけ残らないよう削除する。
		s.delete(ctx, userID)
		return
	}
	if err := s.indexer.Index(ctx, userID, text); err != nil {
		log.Printf("[WARN] student vector index failed user_id=%d: %v", userID, err)
	}
}

func (s *StudentIndexSyncer) delete(ctx context.Context, userID uint) {
	if err := s.indexer.Delete(ctx, userID); err != nil {
		log.Printf("[WARN] student vector delete failed user_id=%d: %v", userID, err)
	}
}
