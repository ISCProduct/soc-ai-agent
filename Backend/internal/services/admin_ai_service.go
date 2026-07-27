package services

import (
	"context"
	"time"
)

// AdminAIMetrics はダッシュボードに表示する簡易メトリクス構造体
type AdminAIMetrics struct {
	CollectionCount int       `json:"collection_count"`
	LastUpdated     time.Time `json:"last_updated"`
	CacheHitRate    float64   `json:"cache_hit_rate"`
	EstimatedSaveUSD float64  `json:"estimated_save_usd"`
}

// AdminAIService は管理者向け AI/RAG 運用の軽量サービス
type AdminAIService struct {
	// 将来的に VectorDB クライアントやメトリクスリポジトリを注入
}

func NewAdminAIService() *AdminAIService {
	return &AdminAIService{}
}

// GetSummary は現状のメトリクスダンプを返す（現状はダミー）
func (s *AdminAIService) GetSummary(ctx context.Context) (*AdminAIMetrics, error) {
	return &AdminAIMetrics{
		CollectionCount: 0,
		LastUpdated:     time.Now().UTC(),
		CacheHitRate:    0.0,
		EstimatedSaveUSD: 0.0,
	}, nil
}

// TriggerReembed は再埋め込みをトリガーする（雛形）
func (s *AdminAIService) TriggerReembed(ctx context.Context) error {
	// TODO: enqueue job or call reembed orchestration
	return nil
}

// ForceResync は高コストな再調査（WebSearch/LLM）を強制的に実行する雛形
func (s *AdminAIService) ForceResync(ctx context.Context) error {
	// TODO: implement forced resync logic
	return nil
}
