// profile-distribution は、企業プロファイルの分布と、「ずれる軸」の効果を、集計の数字だけで出す（SOCAIAGENT-670）。
//
// 読み取り専用（セッションを READ ONLY にして接続する）。氏名・メール・企業名・セッション ID は出さない。
// 接続先は DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME（SSM のポートフォワード越しでよい）。
//
//	cd Backend && go run ./cmd/profile-distribution
//
// 出力の数字だけを共有する。パスワードや接続の値は共有しない。
package main

import (
	"errors"
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
	"Backend/internal/repositories"
	"Backend/internal/services/matching"
)

const topN = 10

type scoreRow struct {
	SessionID      string
	WeightCategory string
	Score          int
}

func main() {
	loadEnv()
	db, err := connectDB()
	if err != nil {
		log.Fatalf("接続できません: %v", err)
	}
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	fmt.Printf("接続先: host=%s db=%s（読み取り専用）\n", host, os.Getenv("DB_NAME"))

	var report matching.FitReport
	var jobs jobCoverage
	err = db.Connection(func(tx *gorm.DB) error {
		if err := tx.Exec("SET SESSION TRANSACTION READ ONLY").Error; err != nil {
			return fmt.Errorf("読み取り専用にできません: %w", err)
		}
		var profiles []*models.CompanyWeightProfile
		if err := tx.Raw(`SELECT p.* FROM company_weight_profiles p
			JOIN companies c ON c.id = p.company_id
			WHERE c.data_status = 'published' AND p.job_position_id IS NULL`).Scan(&profiles).Error; err != nil {
			return err
		}
		// 4 軸以上の評価があるセッションだけを、セッション ID を出さずに集める
		var rows []scoreRow
		if err := tx.Raw(`SELECT session_id, weight_category, score FROM user_weight_scores
			WHERE session_id IN (SELECT session_id FROM user_weight_scores WHERE score <> 0
				GROUP BY session_id HAVING COUNT(*) >= ?)`, matching.MinEvaluatedCategoriesForFinal).Scan(&rows).Error; err != nil {
			return err
		}
		bySession := map[string]map[string]float64{}
		for _, r := range rows {
			if bySession[r.SessionID] == nil {
				bySession[r.SessionID] = map[string]float64{}
			}
			bySession[r.SessionID][r.WeightCategory] = float64(r.Score)
		}
		sessions := make([]map[string]float64, 0, len(bySession))
		for _, s := range bySession {
			sessions = append(sessions, s)
		}
		report = matching.BuildFitReport(profiles, sessions, topN)

		// 職種の募集の量（選んだ職種の加点が効く範囲）
		var companyIDs []uint
		for _, p := range profiles {
			companyIDs = append(companyIDs, p.CompanyID)
		}
		var err error
		if jobs, err = loadJobCoverage(tx, companyIDs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Fatalf("集計に失敗しました: %v", err)
	}
	print(report)
	printJobs(jobs)
}

// jobCoverage は、職種別の募集のデータが、公開企業のどれだけをカバーするか。
type jobCoverage struct {
	Companies, WithPositions int
	// 選ばれた職種ごとに、その職種の募集がある公開企業の割合（職種名は出さず、割合だけ）
	ChosenShares []float64
}

func loadJobCoverage(tx *gorm.DB, companyIDs []uint) (jobCoverage, error) {
	c := jobCoverage{Companies: len(companyIDs)}
	if len(companyIDs) == 0 {
		return c, nil
	}
	var n int64
	if err := tx.Raw(`SELECT COUNT(DISTINCT company_id) FROM company_job_positions
		WHERE job_category_id > 0 AND company_id IN ?`, companyIDs).Scan(&n).Error; err != nil {
		return c, err
	}
	c.WithPositions = int(n)

	var chosen []uint
	if err := tx.Raw(`SELECT DISTINCT COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(job_category_ids, '$[0]')) AS UNSIGNED), 0)
		FROM conversation_contexts WHERE job_category_ids NOT IN ('', '[]')`).Scan(&chosen).Error; err != nil {
		return c, err
	}
	repo := repositories.NewJobPreferenceRepository(tx)
	for _, id := range chosen {
		if id == 0 {
			continue
		}
		hiring, err := repo.CompanyIDsHiringCategory(companyIDs, id)
		if err != nil {
			return c, err
		}
		c.ChosenShares = append(c.ChosenShares, float64(len(hiring))/float64(len(companyIDs)))
	}
	return c, nil
}

func printJobs(j jobCoverage) {
	fmt.Printf("\n職種別の募集: 公開企業 %d 社のうち、募集データがある企業 %d 社\n", j.Companies, j.WithPositions)
	if len(j.ChosenShares) == 0 {
		fmt.Println("選ばれた職種ごとの、募集がある企業の割合: 選択データなし")
		return
	}
	sum, max := 0.0, 0.0
	for _, s := range j.ChosenShares {
		sum += s
		if s > max {
			max = s
		}
	}
	fmt.Printf("選ばれた職種（%d 種類）ごとの、募集がある企業の割合: 平均 %.1f%% / 最大 %.1f%%\n", len(j.ChosenShares), sum/float64(len(j.ChosenShares))*100, max*100)
}

func print(r matching.FitReport) {
	fmt.Printf("\n公開企業（職種別でないプロファイル）: %d 社 / 10軸のうち6軸以上が100点の企業: %d 社\n", r.Companies, r.CompaniesGE6At100)
	fmt.Println("軸ごとの企業間の分布（ばらつきのある軸 = 標準偏差15以上）:")
	for _, a := range r.Axes {
		mark := "対象外"
		if a.Informative {
			mark = "対象"
		}
		fmt.Printf("  %-12s 平均%5.1f 中央値%5.1f 標準偏差%5.1f 100点の割合%5.1f%%  %s\n", a.Name, a.Mean, a.Median, a.Std, a.ShareAt100*100, mark)
	}
	fmt.Printf("\n評価済み（4軸以上）のセッション: %d 件\n", r.Sessions)
	if r.Sessions == 0 {
		return
	}
	fmt.Printf("上位%d件: ずれ1軸以上 %.1f%% → %.1f%% / 参考(2軸以上) %.1f%% → %.1f%% / 上位の入れ替わり %.1f%%\n",
		topN, r.TopOff1*100, r.TopOn1*100, r.TopOffRef*100, r.TopOnRef*100, r.TopShift*100)
	fmt.Printf("全企業のうち「参考」になる割合: 平均 %.1f%%（セッション最大 %.0f%%）\n", r.AllRef*100, r.AllRefMax*100)
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
	host, port := os.Getenv("DB_HOST"), os.Getenv("DB_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "3306"
	}
	cfg := mysqldrv.NewConfig()
	cfg.User, cfg.Passwd = user, os.Getenv("DB_PASSWORD")
	cfg.Net, cfg.Addr, cfg.DBName = "tcp", net.JoinHostPort(host, port), dbname
	cfg.ParseTime, cfg.Loc = true, time.Local
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	return gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{})
}
