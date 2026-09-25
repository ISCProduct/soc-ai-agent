package scraper

import (
	"context"
	"fmt"
)

// Pipeline は gBizINFO 公式 API を使った企業データ収集パイプライン。
// 旧実装で使用していた Mynavi・Rikunabi・CareerTasu スクレイパーは利用規約違反リスクのため削除した。
// (#178 スクレイピング法的リスクの解消)
type Pipeline struct {
	GBiz      *GBizClient // gBizINFO 公式APIクライアント（必須）
	Threshold float64     // 現在は未使用（gBizINFO は公式データのため類似度判定不要）
}

// Run はキーワードで gBizINFO を検索し、CompanyNode マップを返す。
// RunRequest.Sites フィールドは COMPANY_GRAPH_URL 経由の外部サービス呼び出し時の後方互換のため残すが、
// 内部 Pipeline では使用しない。
func (p *Pipeline) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	log := &Logger{}
	year := ResolveYear(req.Year)
	log.Logf("Target graduation year: %d", year)
	log.Logf("gBizINFO 検索開始 keyword=%q", req.Query)

	if p.GBiz == nil {
		return &RunResult{
			TargetYear: year,
			Nodes:      map[string]*CompanyNode{},
			Logs:       log.Lines(),
		}, fmt.Errorf("GBizClient が設定されていません (GBIZINFO_API_KEY を確認してください)")
	}

	limit := req.MaxPages * 10
	if limit <= 0 {
		limit = 20
	}

	nodes, gbizLogs, err := p.GBiz.SearchByKeyword(ctx, req.Query, limit)
	for _, l := range gbizLogs {
		log.Logf("%s", l)
	}
	if err != nil {
		log.Logf("gBizINFO 検索エラー: %v", err)
		return &RunResult{
			TargetYear: year,
			Nodes:      map[string]*CompanyNode{},
			Logs:       log.Lines(),
		}, err
	}

	log.Logf("gBizINFO 検索完了: %d 社", len(nodes))

	nodeMap := make(map[string]*CompanyNode, len(nodes))
	for _, node := range nodes {
		nodeMap[node.CorporateNumber] = node
	}

	return &RunResult{
		TargetYear: year,
		Nodes:      nodeMap,
		Logs:       log.Lines(),
	}, nil
}

func countNeedsReview(nodes map[string]*CompanyNode) int {
	n := 0
	for _, node := range nodes {
		if node.NeedsReview {
			n++
		}
	}
	return n
}
