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
	"log"

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
// dirty かつ当該バージョンのスキーマが揃っている場合は自動修復して再試行する
// （Docker Compose 起動時の途中失敗からの復旧用）。
func Up(dsn string) error {
	return up(dsn, true)
}

func up(dsn string, allowDirtyRepair bool) error {
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
		var dirtyErr migrate.ErrDirty
		if allowDirtyRepair && errors.As(err, &dirtyErr) {
			repaired, rerr := repairDirtyIfSafe(db, dirtyErr.Version)
			if rerr != nil {
				return fmt.Errorf("マイグレーション dirty 修復に失敗 (version %d): %w", dirtyErr.Version, rerr)
			}
			if repaired {
				log.Printf("[migrations] dirty version %d をスキーマ確認のうえ修復し、再適用します", dirtyErr.Version)
				return up(dsn, false)
			}
		}
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

// repairDirtyIfSafe は dirty バージョンのスキーマが揃っているときだけ Force で dirty を解除する。
// 途中失敗（テーブル欠落）の場合は修復せず false を返す。
func repairDirtyIfSafe(db *sql.DB, version int) (bool, error) {
	ok, err := versionLooksApplied(db, version)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	m, err := New(db)
	if err != nil {
		return false, err
	}
	if err := m.Force(version); err != nil {
		return false, err
	}
	return true, nil
}

// versionLooksApplied は各マイグレーション完了の目印となるオブジェクトの有無を確認する。
// 部分適用を完了扱いしないよう、version 4 は関連テーブル／列まで確認する。
func versionLooksApplied(db *sql.DB, version int) (bool, error) {
	switch version {
	case 1:
		return tableExists(db, "users")
	case 2:
		return tableExists(db, "user_refresh_tokens")
	case 3:
		return tableExists(db, "withdrawn_users")
	case 4:
		checks := []struct {
			table  string
			column string
		}{
			{"organizations", ""},
			{"organization_memberships", ""},
			{"users", "organization_id"},
			{"chat_messages", "organization_id"},
			{"user_weight_scores", "organization_id"},
			{"interview_sessions", "organization_id"},
			{"interview_videos", "organization_id"},
			{"resume_documents", "organization_id"},
		}
		for _, c := range checks {
			ok, err := tableExists(db, c.table)
			if err != nil || !ok {
				return ok, err
			}
			if c.column != "" {
				ok, err = columnExists(db, c.table, c.column)
				if err != nil || !ok {
					return ok, err
				}
			}
		}
		return true, nil
	case 5:
		// FK 制約は information_schema で確認（テーブル/列は version 4 で保証済み）
		return foreignKeyExists(db, "chat_messages", "fk_chat_messages_organization")
	default:
		// 未知バージョンは自動修復しない
		return false, nil
	}
}

// foreignKeyExists は指定名の外部キーが存在するか確認する。
func foreignKeyExists(db *sql.DB, table, constraint string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.table_constraints
		 WHERE table_schema = DATABASE() AND table_name = ? AND constraint_name = ? AND constraint_type = 'FOREIGN KEY'`,
		table, constraint,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("FK存在確認に失敗 (%s.%s): %w", table, constraint, err)
	}
	return count > 0, nil
}

// columnExists は現在のデータベースに列が存在するか確認する
func columnExists(db *sql.DB, table, column string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		table, column,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("列存在確認に失敗 (%s.%s): %w", table, column, err)
	}
	return count > 0, nil
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
