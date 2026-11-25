package company

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/houjinbangou"
)

// 法人番号の補完（SOCAIAGENT-726 の前提）。
//
// EDINET の有価証券報告書は法人番号で企業に紐付ける。staging では法人番号が入っている企業が
// 1割に満たず、このままでは取り込めない。手動のコマンド（cmd/fill-corporate-numbers）と同じ
// 特定処理を、定期取得から毎日少しずつ流して埋める。新しく登録された企業も自動で埋まる。
//
//   - 対象は有効・ゲスト投稿でない・法人番号が空の企業。既に法人番号がある企業は変えない。
//   - 特定は houjinbangou.Client.ResolveCorporateNumber（商号と所在地で一意に決まったときだけ。閉鎖法人は採用しない）。
//   - 1回の上限は CORPORATE_NUMBER_FILL_DAILY_LIMIT（既定300）。国税庁APIは1回/秒なので5分程度で終わる。

const fillCorporateNumbersTarget = "fill_corporate_numbers"

var thirteenDigitsRe = regexp.MustCompile(`^[0-9]{13}$`)

// corporateNumberFinder は houjinbangou.Client が満たす。テストでは偽物に差し替える。
type corporateNumberFinder interface {
	Enabled() bool
	ResolveCorporateNumber(ctx context.Context, name, locationHint string) (*houjinbangou.Corporation, error)
}

// SetCorporateNumberFinder は法人番号の補完に使う国税庁APIのクライアントを注入する。
func (s *CrawlService) SetCorporateNumberFinder(f corporateNumberFinder) { s.numberFinder = f }

func corporateNumberFillLimit() int {
	if v, err := strconv.Atoi(os.Getenv("CORPORATE_NUMBER_FILL_DAILY_LIMIT")); err == nil && v > 0 {
		return v
	}
	return 300
}

// executeFillCorporateNumbers は法人番号が空の企業を上限まで特定して埋める。
func (s *CrawlService) executeFillCorporateNumbers() error {
	if s.numberFinder == nil || !s.numberFinder.Enabled() {
		return fmt.Errorf("法人番号APIが設定されていません（HOUJIN_BANGOU_APP_ID）")
	}
	limit := corporateNumberFillLimit()
	var tried, filled, unresolved, failed int
	err := s.forEachActiveCompanyPage(200, func(c models.Company) error {
		if tried >= limit || c.IsGuestEntry || strings.TrimSpace(c.CorporateNumber) != "" {
			return nil
		}
		tried++
		corp, err := s.numberFinder.ResolveCorporateNumber(context.Background(), c.Name, c.Location)
		if err != nil {
			failed++
			return err
		}
		if corp == nil || corp.IsClosed() || !thirteenDigitsRe.MatchString(corp.CorporateNumber) {
			unresolved++
			return nil
		}
		// 一覧を読んでから時間が経っているので、書く直前に読み直して法人番号だけを埋める。
		cur, err := s.companyRepo.FindByID(c.ID)
		if err != nil || cur == nil {
			failed++
			return fmt.Errorf("企業の再取得に失敗: %v", err)
		}
		if strings.TrimSpace(cur.CorporateNumber) != "" {
			return nil
		}
		cur.CorporateNumber = corp.CorporateNumber
		if err := s.companyRepo.Update(cur); err != nil {
			failed++
			return err
		}
		filled++
		return nil
	}, fillCorporateNumbersTarget)
	log.Printf("%s: tried=%d filled=%d unresolved=%d failed=%d limit=%d", fillCorporateNumbersTarget, tried, filled, unresolved, failed, limit)
	return err
}

// EnsureCorporateNumberFillCrawlSource は法人番号の日次補完ジョブが無ければ作成する。
// 法人番号APIが設定されていない環境では作らない。
func (s *CrawlService) EnsureCorporateNumberFillCrawlSource() {
	if s.numberFinder == nil || !s.numberFinder.Enabled() {
		return
	}
	sources, err := s.repo.ListSources()
	if err != nil {
		log.Printf("ensure %s: list failed: %v", fillCorporateNumbersTarget, err)
		return
	}
	for _, src := range sources {
		if src.TargetType == fillCorporateNumbersTarget {
			return
		}
	}
	source := &models.CrawlSource{
		Name:         "法人番号の日次補完",
		TargetType:   fillCorporateNumbersTarget,
		SourceType:   "manual",
		ScheduleType: "daily",
		ScheduleTime: "04:00", // L1 温存（03:00）の後。EDINET 取り込みより前に埋めておく
		IsActive:     true,
	}
	if err := validateCrawlSource(source); err != nil {
		log.Printf("ensure %s: invalid: %v", fillCorporateNumbersTarget, err)
		return
	}
	source.NextRunAt = computeNextRun(time.Now(), source)
	if err := s.repo.CreateSource(source); err != nil {
		log.Printf("ensure %s: create failed: %v", fillCorporateNumbersTarget, err)
		return
	}
	log.Printf("ensure %s: created id=%d next_run_at=%v", fillCorporateNumbersTarget, source.ID, source.NextRunAt)
}
