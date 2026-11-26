package company

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/ragclient"
	"Backend/internal/usagectx"
)

// 企業研究AIの文章を RAG に登録する（SOCAIAGENT-725）。
//
// company_documents のうち未登録（indexed_at が NULL）のものを企業ごとにまとめて RAG へ送り、
// 登録できたら indexed_at を入れる。本文が変わると UpsertDocument が indexed_at を NULL に戻すので、
// ここで拾い直される。RAG が落ちていても文章は MySQL に残り、次回登録する。
//   - COMPANY_RESEARCH_INGEST_ENABLED=true のときだけジョブを作る（設計の既定はオフ）。
//   - 1回に登録する文章は researchIndexBatch 件まで。ベクトル化の費用は api_call_logs に記録する。

const (
	researchIndexTarget  = "company_research_index"
	researchIndexBatch   = 500
	researchIndexPerCall = 20 // RAG 側の1リクエストあたりの上限と同じ
	featureResearchIndex = "company_research_index"
)

// researchIndexer は ragclient.ResearchClient が満たす。テストでは偽物に差し替える。
type researchIndexer interface {
	Enabled() bool
	Index(ctx context.Context, companyID uint, docs []ragclient.ResearchDocument) (ragclient.Usage, error)
}

// SetResearchIndexer は RAG への登録に使うクライアントを注入する。
func (s *CrawlService) SetResearchIndexer(r researchIndexer) { s.indexer = r }

func researchIngestEnabled() bool { return os.Getenv("COMPANY_RESEARCH_INGEST_ENABLED") == "true" }

func (s *CrawlService) executeResearchIndex() error {
	if s.indexer == nil || !s.indexer.Enabled() || s.researchRepo == nil {
		return fmt.Errorf("RAG への登録が設定されていません（RAG_REVIEW_URL）")
	}
	ctx := context.Background()
	docs, err := s.researchRepo.ListDocumentsToIndex(ctx, researchIndexBatch)
	if err != nil {
		return err
	}
	var indexed, failed int
	for start := 0; start < len(docs); {
		// 同じ企業の文章を researchIndexPerCall 件ずつまとめる（ListDocumentsToIndex は企業順に並ぶ）
		end := start + 1
		for end < len(docs) && docs[end].CompanyID == docs[start].CompanyID && end-start < researchIndexPerCall {
			end++
		}
		batch := docs[start:end]
		start = end

		payload := make([]ragclient.ResearchDocument, 0, len(batch))
		ids := make([]uint, 0, len(batch))
		for _, d := range batch {
			payload = append(payload, toResearchDocument(d))
			ids = append(ids, d.ID)
		}
		began := time.Now()
		usage, err := s.indexer.Index(ctx, batch[0].CompanyID, payload)
		if err != nil {
			failed += len(batch)
			log.Printf("%s: company=%d: %v", researchIndexTarget, batch[0].CompanyID, err)
			continue
		}
		s.reportRAGUsage(usagectx.WithFeature(ctx, featureResearchIndex), usage, time.Since(began))
		if err := s.researchRepo.MarkIndexed(ctx, ids, time.Now()); err != nil {
			return err
		}
		indexed += len(batch)
	}
	log.Printf("%s: indexed=%d failed=%d", researchIndexTarget, indexed, failed)
	if len(docs) > 0 && indexed == 0 {
		return fmt.Errorf("%s: %d 件すべての登録に失敗しました", researchIndexTarget, len(docs))
	}
	return nil
}

func toResearchDocument(d models.CompanyDocument) ragclient.ResearchDocument {
	asOf := ""
	if d.AsOf != nil {
		asOf = d.AsOf.Format("2006-01-02")
	}
	return ragclient.ResearchDocument{DocumentID: d.ID, SourceType: d.SourceType, Title: d.Title, URL: d.URL, AsOf: asOf, Body: d.Body}
}

// reportRAGUsage は RAG が実行したベクトル化の使用量を記録する（RAG は api_call_logs を持たない）。
func (s *CrawlService) reportRAGUsage(ctx context.Context, u ragclient.Usage, latency time.Duration) {
	if u.Calls == 0 || s.aiClient == nil {
		return
	}
	s.aiClient.ReportProxyUsage(ctx, openai.ProxyUsage{Model: u.Model, PromptTokens: u.PromptTokens, Latency: latency})
}

// EnsureResearchIndexCrawlSource は RAG への登録ジョブが無ければ作成する。
func (s *CrawlService) EnsureResearchIndexCrawlSource() {
	if !researchIngestEnabled() || s.indexer == nil || !s.indexer.Enabled() || s.researchRepo == nil {
		return
	}
	sources, err := s.repo.ListSources()
	if err != nil {
		log.Printf("ensure %s: list failed: %v", researchIndexTarget, err)
		return
	}
	for _, src := range sources {
		if src.TargetType == researchIndexTarget {
			return
		}
	}
	source := &models.CrawlSource{
		Name:         "企業研究AIの文章を RAG に登録",
		TargetType:   researchIndexTarget,
		SourceType:   "manual",
		ScheduleType: "daily",
		ScheduleTime: "05:30", // EDINET の取り込み（05:00）の後
		IsActive:     true,
	}
	if err := validateCrawlSource(source); err != nil {
		log.Printf("ensure %s: invalid: %v", researchIndexTarget, err)
		return
	}
	source.NextRunAt = computeNextRun(time.Now(), source)
	if err := s.repo.CreateSource(source); err != nil {
		log.Printf("ensure %s: create failed: %v", researchIndexTarget, err)
		return
	}
	log.Printf("ensure %s: created id=%d next_run_at=%v", researchIndexTarget, source.ID, source.NextRunAt)
}
