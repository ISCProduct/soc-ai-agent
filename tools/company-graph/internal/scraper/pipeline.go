package scraper

import (
	"context"
	"fmt"
)

// Pipeline orchestrates scraping across all sites and gBizINFO normalisation.
type Pipeline struct {
	Mynavi     *MynaviScraper     // nil if chromedp not configured
	Rikunabi   *RikunabiScraper
	CareerTasu *CareerTasuScraper
	GBiz       *GBizClient
	Threshold  float64 // match score threshold, default 0.75
}

// Run executes the full pipeline: collect → normalize → return.
func (p *Pipeline) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	log := &Logger{}
	year := ResolveYear(req.Year)
	log.Logf("Target graduation year: %d", year)

	if p.Threshold == 0 {
		p.Threshold = 0.75
	}

	// ── Phase 1: collect ────────────────────────────────────────────────
	var rawCompanies []*RawCompany
	var warnings []ScrapeWarning

	for _, site := range req.Sites {
		switch site {
		case "mynavi":
			if p.Mynavi == nil {
				continue
			}
			log.Logf("[mynavi] 検索開始 keyword=%q year=%d pages=%d", req.Query, year, req.MaxPages)
			urls, err := p.Mynavi.Search(req.Query, year, req.MaxPages)
			if err != nil {
				log.Logf("[mynavi] 検索エラー: %v", err)
				warnings = append(warnings, ScrapeWarning{
					Site:    "mynavi",
					Message: fmt.Sprintf("検索失敗（セレクタ v%d）: %v", MynaviSelectorVersion, err),
				})
				continue
			}
			log.Logf("[mynavi] %d 件の企業URLを取得", len(urls))
			if len(urls) == 0 {
				warnings = append(warnings, ScrapeWarning{
					Site: "mynavi",
					Message: fmt.Sprintf(
						"企業URL取得件数が0件（セレクタ v%d）。DOM変更・ログイン要求・アクセス制限の可能性があります",
						MynaviSelectorVersion,
					),
				})
			}
			before := len(rawCompanies)
			emptyName := 0
			for _, u := range urls {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}
				company, err := p.Mynavi.ParseDetail(u)
				if err != nil {
					log.Logf("[mynavi] 詳細取得エラー %s: %v", u, err)
					continue
				}
				if company != nil {
					if company.RawName == "" {
						emptyName++
					}
					rawCompanies = append(rawCompanies, company)
				}
			}
			collected := len(rawCompanies) - before
			log.Logf("[mynavi] %d 件取得完了", collected)
			if collected > 0 && emptyName*2 > collected {
				warnings = append(warnings, ScrapeWarning{
					Site: "mynavi",
					Message: fmt.Sprintf(
						"企業名空欄率が高い (%d/%d)（セレクタ v%d）。DOM変更の可能性があります",
						emptyName, collected, MynaviSelectorVersion,
					),
				})
			}

		case "rikunabi":
			if p.Rikunabi == nil {
				continue
			}
			log.Logf("[rikunabi] 検索開始 keyword=%q pages=%d", req.Query, req.MaxPages)
			urls, err := p.Rikunabi.Search(req.Query, req.MaxPages)
			if err != nil {
				log.Logf("[rikunabi] 検索エラー: %v", err)
				warnings = append(warnings, ScrapeWarning{
					Site:    "rikunabi",
					Message: fmt.Sprintf("検索失敗（セレクタ v%d）: %v", RikunabiSelectorVersion, err),
				})
				continue
			}
			log.Logf("[rikunabi] %d 件の求人URLを取得", len(urls))
			if len(urls) == 0 {
				warnings = append(warnings, ScrapeWarning{
					Site: "rikunabi",
					Message: fmt.Sprintf(
						"求人URL取得件数が0件（セレクタ v%d）。DOM変更・ログイン要求・アクセス制限の可能性があります",
						RikunabiSelectorVersion,
					),
				})
			}
			before := len(rawCompanies)
			emptyName := 0
			for _, u := range urls {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}
				company, err := p.Rikunabi.ParseDetail(u)
				if err != nil {
					log.Logf("[rikunabi] 詳細取得エラー: %v", err)
					continue
				}
				if company != nil {
					if company.RawName == "" {
						emptyName++
					}
					rawCompanies = append(rawCompanies, company)
				}
			}
			collected := len(rawCompanies) - before
			log.Logf("[rikunabi] %d 件取得完了", collected)
			if collected > 0 && emptyName*2 > collected {
				warnings = append(warnings, ScrapeWarning{
					Site: "rikunabi",
					Message: fmt.Sprintf(
						"企業名空欄率が高い (%d/%d)（セレクタ v%d）。DOM変更の可能性があります",
						emptyName, collected, RikunabiSelectorVersion,
					),
				})
			}

		case "career_tasu":
			if p.CareerTasu == nil {
				continue
			}
			log.Logf("[career_tasu] 検索開始 keyword=%q pages=%d", req.Query, req.MaxPages)
			urls, err := p.CareerTasu.Search(req.Query, req.MaxPages)
			if err != nil {
				log.Logf("[career_tasu] 検索エラー: %v", err)
				warnings = append(warnings, ScrapeWarning{
					Site:    "career_tasu",
					Message: fmt.Sprintf("検索失敗（セレクタ v%d）: %v", CareerTasuSelectorVersion, err),
				})
				continue
			}
			log.Logf("[career_tasu] %d 件の企業URLを取得", len(urls))
			if len(urls) == 0 {
				warnings = append(warnings, ScrapeWarning{
					Site: "career_tasu",
					Message: fmt.Sprintf(
						"企業URL取得件数が0件（セレクタ v%d）。DOM変更・ログイン要求・アクセス制限の可能性があります",
						CareerTasuSelectorVersion,
					),
				})
			}
			before := len(rawCompanies)
			emptyName := 0
			for _, u := range urls {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}
				company, err := p.CareerTasu.ParseDetail(u)
				if err != nil {
					log.Logf("[career_tasu] 詳細取得エラー: %v", err)
					continue
				}
				if company != nil {
					if company.RawName == "" {
						emptyName++
					}
					rawCompanies = append(rawCompanies, company)
				}
			}
			collected := len(rawCompanies) - before
			log.Logf("[career_tasu] %d 件取得完了", collected)
			if collected > 0 && emptyName*2 > collected {
				warnings = append(warnings, ScrapeWarning{
					Site: "career_tasu",
					Message: fmt.Sprintf(
						"企業名空欄率が高い (%d/%d)（セレクタ v%d）。DOM変更の可能性があります",
						emptyName, collected, CareerTasuSelectorVersion,
					),
				})
			}

		default:
			log.Logf("不明なサイト: %s (スキップ)", site)
		}
	}

	log.Logf("合計 %d 件の企業を収集", len(rawCompanies))

	// Log warnings to the run log as well
	for _, w := range warnings {
		log.Logf("[WARNING][%s] %s", w.Site, w.Message)
	}

	if len(rawCompanies) == 0 {
		return &RunResult{
			TargetYear: year,
			Nodes:      map[string]*CompanyNode{},
			Logs:       log.Lines(),
			Warnings:   warnings,
		}, fmt.Errorf("no companies collected")
	}

	// ── Phase 2: normalize via gBizINFO ────────────────────────────────
	nodes := map[string]*CompanyNode{}

	if p.GBiz != nil {
		for _, raw := range rawCompanies {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			node, err := p.GBiz.Match(ctx, raw, p.Threshold)
			if err != nil {
				log.Logf("gBizINFO error for %q: %v", raw.RawName, err)
				node = fallbackNode(raw)
			}
			if existing, ok := nodes[node.CorporateNumber]; ok {
				// Merge source URLs
				existing.SourceURLs = append(existing.SourceURLs, node.SourceURLs...)
				if node.MatchScore > existing.MatchScore {
					existing.MatchScore = node.MatchScore
				}
				// 関連会社・事業内容は先着優先でマージ（空の場合のみ上書き）
				mergeRelations(existing, node)
			} else {
				nodes[node.CorporateNumber] = node
			}
		}
	} else {
		// No gBizINFO client: create fallback nodes
		for _, raw := range rawCompanies {
			node := fallbackNode(raw)
			if _, ok := nodes[node.CorporateNumber]; !ok {
				nodes[node.CorporateNumber] = node
			}
		}
	}

	log.Logf("名寄せ完了: %d 社 (needs_review: %d)", len(nodes), countNeedsReview(nodes))

	return &RunResult{
		TargetYear: year,
		Nodes:      nodes,
		Logs:       log.Lines(),
		Warnings:   warnings,
	}, nil
}

// mergeRelations copies relation fields from src into dst when dst fields are empty.
func mergeRelations(dst, src *CompanyNode) {
	if dst.BusinessDescription == "" {
		dst.BusinessDescription = src.BusinessDescription
	}
	if len(dst.RelatedCompanies) == 0 {
		dst.RelatedCompanies = src.RelatedCompanies
	}
	if len(dst.BusinessPartners) == 0 {
		dst.BusinessPartners = src.BusinessPartners
	}
}

func countSite(companies []*RawCompany, site string) int {
	n := 0
	for _, c := range companies {
		if c.SourceSite == site {
			n++
		}
	}
	return n
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
