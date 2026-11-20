// fill-corporate-numbers は、法人番号（companies.corporate_number）が空の企業の法人番号を、
// 国税庁の法人番号システム Web-API から補う。
//
// 既定は確認だけで、DB には書き込まない（国税庁への問い合わせは行う。読み取りのみ・無料）。
// 特定できた企業と、採用しなかった企業（理由つき）を表示する。
//
//		cd Backend
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/fill-corporate-numbers                              # 確認のみ（公開済みだけ）
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/fill-corporate-numbers --include-drafts --limit 50  # 公開前・仮登録も含め、50社まで
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/fill-corporate-numbers --apply --backup before.json # 空の法人番号だけを埋める
//		go run ./cmd/fill-corporate-numbers --restore before.json                                 # 埋めた法人番号を空に戻す
//
//	  - 対象は、有効（is_active）・ゲスト投稿でない・法人番号が空の企業。既定は公開済み（published）だけ。
//	    --include-drafts で公開前（draft）・仮登録（is_provisional）も含める。既に法人番号がある企業は変えない。
//	  - 特定は houjinbangou.Client.ResolveCorporateNumber（商号と所在地で、一意に決まったときだけ採用。閉鎖法人は採用しない）。
//	    この関数は「採用しない」理由を返さないので、該当なし・複数・閉鎖・所在地の矛盾は「特定できない」とまとめる。
//	  - 国税庁への問い合わせ間隔は、クライアントのレート制限（1回/秒）に従う。
//	  - 書き込むのは corporate_number だけ。
//	  - 本番（APP_ENV=production）や localhost 以外の DB への書き込みは --allow-production が必要（接続より先に確認する）。
//	  - アプリケーションID（HOUJIN_BANGOU_APP_ID）は、ログにも出さない。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"Backend/internal/models"
	"Backend/internal/services/houjinbangou"
)

const (
	reasonNotResolved = "特定できない（該当なし・複数・閉鎖・所在地が矛盾のいずれか）"
	reasonClosed      = "閉鎖された法人"
	reasonBadNumber   = "返された法人番号が13桁の数字ではない"

	attribution = "国税庁法人番号公表サイト（国税庁）"
)

var thirteenDigits = regexp.MustCompile(`^[0-9]{13}$`)

// finder は ResolveCorporateNumber だけを使う（houjinbangou.Client が満たす。テストでは偽物に差し替える）。
type finder interface {
	ResolveCorporateNumber(ctx context.Context, name, locationHint string) (*houjinbangou.Corporation, error)
}

// backupRow は、書き込んだ企業（復元に使う）。書き込む前は空だった企業だけが対象。
type backupRow struct {
	CompanyID       uint   `json:"company_id"`
	CorporateNumber string `json:"corporate_number"`
}

type unresolved struct {
	ID     uint
	Name   string
	Reason string
}

type result struct {
	Found  []backupRow
	Names  map[uint]string // 表示用
	Misses []unresolved
	Failed int
}

func main() {
	apply := flag.Bool("apply", false, "DB に書き込む。無ければ確認のみ")
	backup := flag.String("backup", "", "書き込む前の値の保存先（--apply で必須。既存のファイルは上書きしない）")
	restore := flag.String("restore", "", "バックアップの企業の法人番号を、空に戻す")
	includeDrafts := flag.Bool("include-drafts", false, "公開前（draft）・仮登録（is_provisional）の企業も対象にする")
	limit := flag.Int("limit", 0, "対象の件数の上限（0 は無制限）")
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
	client := houjinbangou.NewClientFromEnv()
	if *restore == "" && !client.Enabled() {
		log.Fatal("HOUJIN_BANGOU_APP_ID が未設定です（国税庁の法人番号システム Web-API のアプリケーションID）")
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

	var targets []models.Company
	if err := targetQuery(db, *includeDrafts, *limit).Find(&targets).Error; err != nil {
		log.Fatalf("対象の取得に失敗: %v", err)
	}
	scope := "公開済み"
	if *includeDrafts {
		scope = "公開前・仮登録を含む"
	}
	fmt.Printf("対象（%s・法人番号が空）: %d 社\n\n", scope, len(targets))

	res := resolveAll(context.Background(), client, targets)
	printReport(res, len(targets))

	if !*apply {
		fmt.Println("\n確認のみです（DB には書き込んでいません）。書き込むには --apply --backup <ファイル> を付けてください。")
		return
	}
	if len(res.Found) == 0 {
		fmt.Println("\n書き込む法人番号がありません。")
		return
	}
	if err := writeBackup(*backup, res.Found); err != nil {
		log.Fatalf("バックアップに失敗: %v", err)
	}
	n, err := applyNumbers(db, res.Found)
	if err != nil {
		log.Fatalf("書き込みに失敗（全件ロールバック済み）: %v", err)
	}
	fmt.Printf("\n書き込みました: %d 社（バックアップ: %s）\n元に戻す: go run ./cmd/fill-corporate-numbers --restore %s\n", n, *backup, *backup)
}

// targetQuery は対象の企業を選ぶ。既定は公開済みだけ。includeDrafts なら data_status は問わない。
func targetQuery(db *gorm.DB, includeDrafts bool, limit int) *gorm.DB {
	q := db.Model(&models.Company{}).
		Where("is_active = ? AND is_guest_entry = ? AND (corporate_number IS NULL OR corporate_number = '')", true, false)
	if !includeDrafts {
		q = q.Where("data_status = ?", "published")
	}
	q = q.Order("id")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return q
}

// resolveAll は、各企業の法人番号を特定する。採用の判断は ResolveCorporateNumber に任せ、独自の判定は足さない。
func resolveAll(ctx context.Context, f finder, targets []models.Company) result {
	res := result{Names: map[uint]string{}}
	for i := range targets {
		c := &targets[i]
		corp, err := f.ResolveCorporateNumber(ctx, c.Name, c.Location)
		switch {
		case err != nil:
			res.Failed++
			log.Printf("取得に失敗（企業 %d）: %v", c.ID, err)
		case corp == nil:
			res.Misses = append(res.Misses, unresolved{c.ID, c.Name, reasonNotResolved})
		case corp.IsClosed():
			res.Misses = append(res.Misses, unresolved{c.ID, c.Name, reasonClosed})
		case !thirteenDigits.MatchString(strings.TrimSpace(corp.CorporateNumber)):
			res.Misses = append(res.Misses, unresolved{c.ID, c.Name, reasonBadNumber})
		default:
			num := strings.TrimSpace(corp.CorporateNumber)
			res.Found = append(res.Found, backupRow{CompanyID: c.ID, CorporateNumber: num})
			res.Names[c.ID] = c.Name
		}
	}
	return res
}

func printReport(res result, total int) {
	for _, r := range res.Found {
		fmt.Printf("  %-5d %s → %s\n", r.CompanyID, res.Names[r.CompanyID], r.CorporateNumber)
	}
	byReason := map[string]int{}
	for _, m := range res.Misses {
		byReason[m.Reason]++
	}
	fmt.Printf("\n結果: 特定 %d 社 / 採用しない %d 社 / 通信などの失敗 %d 社（対象 %d 社）\n", len(res.Found), len(res.Misses), res.Failed, total)
	for reason, n := range byReason {
		fmt.Printf("  採用しない（%s）: %d 社\n", reason, n)
	}
	if len(res.Misses) > 0 {
		fmt.Println("\n採用しなかった企業（人が確認して、必要なら手で入れてください）:")
		for _, m := range res.Misses {
			fmt.Printf("  %-5d %s — %s\n", m.ID, m.Name, m.Reason)
		}
	}
	fmt.Printf("\n【出典の表示】国税庁のデータを使うため、「%s」の出典表示が必要です。利用規約の確認は人が行ってください。\n", attribution)
}

// applyNumbers は、法人番号が空の企業だけを埋める（既に値がある企業は変えない）。corporate_number 以外は触らない。全件を1トランザクションで行う。
func applyNumbers(db *gorm.DB, rows []backupRow) (int64, error) {
	var total int64
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			res := fillOne(tx, r)
			if res.Error != nil {
				return res.Error
			}
			total += res.RowsAffected
		}
		return nil
	})
	return total, err
}

// fillOne は1社の書き込み。法人番号が空のときだけ、corporate_number だけを更新する。
func fillOne(tx *gorm.DB, r backupRow) *gorm.DB {
	return tx.Model(&models.Company{}).
		Where("id = ? AND (corporate_number IS NULL OR corporate_number = '')", r.CompanyID).
		UpdateColumn("corporate_number", r.CorporateNumber)
}

// restoreOne は1社の復元。書き込んだ値のままのときだけ空に戻す。
func restoreOne(tx *gorm.DB, r backupRow) *gorm.DB {
	return tx.Model(&models.Company{}).Where("id = ? AND corporate_number = ?", r.CompanyID, r.CorporateNumber).
		UpdateColumn("corporate_number", "")
}

func writeBackup(path string, rows []backupRow) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // 保存先は運用者が指定するCLI引数
	if err != nil {
		return err // 既存のファイルは上書きしない
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func readBackup(path string) ([]backupRow, error) {
	b, err := os.ReadFile(path) //nolint:gosec // 読み込み元は運用者が指定するCLI引数
	if err != nil {
		return nil, err
	}
	var rows []backupRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("バックアップの形式が不正: %w", err)
	}
	return rows, nil
}

// runRestore は、バックアップの企業の法人番号を空に戻す。ただし、書き込んだ値のままの企業だけ（その後に人が直した値は消さない）。
func runRestore(db *gorm.DB, path string) {
	rows, err := readBackup(path)
	if err != nil {
		log.Fatalf("バックアップを読めません: %v", err)
	}
	var restored int64
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			res := restoreOne(tx, r)
			if res.Error != nil {
				return res.Error
			}
			restored += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		log.Fatalf("復元に失敗（全件ロールバック済み）: %v", err)
	}
	fmt.Printf("空に戻しました: %d 社（バックアップ %d 社のうち、書き込んだ値のままだった企業）\n", restored, len(rows))
}

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

// connectDB は DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME だけで接続する（認証用の秘密値は要らない）。
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
