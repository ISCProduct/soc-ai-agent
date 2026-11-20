// import-market-info は、公開済みの企業の上場区分（company_market_info）を、無料の公的データから埋める。
//
//   - EDINET のコードリスト（金融庁。キー不要）: 上場区分・証券コード・提出者法人番号
//   - 東証の上場会社一覧（日本取引所グループ。data_j.xlsx）: 市場・商品区分（プライム/スタンダード/グロース）
//
// 既定は確認のみで、DB には書き込まない。
//
//	cd Backend
//	go run ./cmd/import-market-info                                       # 確認のみ（ファイルは URL から取得）
//	go run ./cmd/import-market-info --edinet-zip a.zip --jpx-xlsx b.xlsx  # ローカルのファイルを使う
//	go run ./cmd/import-market-info --apply --backup before.json          # 書き込む
//	go run ./cmd/import-market-info --restore before.json                 # 書き込む前に戻す
//
// 突き合わせの規則（正しさ優先。名前だけの一致は使わない）:
//   - 企業の corporate_number と EDINET の提出者法人番号が完全一致したものだけが対象。
//   - 「上場」→ is_listed=true。証券コード先頭4桁が東証の一覧にあれば市場区分も入れる（無ければ market_type は空。断定しない）。
//   - 「非上場」→ is_listed=false, market_type=unlisted。
//   - 上場区分が空、または EDINET に無い企業は、何も書かない（未確認のまま）。
//   - 既に市場情報がある企業は変更しない。--overwrite のときだけ、差分がある企業を更新する。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"Backend/internal/models"
)

const (
	edinetURL        = "https://disclosure2dl.edinet-fsa.go.jp/searchdocument/codelist/Edinetcode.zip"
	jpxPage          = "https://www.jpx.co.jp/markets/statistics-equities/misc/01.html"
	userAgent        = "soc-ai-agent-import-market-info/1.0 (+https://github.com/ISCProduct/soc-ai-agent)"
	maxDownloadBytes = 100 << 20
)

// backupRow は、書き込んだ企業（復元に使う）。Before が nil のときは、書き込む前は市場情報の行が無かった。
type backupRow struct {
	CompanyID uint         `json:"company_id"`
	Before    *marketValue `json:"before"`
	After     marketValue  `json:"after"`
}

func main() {
	edinetZip := flag.String("edinet-zip", "", "EDINET の Edinetcode.zip のパス（無ければ URL から取得）")
	jpxXLSX := flag.String("jpx-xlsx", "", "東証の data_j.xlsx のパス（無ければ配布ページから取得）")
	apply := flag.Bool("apply", false, "DB に書き込む。無ければ確認のみ")
	backup := flag.String("backup", "", "書き込む前の値の保存先（--apply で必須。既存のファイルは上書きしない）")
	restore := flag.String("restore", "", "バックアップの企業を、書き込む前の状態に戻す")
	overwrite := flag.Bool("overwrite", false, "既に市場情報がある企業も、差分があれば更新する")
	allowProd := flag.Bool("allow-production", false, "本番・localhost 以外への書き込みを許す")
	flag.Parse()

	loadEnv()
	if *apply || *restore != "" {
		if err := writeAllowed(*allowProd, dbHost(), os.Getenv("APP_ENV")); err != nil {
			log.Fatal(err)
		}
	}
	if *apply && *backup == "" {
		log.Fatal("--apply には --backup が必要です（元に戻せるように）")
	}
	db, err := connectDB()
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	fmt.Printf("接続先: host=%s db=%s\n", dbHost(), os.Getenv("DB_NAME"))
	if *restore != "" {
		runRestore(db, *restore)
		return
	}

	ctx := context.Background()
	edinetRaw, err := loadFile(ctx, *edinetZip, func(ctx context.Context) ([]byte, error) { return download(ctx, edinetURL) })
	if err != nil {
		log.Fatalf("EDINET コードリストの取得に失敗: %v", err)
	}
	edinet, err := parseEDINETZip(edinetRaw)
	if err != nil {
		log.Fatalf("EDINET コードリストの解析に失敗: %v", err)
	}
	jpxRaw, err := loadFile(ctx, *jpxXLSX, fetchJPX)
	if err != nil {
		log.Fatalf("東証の上場会社一覧の取得に失敗: %v", err)
	}
	jpx, err := parseJPXXLSX(jpxRaw)
	if err != nil {
		log.Fatalf("東証の上場会社一覧の解析に失敗: %v", err)
	}
	fmt.Printf("EDINET: %d 法人 / 東証一覧: %d 銘柄\n", len(edinet), len(jpx))

	refs, err := loadCompanies(db)
	if err != nil {
		log.Fatalf("対象の取得に失敗: %v", err)
	}
	decisions := make([]decision, 0, len(refs))
	for _, c := range refs {
		decisions = append(decisions, decide(c, edinet, jpx))
	}
	writes := printReport(decisions, *overwrite)

	if !*apply {
		fmt.Println("\n確認のみです（DB には書き込んでいません）。書き込むには --apply --backup <ファイル> を付けてください。")
		return
	}
	if len(writes) == 0 {
		fmt.Println("\n書き込む企業がありません。")
		return
	}
	if err := writeBackup(*backup, writes); err != nil {
		log.Fatalf("バックアップに失敗: %v", err)
	}
	if err := applyWrites(db, writes); err != nil {
		log.Fatalf("書き込みに失敗（全件ロールバック済み）: %v", err)
	}
	fmt.Printf("\n書き込みました: %d 社（バックアップ: %s）\n元に戻す: go run ./cmd/import-market-info --restore %s\n", len(writes), *backup, *backup)
}

// printReport は結果の内訳と出典の注意を出し、書き込む対象を返す。
func printReport(ds []decision, overwrite bool) []backupRow {
	var writes []backupRow
	counts := map[string]int{}
	var diffs []decision
	for _, d := range ds {
		counts[d.Kind]++
		switch d.Kind {
		case kindNew:
			writes = append(writes, backupRow{CompanyID: d.CompanyID, After: *d.After})
		case kindDiff:
			diffs = append(diffs, d)
			if overwrite {
				writes = append(writes, backupRow{CompanyID: d.CompanyID, Before: d.Before, After: *d.After})
			}
		}
	}
	fmt.Printf("\n結果（対象 %d 社）: 新規 %d / 変更なし %d / 未確認 %d / 差分 %d\n", len(ds), counts[kindNew], counts[kindSame], counts[kindUnchecked], counts[kindDiff])
	if len(diffs) > 0 {
		verb := "（--overwrite が無いので更新しません）"
		if overwrite {
			verb = "（--overwrite により更新します）"
		}
		fmt.Printf("\n既存の値と違う企業 %s:\n", verb)
		for _, d := range diffs {
			fmt.Printf("  %-5d %s: %s → %s\n", d.CompanyID, d.Name, fmtValue(d.Before), fmtValue(d.After))
		}
	}
	fmt.Println("\n【出典の表示について】取り込んだ値を画面に出す場合は、「出典: EDINET（金融庁）、日本取引所グループ」の表示が必要な可能性があります。利用規約の確認は人が行ってください。")
	return writes
}

func fmtValue(v *marketValue) string {
	if v == nil {
		return "（なし）"
	}
	return fmt.Sprintf("listed=%t type=%q code=%q", v.IsListed, v.MarketType, v.StockCode)
}

// loadCompanies は公開済み・有効な企業と、その市場情報を返す。
func loadCompanies(db *gorm.DB) ([]companyRef, error) {
	var cs []models.Company
	if err := db.Where("data_status = ? AND is_active = ?", "published", true).Order("id").Find(&cs).Error; err != nil {
		return nil, err
	}
	var infos []models.CompanyMarketInfo
	if err := db.Find(&infos).Error; err != nil {
		return nil, err
	}
	byID := map[uint]marketValue{}
	for _, i := range infos {
		byID[i.CompanyID] = marketValue{MarketType: i.MarketType, IsListed: i.IsListed, StockCode: i.StockCode}
	}
	refs := make([]companyRef, 0, len(cs))
	for _, c := range cs {
		ref := companyRef{ID: c.ID, Name: c.Name, CorporateNumber: c.CorporateNumber}
		// 行があっても中身が空（既定値のまま）なら「市場情報なし」として扱う
		if v, ok := byID[c.ID]; ok && (v.MarketType != "" || v.IsListed || v.StockCode != "") {
			ref.Existing = &v
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// applyWrites は、全件を 1 トランザクションで書く。
func applyWrites(db *gorm.DB, rows []backupRow) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			if err := upsert(tx, r.CompanyID, r.After); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsert(tx *gorm.DB, companyID uint, v marketValue) error {
	info := models.CompanyMarketInfo{CompanyID: companyID, MarketType: v.MarketType, IsListed: v.IsListed, StockCode: v.StockCode}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"market_type", "is_listed", "stock_code", "updated_at"}),
	}).Create(&info).Error
}

func writeBackup(path string, rows []backupRow) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- 利用者がコマンドラインで指定したパス
	if err != nil {
		return err // 既存のファイルは上書きしない
	}
	if _, err = f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// restoreAction は、復元で何をするか。書き込んだ値のままの企業だけを戻す（その後に変わった値は触らない）。
// "delete": 行を消す / "update": 書き込む前の値に戻す / "skip": 触らない。
func restoreAction(cur *marketValue, r backupRow) string {
	if cur == nil || *cur != r.After {
		return "skip"
	}
	if r.Before == nil {
		return "delete"
	}
	return "update"
}

func runRestore(db *gorm.DB, path string) {
	b, err := os.ReadFile(path) // #nosec G304 -- 利用者がコマンドラインで指定したパス
	if err != nil {
		log.Fatalf("バックアップを読めません: %v", err)
	}
	var rows []backupRow
	if err := json.Unmarshal(b, &rows); err != nil {
		log.Fatalf("バックアップの形式が不正: %v", err)
	}
	var restored int
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			var info models.CompanyMarketInfo
			var cur *marketValue
			if err := tx.Where("company_id = ?", r.CompanyID).First(&info).Error; err == nil {
				cur = &marketValue{MarketType: info.MarketType, IsListed: info.IsListed, StockCode: info.StockCode}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			switch restoreAction(cur, r) {
			case "delete":
				if err := tx.Unscoped().Where("company_id = ?", r.CompanyID).Delete(&models.CompanyMarketInfo{}).Error; err != nil {
					return err
				}
			case "update":
				if err := upsert(tx, r.CompanyID, *r.Before); err != nil {
					return err
				}
			default:
				continue
			}
			restored++
		}
		return nil
	})
	if err != nil {
		log.Fatalf("復元に失敗（全件ロールバック済み）: %v", err)
	}
	fmt.Printf("元に戻しました: %d 社（バックアップ %d 社のうち、書き込んだ値のままだった企業）\n", restored, len(rows))
}

// ---- ファイル取得 ----

func loadFile(ctx context.Context, path string, fetch func(context.Context) ([]byte, error)) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path) // #nosec G304 -- 利用者がコマンドラインで指定したパス
	}
	return fetch(ctx)
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// download は、User-Agent 付き・サイズ上限つきで取得する。
func download(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxDownloadBytes {
		return nil, errors.New("ファイルが大きすぎます")
	}
	return b, nil
}

var xlsxHrefRe = regexp.MustCompile(`href="([^"]*data_j[^"]*\.xlsx)"`)

// findXLSXLink は、配布ページから data_j.xlsx のリンクを絶対 URL で返す。
func findXLSXLink(pageURL string, html []byte) (string, error) {
	m := xlsxHrefRe.FindSubmatch(html)
	if m == nil {
		return "", errors.New("配布ページに data_j.xlsx のリンクが見つかりません")
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(string(m[1])))
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func fetchJPX(ctx context.Context) ([]byte, error) {
	page, err := download(ctx, jpxPage)
	if err != nil {
		return nil, err
	}
	link, err := findXLSXLink(jpxPage, page)
	if err != nil {
		return nil, err
	}
	return download(ctx, link)
}

// ---- 接続・ガード（fill-name-readings と同じ作り） ----

// writeAllowed は、書き込んでよいか。本番・localhost 以外の DB は、--allow-production が無ければ止める。
func writeAllowed(allowProd bool, host, appEnv string) error {
	if allowProd {
		return nil
	}
	local := host == "localhost"
	if ip := net.ParseIP(host); ip != nil {
		local = ip.IsLoopback()
	}
	if appEnv == "production" || !local {
		return errors.New("本番または localhost 以外の DB への書き込みには --allow-production が必要です（接続先: " + host + "）")
	}
	return nil
}

func dbHost() string {
	if h := os.Getenv("DB_HOST"); h != "" {
		return h
	}
	return "localhost"
}

func loadEnv() {
	if os.Getenv("APP_ENV") == "production" {
		return
	}
	for _, p := range []string{".env", "../.env", "../../.env"} {
		abs, _ := filepath.Abs(p)
		if _, err := os.Stat(abs); err == nil {
			_ = godotenv.Load(abs)
			return
		}
	}
}

func connectDB() (*gorm.DB, error) {
	user, dbname := os.Getenv("DB_USER"), os.Getenv("DB_NAME")
	if user == "" || dbname == "" {
		return nil, errors.New("DB_USER / DB_NAME が設定されていません")
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	cfg := mysqldrv.NewConfig()
	cfg.User, cfg.Passwd = user, os.Getenv("DB_PASSWORD")
	cfg.Net, cfg.Addr, cfg.DBName = "tcp", net.JoinHostPort(dbHost(), port), dbname
	cfg.ParseTime, cfg.Loc = true, time.Local
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	return gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{})
}
