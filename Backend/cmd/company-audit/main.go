// company-audit は既存の companies を sanitizeCompanyInfo 相当の基準で点検し、必要なら直す(SOCAIAGENT-326)。
//
// 既定は読み取り専用(dry-run)。分布と「何が変わるか」を表示するだけで書き込まない。
//
//	go run ./cmd/company-audit                                  # 点検のみ
//	go run ./cmd/company-audit --apply --backup before.json     # 修正を適用(適用前の値を before.json に保存)
//	go run ./cmd/company-audit --restore before.json            # backup から元の値へ戻す
//
// 更新は「読み込み時の値のまま」の行だけに行い、1件でも違えば全件ロールバックする。
// 本番(APP_ENV=production)や localhost 以外のDBへの書き込みは --allow-production を付けた場合だけ許す
// (SSM のポートフォワード経由など、localhost の先が本番でも見分けられない点に注意。接続先を表示する)。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/company"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	apply := flag.Bool("apply", false, "修正をDBへ書き込む(省略時は点検のみ)")
	backup := flag.String("backup", "", "--apply 時に適用前の値を書き出すJSONファイル(必須)")
	restore := flag.String("restore", "", "backup のJSONから元の値へ戻す")
	allowProd := flag.Bool("allow-production", false, "本番(APP_ENV=production)や localhost 以外のDBへの書き込みを許可する")
	flag.Parse()

	if *apply && strings.TrimSpace(*backup) == "" {
		log.Fatal("--apply には --backup <file> が必要です(適用前の値を残さずに書き換えない)")
	}
	if *apply && *restore != "" {
		log.Fatal("--apply と --restore は同時に指定できません")
	}
	loadEnv()
	if (*apply || *restore != "") && !*allowProd {
		if err := checkWriteTarget(os.Getenv("APP_ENV"), dbHost()); err != nil {
			log.Fatal(err)
		}
	}

	db, err := connectDB()
	if err != nil {
		log.Fatalf("DB接続失敗: %v", err)
	}
	fmt.Printf("接続先: %s:%s/%s (APP_ENV=%q)\n", dbHost(), dbPort(), os.Getenv("DB_NAME"), os.Getenv("APP_ENV"))

	if *restore != "" {
		runRestore(db, *restore)
		return
	}

	cs, err := loadAuditTargets(db)
	if err != nil {
		log.Fatalf("読み込み失敗: %v", err)
	}

	rows := make([]company.AuditRow, len(cs))
	industries := make([]string, len(cs))
	workStyles := make([]string, len(cs))
	for i, c := range cs {
		rows[i] = company.AuditRow{ID: c.ID, Name: c.Name, Industry: c.Industry, WorkStyle: c.WorkStyle,
			WebsiteURL: c.WebsiteURL, FoundedYear: c.FoundedYear, EmployeeCount: c.EmployeeCount, EmployeeCountBasis: c.EmployeeCountBasis}
		industries[i], workStyles[i] = c.Industry, c.WorkStyle
	}

	fmt.Printf("対象: %d 社\n\n== industry の分布 ==\n", len(rows))
	for _, d := range company.Distribution(industries) {
		mark := "ゆれ"
		if d.Value == "" || company.IsIndustryOption(d.Value) {
			mark = "正規"
		}
		fmt.Printf("  %-4s %5d  %q\n", mark, d.Count, company.DisplayValue(d.Value))
	}
	fmt.Println("\n== work_style の分布 ==")
	for _, d := range company.Distribution(workStyles) {
		fmt.Printf("  %5d  %q\n", d.Count, company.DisplayValue(d.Value))
	}

	var changes []company.AuditChange
	unmapped := map[string]int{}
	for _, r := range rows {
		plan, ok := company.PlanFix(r)
		if !ok {
			continue
		}
		for col, v := range plan.Unmapped {
			unmapped[col+": "+v]++
		}
		if len(plan.Changes) == 0 {
			continue
		}
		changes = append(changes, company.AuditChange{ID: plan.ID, Name: plan.Name, Before: beforeValues(r, plan.Changes), After: plan.Changes})
	}

	fmt.Printf("\n== 修正対象 %d 社 ==\n", len(changes))
	for _, c := range changes {
		fmt.Printf("  #%d %q: %v → %v\n", c.ID, c.Name, c.Before, c.After)
	}
	fmt.Printf("\n== 選択肢に寄せられず残す値(要判断) ==\n")
	keys := make([]string, 0, len(unmapped))
	for k := range unmapped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %5d  %q\n", unmapped[k], k)
	}

	if !*apply {
		fmt.Println("\n点検のみ(書き込みなし)。適用するには --apply --backup <file>")
		return
	}
	if len(changes) == 0 {
		fmt.Println("\n適用する修正はありません")
		return
	}

	// 書き換える前に、戻すための値をファイルへ残す。書き出せなければ適用しない。
	b, err := json.MarshalIndent(changes, "", "  ")
	if err != nil {
		log.Fatalf("backup 生成失敗: %v", err)
	}
	if err := writeNewFile(*backup, b); err != nil {
		log.Fatalf("backup 書き出し失敗(適用しません): %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return company.ApplyAuditChanges(tx, changes, false)
	}); err != nil {
		log.Fatalf("適用失敗(全件ロールバック済み): %v", err)
	}
	fmt.Printf("\n✓ %d 社へ適用しました。戻す値: %s\n", len(changes), *backup)
}

// loadAuditTargets は点検対象の企業を読む。
//
// 論理削除(is_active=false)された企業は分布・修正計画・--apply のいずれの対象にもしない。
// CompanyRepository.FindAllActive と同じ条件で、wiki の「論理削除された企業は対象外」に合わせる。
func loadAuditTargets(db *gorm.DB) ([]models.Company, error) {
	var cs []models.Company
	err := db.Select("id", "name", "industry", "work_style", "website_url", "founded_year", "employee_count", "employee_count_basis").
		Where("is_active = ?", true).
		Order("id").Find(&cs).Error
	return cs, err
}

// runRestore は backup の After → Before へ戻す。現在値が After のままの行だけ戻す。
func runRestore(db *gorm.DB, path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("backup 読み込み失敗: %v", err)
	}
	changes, err := parseBackup(b)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return company.ApplyAuditChanges(tx, changes, true)
	}); err != nil {
		log.Fatalf("戻し失敗(全件ロールバック済み): %v", err)
	}
	fmt.Printf("✓ %d 社を元の値へ戻しました\n", len(changes))
}

// parseBackup は backup の JSON を復元用の changes へ変換する。
//
// --apply は修正対象が空のとき backup を作らないので、空の backup(null / [])は入力として
// 成立しない。黙って成功扱いにすると「戻した」と誤解する。
func parseBackup(b []byte) ([]company.AuditChange, error) {
	var changes []company.AuditChange
	if err := json.Unmarshal(b, &changes); err != nil {
		return nil, fmt.Errorf("backup の形式が不正です: %w", err)
	}
	if len(changes) == 0 {
		return nil, errors.New("backup に復元対象がありません")
	}
	// JSON の数値は float64 になる。整数列へ渡すので整数へ戻す。
	for _, c := range changes {
		for _, m := range []map[string]any{c.Before, c.After} {
			for k, v := range m {
				if f, ok := v.(float64); ok {
					m[k] = int(f)
				}
			}
		}
	}
	return changes, nil
}

// writeNewFile は新規ファイルとして書く。既存ファイルがあれば失敗する(前回の戻し用データを消さない)。
func writeNewFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// checkWriteTarget は書き込みを許してよい接続先かを返す。
// 本番(APP_ENV が production/prod、大文字小文字は問わない)と、ループバック以外のホストは拒否する。
func checkWriteTarget(appEnv, host string) error {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "production", "prod":
		return errors.New("APP_ENV=production への書き込みは --allow-production が必要です")
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return nil
	}
	return fmt.Errorf("DB_HOST=%s はローカルではありません。書き込むには --allow-production が必要です", host)
}

func dbHost() string {
	if h := os.Getenv("DB_HOST"); h != "" {
		return h
	}
	return "localhost"
}

// dbAddr は接続先。DB_HOST=::1 のとき "::1:3306" を作るとドライバが接続先を取り違えるため、
// IPv6 を角括弧で包む net.JoinHostPort を使う。
func dbAddr() string {
	return net.JoinHostPort(dbHost(), dbPort())
}

func dbPort() string {
	if p := os.Getenv("DB_PORT"); p != "" {
		return p
	}
	return "3306"
}

// beforeValues は changes の各列について、現在値を返す。
func beforeValues(r company.AuditRow, changes map[string]any) map[string]any {
	cur := map[string]any{
		"industry": r.Industry, "work_style": r.WorkStyle, "website_url": r.WebsiteURL,
		"founded_year": r.FoundedYear, "employee_count": r.EmployeeCount, "employee_count_basis": r.EmployeeCountBasis,
	}
	out := make(map[string]any, len(changes))
	for col := range changes {
		out[col] = cur[col]
	}
	return out
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
	cfg := mysqldrv.NewConfig()
	cfg.User, cfg.Passwd = user, os.Getenv("DB_PASSWORD")
	cfg.Net, cfg.Addr, cfg.DBName = "tcp", dbAddr(), dbname
	cfg.ParseTime, cfg.Loc = true, time.Local
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	return gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{})
}
