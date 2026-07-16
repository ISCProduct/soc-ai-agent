// Package migrations はバージョン管理型DBマイグレーションを提供する (#614)
//
// マイグレーションファイル（*.up.sql / *.down.sql）はこのディレクトリに配置し、
// バイナリに埋め込まれる。スキーマ変更は必ずマイグレーションファイルで行うこと
// （GORM AutoMigrate は使用禁止）。
package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/go-sql-driver/mysql"
)

//go:embed *.sql
var migrationFiles embed.FS

// baselineVersion は既存DB（AutoMigrate運用時代のスキーマ）を適用済みとみなす初期バージョン
const baselineVersion = 1

// Open は DSN からマイグレーション用の DB 接続を開く。
// マイグレーションファイルは複数ステートメントを含むため multiStatements=true を強制する。
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn+"&multiStatements=true")
	if err != nil {
		return nil, fmt.Errorf("マイグレーション用DB接続に失敗: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("マイグレーション用DB疎通に失敗: %w", err)
	}
	return db, nil
}

// New は埋め込みマイグレーションファイルからマイグレータを生成する
func New(db *sql.DB) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationFiles, ".")
	if err != nil {
		return nil, fmt.Errorf("マイグレーションソースの読み込みに失敗: %w", err)
	}
	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{})
	if err != nil {
		return nil, fmt.Errorf("マイグレーションドライバの初期化に失敗: %w", err)
	}
	return migrate.NewWithInstance("iofs", src, "mysql", driver)
}

// Up は未適用のマイグレーションをすべて適用する。
// AutoMigrate 時代に構築された既存DBは、初期スナップショット(version 1)を
// 適用済みとして記録（ベースライン）してから差分のみ適用する。
func Up(dsn string) error {
	db, err := Open(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureBaseline(db); err != nil {
		return err
	}

	m, err := New(db)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("マイグレーション適用に失敗: %w", err)
	}
	return nil
}

// Down は直近のマイグレーションを1つロールバックする
func Down(dsn string) error {
	db, err := Open(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	m, err := New(db)
	if err != nil {
		return err
	}
	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("ロールバックに失敗: %w", err)
	}
	return nil
}

// Version は現在のマイグレーションバージョンと dirty フラグを返す
func Version(dsn string) (version uint, dirty bool, err error) {
	db, err := Open(dsn)
	if err != nil {
		return 0, false, err
	}
	defer db.Close()

	m, err := New(db)
	if err != nil {
		return 0, false, err
	}
	version, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return version, dirty, err
}

// Force はバージョンを強制的に設定する（dirty 状態からの復旧用）
func Force(dsn string, version int) error {
	db, err := Open(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	m, err := New(db)
	if err != nil {
		return err
	}
	return m.Force(version)
}

// ensureBaseline は schema_migrations が存在せず、かつ既存テーブルがあるDBを
// version 1（初期スナップショット）適用済みとして記録する。
// 新規DB（テーブルなし）の場合は何もしない（通常の Up で全適用される）。
func ensureBaseline(db *sql.DB) error {
	hasVersionTable, err := tableExists(db, "schema_migrations")
	if err != nil {
		return err
	}
	if hasVersionTable {
		return nil
	}

	hasExistingSchema, err := tableExists(db, "users")
	if err != nil {
		return err
	}
	if !hasExistingSchema {
		return nil
	}

	m, err := New(db)
	if err != nil {
		return err
	}
	if err := m.Force(baselineVersion); err != nil {
		return fmt.Errorf("既存DBのベースライン記録に失敗: %w", err)
	}
	return nil
}

// tableExists は現在のデータベースにテーブルが存在するか確認する
func tableExists(db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
		name,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("テーブル存在確認に失敗 (%s): %w", name, err)
	}
	return count > 0, nil
}
