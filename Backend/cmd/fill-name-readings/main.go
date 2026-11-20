// fill-name-readings は、公開済みの企業の読み仮名（companies.name_reading）を、法人番号公表サイトから埋める。
//
// 既定は確認だけで、DB には書き込まない（法人番号公表サイトへの問い合わせは行う。読み取りのみ・無料）。
// 取得結果と、採用できなかった企業（理由つき）を表示する。
//
//		cd Backend
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/fill-name-readings                              # 確認のみ
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/fill-name-readings --apply --backup before.json  # 空の読み仮名だけを埋める
//		go run ./cmd/fill-name-readings --restore before.json                                  # 埋めた読み仮名を空に戻す
//
//	  - 対象は、公開済み（published）・有効・読み仮名が空の企業だけ。既に値がある企業は変えない。
//	  - まず法人番号公表サイトのフリガナ（公的データ）を使う。取れなかった企業だけ、公式サイトの会社概要から読みを抜き出す
//	    （--no-website で止められる。OPENAI_API_KEY が必要）。AI には推測させず、サイトに書かれた記載の抜き出しだけをさせ、
//	    引用文がページに実在すること・読みが引用文に書かれていることをコードで検証する。満たさなければ採用しない。
//	  - 読みの出どころ（corporate_registry / website）と、出典の URL を DB に残す。
//	  - 本番（APP_ENV=production）や localhost 以外の DB への書き込みは --allow-production が必要（接続より先に確認する）。
//	  - 法人番号公表サイトのアプリケーションID（HOUJIN_BANGOU_APP_ID）は、ログにも出さない。
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
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/company"
	"Backend/internal/services/houjinbangou"
)

// 法人番号公表サイトへの問い合わせの間隔（相手のサーバーに負荷をかけない）。
const requestInterval = 300 * time.Millisecond

// backupRow は、書き込んだ企業（復元に使う）。書き込む前は空だった企業だけが対象。
type backupRow struct {
	CompanyID uint   `json:"company_id"`
	Reading   string `json:"reading"`
	Method    string `json:"method"`
	SourceURL string `json:"source_url,omitempty"`
	Quote     string `json:"quote,omitempty"` // 公式サイトから読み取った場合の、根拠の引用（確認用）
}

type unresolved struct {
	ID     uint
	Name   string
	Reason string
}

func main() {
	apply := flag.Bool("apply", false, "DB に書き込む。無ければ確認のみ")
	backup := flag.String("backup", "", "書き込んだ企業の保存先（--apply で必須。既存のファイルは上書きしない）")
	restore := flag.String("restore", "", "バックアップの企業の読み仮名を、空に戻す")
	noWebsite := flag.Bool("no-website", false, "公式サイトからの読み取り（AI を使う）をしない")
	allowProd := flag.Bool("allow-production", false, "本番・localhost 以外への書き込みを許す")
	flag.Parse()

	loadEnv()
	if *apply || *restore != "" {
		mustAllowWrite(*allowProd)
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
	if *apply && *backup == "" {
		log.Fatal("--apply には --backup が必要です（元に戻せるように）")
	}

	finder := houjinbangou.NewClientFromEnv()
	if !finder.Enabled() {
		log.Fatal("HOUJIN_BANGOU_APP_ID が未設定です（国税庁の法人番号システム Web-API のアプリケーションID）")
	}

	var siteExtractor *company.SiteReadingExtractor
	if !*noWebsite {
		ai, err := openai.NewFromEnv("")
		if err != nil {
			log.Fatalf("公式サイトからの読み取りには AI の設定が必要です（--no-website で止められます）: %v", err)
		}
		siteExtractor = company.NewSiteReadingExtractor(ai)
	}

	var targets []models.Company
	if err := db.Where("data_status = ? AND is_active = ? AND (name_reading IS NULL OR name_reading = '')", "published", true).
		Order("id").Find(&targets).Error; err != nil {
		log.Fatalf("対象の取得に失敗: %v", err)
	}
	fmt.Printf("対象（公開済み・読み仮名が空）: %d 社\n\n", len(targets))

	var found []backupRow
	var misses []unresolved
	failed := 0
	for i := range targets {
		c := &targets[i]
		res, err := company.ResolveNameReading(context.Background(), finder, c)
		time.Sleep(requestInterval)
		if err != nil {
			failed++
			log.Printf("取得に失敗（企業 %d）: %v", c.ID, err)
			continue
		}
		if res.Reading != "" {
			found = append(found, backupRow{CompanyID: c.ID, Reading: res.Reading, Method: res.Method})
			fmt.Printf("  %-5d %s → %s（%s）\n", c.ID, c.Name, res.Reading, res.Method)
			continue
		}
		// 法人番号公表サイトで取れなかった企業だけ、公式サイトの会社概要から読み取る
		if siteExtractor == nil {
			misses = append(misses, unresolved{c.ID, c.Name, res.Reason})
			continue
		}
		site, err := siteExtractor.Extract(context.Background(), c)
		if err != nil {
			failed++
			log.Printf("公式サイトからの読み取りに失敗（企業 %d）: %v", c.ID, err)
			continue
		}
		if site.Reading == "" {
			misses = append(misses, unresolved{c.ID, c.Name, res.Reason + " / " + site.Reason})
			continue
		}
		found = append(found, backupRow{CompanyID: c.ID, Reading: site.Reading, Method: company.MethodWebsite, SourceURL: site.SourceURL, Quote: site.Quote})
		fmt.Printf("  %-5d %s → %s（%s）\n        根拠: 「%s」 %s\n", c.ID, c.Name, site.Reading, company.MethodWebsite, site.Quote, site.SourceURL)
	}

	fmt.Printf("\n結果: 取得 %d 社 / 採用しない %d 社 / 通信などの失敗 %d 社（対象 %d 社）\n", len(found), len(misses), failed, len(targets))
	if len(misses) > 0 {
		fmt.Println("\n採用しなかった企業（人が確認して、必要なら手で入れてください）:")
		for _, m := range misses {
			fmt.Printf("  %-5d %s — %s\n", m.ID, m.Name, m.Reason)
		}
	}
	if !*apply {
		fmt.Println("\n確認のみです（DB には書き込んでいません）。書き込むには --apply --backup <ファイル> を付けてください。")
		return
	}
	if len(found) == 0 {
		fmt.Println("\n書き込む読み仮名がありません。")
		return
	}
	if err := writeBackup(*backup, found); err != nil {
		log.Fatalf("バックアップに失敗: %v", err)
	}
	n, err := applyReadings(db, found)
	if err != nil {
		log.Fatalf("書き込みに失敗（全件ロールバック済み）: %v", err)
	}
	fmt.Printf("\n書き込みました: %d 社（バックアップ: %s）\n元に戻す: go run ./cmd/fill-name-readings --restore %s\n", n, *backup, *backup)
}

// applyReadings は、読み仮名が空の企業だけを埋める（既に値がある企業は変えない）。全件を1トランザクションで行う。
func applyReadings(db *gorm.DB, rows []backupRow) (int64, error) {
	var total int64
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			res := tx.Model(&models.Company{}).
				Where("id = ? AND (name_reading IS NULL OR name_reading = '')", r.CompanyID).
				Updates(map[string]any{
					"name_reading":            r.Reading,
					"name_reading_source":     sourceOf(r.Method),
					"name_reading_source_url": r.SourceURL,
				})
			if res.Error != nil {
				return res.Error
			}
			total += res.RowsAffected
		}
		return nil
	})
	return total, err
}

// sourceOf は、取得方法を、DB に残す出どころの値にする。
func sourceOf(method string) string {
	if method == company.MethodWebsite {
		return "website"
	}
	return "corporate_registry" // 法人番号公表サイト（登録の法人番号、または商号と所在地での特定）
}

func writeBackup(path string, rows []backupRow) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err // 既存のファイルは上書きしない
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

// runRestore は、バックアップの企業の読み仮名を空に戻す。ただし、書き込んだ値のままの企業だけ（その後に人が直した値は消さない）。
func runRestore(db *gorm.DB, path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("バックアップを読めません: %v", err)
	}
	var rows []backupRow
	if err := json.Unmarshal(b, &rows); err != nil {
		log.Fatalf("バックアップの形式が不正: %v", err)
	}
	var restored int64
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			res := tx.Model(&models.Company{}).Where("id = ? AND name_reading = ?", r.CompanyID, r.Reading).
				Updates(map[string]any{"name_reading": "", "name_reading_source": "", "name_reading_source_url": ""})
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

func mustAllowWrite(allowProd bool) {
	if err := writeAllowed(allowProd, dbHost(), os.Getenv("APP_ENV")); err != nil {
		log.Fatal(err)
	}
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
