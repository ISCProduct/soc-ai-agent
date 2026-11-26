package company

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/edinet"
)

// EDINET の有価証券報告書の取り込み（SOCAIAGENT-726）。設計は DesignDoc SOCAIAGENT-722 §3.6。
//
// 毎日、前回処理した日の翌日から昨日までの書類一覧を見て、一般企業の有価証券報告書のうち
// 法人番号が自社の企業と一致するものだけを取り込む。AI は使わない。
//   - 進み具合は crawl_sources.progress_cursor（最後に処理した日付）に持つ。本番は稼働日しか
//     起動しないため、たまった日をまとめて処理する（1回の上限は EDINET_DAYS_PER_RUN、既定31日）。
//   - 初回はさかのぼって過去 edinetBackfillDays 日分を処理する（有報は年1回なので、決算期がいつでも拾える）。
//   - 当日分は提出が続くので処理せず、昨日までで止める。
//   - 429 を受けたらその日の処理を打ち切り、次回その日から再開する。
//   - 同じ企業の新しい期の有報を既に持っていれば、古い期（遅れて出た訂正など）で上書きしない。

const (
	edinetDailyTarget  = "edinet_daily"
	edinetBackfillDays = 456 // 約15か月
)

var jst = time.FixedZone("JST", 9*60*60)

// errEDINETPermanent は取り直しても直らない失敗（書類の中身が読めないなど）。この書類は飛ばして先へ進む。
var errEDINETPermanent = errors.New("edinet: 書類を読み取れません")

// edinetFetcher は edinet.Client が満たす。テストでは偽物に差し替える。
type edinetFetcher interface {
	Enabled() bool
	ListDocuments(ctx context.Context, date time.Time) ([]edinet.Document, error)
	FetchCSVZip(ctx context.Context, docID string) ([]byte, error)
}

// SetEDINETIngest は EDINET の取り込みに使うクライアントと保存先を注入する。
func (s *CrawlService) SetEDINETIngest(f edinetFetcher, research repository.CompanyResearchRepository) {
	s.edinet = f
	s.researchRepo = research
}

func edinetDaysPerRun() int {
	if v, err := strconv.Atoi(os.Getenv("EDINET_DAYS_PER_RUN")); err == nil && v > 0 {
		return v
	}
	return 31
}

func (s *CrawlService) edinetReady() bool {
	return s.edinet != nil && s.edinet.Enabled() && s.researchRepo != nil
}

// executeEDINETDaily は書類一覧を日付順に処理し、source.ProgressCursor を進める（保存は runSource が行う）。
func (s *CrawlService) executeEDINETDaily(source *models.CrawlSource) error {
	if !s.edinetReady() {
		return fmt.Errorf("EDINET の取り込みが設定されていません（EDINET_API_KEY）")
	}
	ctx := context.Background()
	now := time.Now().In(jst)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jst)
	yesterday := today.AddDate(0, 0, -1)

	start := today.AddDate(0, 0, -edinetBackfillDays)
	if source.ProgressCursor != "" {
		last, err := time.ParseInLocation("2006-01-02", source.ProgressCursor, jst)
		if err != nil {
			return fmt.Errorf("progress_cursor が日付ではありません: %q", source.ProgressCursor)
		}
		start = last.AddDate(0, 0, 1)
	}

	maxDays := edinetDaysPerRun()
	var days, matched, ingested, failed int
	for d := start; !d.After(yesterday) && days < maxDays; d = d.AddDate(0, 0, 1) {
		docs, err := s.edinet.ListDocuments(ctx, d)
		if errors.Is(err, edinet.ErrRateLimited) {
			log.Printf("%s: 429 のため %s で打ち切り、次回再開します", edinetDailyTarget, d.Format("2006-01-02"))
			break
		}
		if err != nil {
			return fmt.Errorf("%s の書類一覧: %w", d.Format("2006-01-02"), err)
		}
		stopped := false
		for _, doc := range docs {
			if !doc.IsAnnualReport() {
				continue
			}
			c, err := s.companyRepo.FindByCorporateNumber(doc.JCN)
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				// DB の一時的な障害を「自社に無い会社」として読み飛ばすと、この書類を二度と拾えない。
				log.Printf("%s: 法人番号での検索に失敗: %v", edinetDailyTarget, err)
				stopped = true
				break
			}
			if c == nil || !c.IsActive {
				continue
			}
			matched++
			switch err := s.ingestEDINETDocument(ctx, c, doc); {
			case err == nil:
				ingested++
			case errors.Is(err, errEDINETPermanent):
				// 書類の中身が読めないなど、取り直しても直らない失敗は飛ばして先へ進む。
				failed++
				log.Printf("%s: company=%d doc=%s: %v", edinetDailyTarget, c.ID, doc.DocID, err)
			default:
				// 429・通信・DB など、取り直せば直り得る失敗はこの日を進めずに打ち切る。
				failed++
				log.Printf("%s: company=%d doc=%s: %v", edinetDailyTarget, c.ID, doc.DocID, err)
				stopped = true
			}
			if stopped {
				break
			}
		}
		if stopped {
			// この日は途中までなので進めない（次回この日を最初から処理する。取り込み済みは本文が同じなら書かない）。
			log.Printf("%s: %s の途中で打ち切り、次回この日から再開します", edinetDailyTarget, d.Format("2006-01-02"))
			break
		}
		source.ProgressCursor = d.Format("2006-01-02")
		days++
	}
	log.Printf("%s: days=%d matched=%d ingested=%d failed=%d cursor=%s", edinetDailyTarget, days, matched, ingested, failed, source.ProgressCursor)
	if matched > 0 && failed == matched {
		return fmt.Errorf("%s: 対象 %d 件すべての取り込みに失敗しました", edinetDailyTarget, matched)
	}
	return nil
}

// ingestEDINETDocument は1通の有価証券報告書を取得し、数字と文章を保存する。
func (s *CrawlService) ingestEDINETDocument(ctx context.Context, c *models.Company, doc edinet.Document) error {
	var asOf *time.Time
	if t, err := time.ParseInLocation("2006-01-02", doc.PeriodEnd, jst); err == nil {
		asOf = &t
	}
	if newer, err := s.hasNewerEDINETReport(ctx, c.ID, asOf); err != nil {
		return err
	} else if newer {
		return nil
	}

	zipBytes, err := s.edinet.FetchCSVZip(ctx, doc.DocID)
	if err != nil {
		return err
	}
	rep, err := edinet.ParseAnnualReportZip(zipBytes)
	if err != nil {
		return fmt.Errorf("%w: %v", errEDINETPermanent, err)
	}

	now := time.Now()
	label := "有価証券報告書"
	if asOf != nil {
		label = fmt.Sprintf("有価証券報告書（%d年%d月期）", asOf.Year(), asOf.Month())
	}
	for _, sec := range rep.Sections {
		sum := sha256.Sum256([]byte(sec.Body))
		if _, err := s.researchRepo.UpsertDocument(ctx, &models.CompanyDocument{
			CompanyID:   c.ID,
			SourceType:  models.ResearchSourceEDINET,
			SourceRef:   "annual_report:" + sec.Key, // 企業ごとに最新の1通だけを持つ
			Title:       label + " " + sec.Title,
			URL:         doc.ViewerURL(),
			AsOf:        asOf,
			FetchedAt:   now,
			ContentHash: hex.EncodeToString(sum[:]),
			Body:        sec.Body,
		}); err != nil {
			return err
		}
	}
	facts := make([]models.CompanyFact, 0, len(rep.Facts))
	for _, f := range rep.Facts {
		facts = append(facts, models.CompanyFact{
			CompanyID:  c.ID,
			Field:      f.Field,
			Value:      f.Value,
			Unit:       f.Unit,
			SourceType: models.ResearchSourceEDINET,
			SourceURL:  doc.ViewerURL(),
			AsOf:       asOf,
			FetchedAt:  now,
		})
	}
	return s.researchRepo.UpsertFacts(ctx, facts)
}

// hasNewerEDINETReport は、この企業について asOf より新しい期の有報を既に持っているかを返す。
func (s *CrawlService) hasNewerEDINETReport(ctx context.Context, companyID uint, asOf *time.Time) (bool, error) {
	if asOf == nil {
		return false, nil
	}
	// 文章は本文が同じだと対象年度（as_of）を更新しないので、毎回書き直す数字の対象年度も見る。
	docs, err := s.researchRepo.ListDocuments(ctx, companyID)
	if err != nil {
		return false, err
	}
	for _, d := range docs {
		if d.SourceType == models.ResearchSourceEDINET && d.AsOf != nil && d.AsOf.After(*asOf) {
			return true, nil
		}
	}
	facts, err := s.researchRepo.ListFacts(ctx, companyID)
	if err != nil {
		return false, err
	}
	for _, f := range facts {
		if f.SourceType == models.ResearchSourceEDINET && f.AsOf != nil && f.AsOf.After(*asOf) {
			return true, nil
		}
	}
	return false, nil
}

// EnsureEDINETDailyCrawlSource は EDINET の日次取り込みジョブが無ければ作成する。
// キーが設定されていない環境では作らない。
func (s *CrawlService) EnsureEDINETDailyCrawlSource() {
	if !s.edinetReady() {
		return
	}
	sources, err := s.repo.ListSources()
	if err != nil {
		log.Printf("ensure %s: list failed: %v", edinetDailyTarget, err)
		return
	}
	for _, src := range sources {
		if src.TargetType == edinetDailyTarget {
			return
		}
	}
	source := &models.CrawlSource{
		Name:         "EDINET 有価証券報告書の日次取り込み",
		TargetType:   edinetDailyTarget,
		SourceType:   "official",
		ScheduleType: "daily",
		ScheduleTime: "05:00", // 法人番号の補完（04:00）の後
		IsActive:     true,
	}
	if err := validateCrawlSource(source); err != nil {
		log.Printf("ensure %s: invalid: %v", edinetDailyTarget, err)
		return
	}
	source.NextRunAt = computeNextRun(time.Now(), source)
	if err := s.repo.CreateSource(source); err != nil {
		log.Printf("ensure %s: create failed: %v", edinetDailyTarget, err)
		return
	}
	log.Printf("ensure %s: created id=%d next_run_at=%v", edinetDailyTarget, source.ID, source.NextRunAt)
}
