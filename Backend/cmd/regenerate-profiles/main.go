// regenerate-profiles は、100 に張り付いた旧い企業プロファイルを、今のプロンプト（相対評価）で作り直す（SOCAIAGENT-670）。
//
// 既定は試算だけで、AI を呼ばず、書き込まない。候補と、作り直す前の分布（軸ごとの企業間の幅）を出す。
//
//	cd Backend
//	go run ./cmd/regenerate-profiles                                        # 試算のみ
//	go run ./cmd/regenerate-profiles --apply --limit 10 --backup before.json # 10社だけ作り直す（AI を呼ぶ）
//	go run ./cmd/regenerate-profiles --restore before.json                  # バックアップから元の値へ戻す
//
// 作り直しは AI の費用がかかる（1社1回）。まず少数（--limit 10）で、前後の分布を見てから広げる。
// 本番（APP_ENV=production）や localhost 以外の DB への書き込みは --allow-production を付けた場合だけ許す
// （SSM のポートフォワード越しなど、localhost の先が本番でも見分けられない点に注意。接続先を表示する）。
// 作り直したあとの推薦は、次にその学生が診断した（マッチングを計算した）ときに反映される。
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
	"Backend/internal/repositories"
	"Backend/internal/services/company"
)

var axisNames = [10]string{"技術志向", "チームワーク志向", "リーダーシップ志向", "創造性志向", "安定志向", "成長志向", "ワークライフバランス", "チャレンジ志向", "細部志向", "コミュニケーション力"}

// backupRow は、作り直す前の値（復元に使う）。
type backupRow struct {
	CompanyID uint    `json:"company_id"`
	Axes      [10]int `json:"axes"`
}

func main() {
	apply := flag.Bool("apply", false, "作り直す（AI を呼ぶ）。無ければ試算のみ")
	limit := flag.Int("limit", 10, "作り直す企業数の上限")
	backup := flag.String("backup", "", "作り直す前の値の保存先（--apply で必須。既存のファイルは上書きしない）")
	restore := flag.String("restore", "", "バックアップから元の値へ戻す")
	allowProd := flag.Bool("allow-production", false, "本番・localhost 以外への書き込みを許す")
	flag.Parse()

	loadEnv()
	// 書き込む操作は、接続の前に、接続先が許されるかを確かめる（接続できない先でも、本番への誤爆を先に止める）
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

	before := loadProfiles(db)
	candidates := candidatesOf(before)
	printSpread("作り直す前（公開企業すべて）", company.MeasureProfileSpread(toPtrs(before)))
	n := min(*limit, len(candidates))
	fmt.Printf("\n作り直しの候補（100点の軸が%d以上）: %d 社 / 公開企業 %d 社\n", company.LegacySaturatedAxes, len(candidates), len(before))
	fmt.Printf("今回の対象: %d 社（AI の呼び出し %d 回）\n", n, n)
	if !*apply {
		fmt.Println("\n試算のみです（AI を呼んでいません）。作り直すには --apply --backup <ファイル> を付けてください。")
		return
	}
	if *backup == "" {
		log.Fatal("--apply には --backup が必要です（元の値へ戻せるように）")
	}
	selected := candidates[:n]
	if err := writeBackup(*backup, selected); err != nil {
		log.Fatalf("バックアップに失敗: %v", err)
	}
	fmt.Printf("バックアップ: %s（%d 社）\n", *backup, len(selected))

	aiClient, err := openai.NewFromEnv("")
	if err != nil {
		log.Fatalf("AI クライアント: %v", err)
	}
	svc := company.NewJobFetchService(repositories.NewCompanyRepository(db), aiClient)
	ok, failed := 0, 0
	for _, p := range selected {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, err := svc.FetchAndSavePersona(ctx, p.CompanyID, true)
		cancel()
		if err != nil {
			failed++
			log.Printf("作り直せませんでした（企業 %d）: %v", p.CompanyID, err)
			continue
		}
		ok++
	}
	fmt.Printf("\n作り直し: 成功 %d 社 / 失敗 %d 社\n", ok, failed)
	after := loadProfiles(db)
	printSpread("作り直したあと（公開企業すべて）", company.MeasureProfileSpread(toPtrs(after)))
	fmt.Println("\n元に戻す: go run ./cmd/regenerate-profiles --restore " + *backup)
	fmt.Println("推薦への反映: その学生が次にマッチングを計算したとき（診断・再計算）。")
}

func loadProfiles(db *gorm.DB) []models.CompanyWeightProfile {
	var ps []models.CompanyWeightProfile
	if err := db.Raw(`SELECT p.* FROM company_weight_profiles p
		JOIN companies c ON c.id = p.company_id
		WHERE c.data_status = 'published' AND p.job_position_id IS NULL ORDER BY p.company_id`).Scan(&ps).Error; err != nil {
		log.Fatalf("プロファイルの取得に失敗: %v", err)
	}
	return ps
}

func candidatesOf(ps []models.CompanyWeightProfile) []models.CompanyWeightProfile {
	var out []models.CompanyWeightProfile
	for i := range ps {
		if company.IsLegacySaturated(&ps[i]) {
			out = append(out, ps[i])
		}
	}
	return out
}

func toPtrs(ps []models.CompanyWeightProfile) []*models.CompanyWeightProfile {
	out := make([]*models.CompanyWeightProfile, len(ps))
	for i := range ps {
		out[i] = &ps[i]
	}
	return out
}

func axesOf(p models.CompanyWeightProfile) [10]int {
	return [10]int{p.TechnicalOrientation, p.TeamworkOrientation, p.LeadershipOrientation, p.CreativityOrientation, p.StabilityOrientation,
		p.GrowthOrientation, p.WorkLifeBalance, p.ChallengeSeeking, p.DetailOrientation, p.CommunicationSkill}
}

func printSpread(title string, s company.ProfileSpread) {
	fmt.Printf("\n%s: %d 社 / 100点の軸が%d以上の企業 %d 社 / ばらつきのある軸（標準偏差15以上）%d / 10\n", title, s.Companies, company.LegacySaturatedAxes, s.Saturated, s.Informative)
	for i, name := range axisNames {
		fmt.Printf("  %-12s 標準偏差 %5.1f\n", name, s.AxisStd[i])
	}
}

func writeBackup(path string, ps []models.CompanyWeightProfile) error {
	rows := make([]backupRow, 0, len(ps))
	for _, p := range ps {
		rows = append(rows, backupRow{CompanyID: p.CompanyID, Axes: axesOf(p)})
	}
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

func runRestore(db *gorm.DB, path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("バックアップを読めません: %v", err)
	}
	var rows []backupRow
	if err := json.Unmarshal(b, &rows); err != nil {
		log.Fatalf("バックアップの形式が不正: %v", err)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			res := tx.Model(&models.CompanyWeightProfile{}).Where("company_id = ? AND job_position_id IS NULL", r.CompanyID).Updates(map[string]any{
				"technical_orientation": r.Axes[0], "teamwork_orientation": r.Axes[1], "leadership_orientation": r.Axes[2],
				"creativity_orientation": r.Axes[3], "stability_orientation": r.Axes[4], "growth_orientation": r.Axes[5],
				"work_life_balance": r.Axes[6], "challenge_seeking": r.Axes[7], "detail_orientation": r.Axes[8], "communication_skill": r.Axes[9],
			})
			if res.Error != nil {
				return res.Error
			}
		}
		return nil // 1件でも失敗したら全件ロールバック
	})
	if err != nil {
		log.Fatalf("復元に失敗（全件ロールバック済み）: %v", err)
	}
	fmt.Printf("復元しました: %d 社\n", len(rows))
}

func dbHost() string {
	if h := os.Getenv("DB_HOST"); h != "" {
		return h
	}
	return "localhost"
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
