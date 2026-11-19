// training-export は軽量採点モデルの学習データを書き出す（#2168。設計は #1834 の 2・4）。
//
//	TRAINING_EXPORT_HMAC_KEY=<32バイト以上の秘密鍵> TRAINING_EXPORT_S3_URI=s3://bucket/prefix go run ./cmd/training-export
//
// 学習に使ってよい学生（同意済み・未退会）の完了した面接を、マスキングして S3 の非公開のプレフィックスに書き、
// 監査ログ（training_scorer_export_runs）に結果を残す。鍵か出力先が無ければ、DB にもつながずに終わる。
//
// 管理者の API ではなく CLI にしているのは、API にすると、誰が実行でき、結果をどこまで返すか
// （権限と公開範囲）の設計が別に要るため。書き出したデータはアプリのほかの経路から読めないようにする方針
// （#2168）なので、実行もアプリの外（実行ロールを絞ったタスク）に置く。
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"Backend/internal/repositories"
	"Backend/internal/services/training/scorerexport"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	loadEnv()
	res, err := export()
	if err != nil {
		log.Fatalf("書き出しに失敗しました: %v", err)
	}
	fmt.Printf("書き出しました: 学生 %d 人・面接 %d 件（書き出さなかった面接 %d 件）→ %s\n",
		res.Students, res.Sessions, res.SkippedSessions, res.Destination)
}

// export は Ctrl+C・SIGTERM で取り消せる ctx で書き出しを1回行う。取り消して失敗した回も、監査ログには「失敗」が残る。
func export() (scorerexport.Result, error) {
	// コンテナのタスクの停止・タイムアウトでは SIGTERM が送られる。SIGTERM でも取り消して、
	// 監査ログに「失敗」を残してから終わる（#2325 のレビュー）。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return scorerexport.RunFromEnv(ctx, os.Getenv, connectStore, newS3Writer)
}

func connectStore() (scorerexport.Store, error) {
	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	return repositories.NewTrainingScorerExportRepository(db), nil
}

func newS3Writer(ctx context.Context, bucket string) (scorerexport.ObjectWriter, error) {
	return scorerexport.NewS3Writer(ctx, bucket)
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
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		return nil, errors.New("DB_USER が設定されていません")
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		return nil, errors.New("DB_NAME が設定されていません")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, os.Getenv("DB_PASSWORD"), host, port, dbname)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}
