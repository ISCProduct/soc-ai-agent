package repositories

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/shared"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const sessionOwnershipRaceIterations = 20

func TestClaimSessionOwnershipMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN が未設定のため、MySQL回帰テストをスキップします")
	}

	db, repo := newSessionOwnershipMySQLTestRepository(t, dsn)
	var isolation string
	if err := db.Raw("SELECT @@transaction_isolation").Scan(&isolation).Error; err != nil {
		t.Fatalf("トランザクション分離レベルの取得に失敗: %v", err)
	}
	if isolation != "REPEATABLE-READ" {
		t.Fatalf("トランザクション分離レベル = %q, want REPEATABLE-READ", isolation)
	}

	t.Run("ConcurrentDifferentUsers", func(t *testing.T) {
		for iteration := range sessionOwnershipRaceIterations {
			resetSessionOwnershipRows(t, db)
			sessionID := fmt.Sprintf("same-session-%02d", iteration)
			results := claimSessionOwnershipConcurrently(repo, []sessionOwnershipClaim{
				{sessionID: sessionID, userID: 101},
				{sessionID: sessionID, userID: 202},
			})

			successes, forbidden := 0, 0
			for _, result := range results {
				switch {
				case result.err == nil:
					successes++
				case errors.Is(result.err, shared.ErrForbidden):
					forbidden++
				default:
					t.Fatalf("iteration %d: ClaimSessionOwnership(%q, %d) error = %v, want nil or ErrForbidden",
						iteration, result.claim.sessionID, result.claim.userID, result.err)
				}
			}
			if successes != 1 || forbidden != 1 {
				t.Fatalf("iteration %d: success count = %d, forbidden count = %d, want 1 each",
					iteration, successes, forbidden)
			}

			var owners []models.SessionValidation
			if err := db.Where("session_id = ? AND user_id IS NOT NULL", sessionID).Find(&owners).Error; err != nil {
				t.Fatalf("iteration %d: owner lookup failed: %v", iteration, err)
			}
			if len(owners) != 1 || owners[0].UserID == nil {
				t.Fatalf("iteration %d: persisted owners = %#v, want exactly one", iteration, owners)
			}
			if ownerID := *owners[0].UserID; ownerID != 101 && ownerID != 202 {
				t.Fatalf("iteration %d: persisted owner = %d, want 101 or 202", iteration, ownerID)
			}
		}
	})

	t.Run("ConcurrentDifferentSessions", func(t *testing.T) {
		for iteration := range sessionOwnershipRaceIterations {
			resetSessionOwnershipRows(t, db)
			claims := make([]sessionOwnershipClaim, 8)
			for index := range claims {
				claims[index] = sessionOwnershipClaim{
					sessionID: fmt.Sprintf("different-session-%02d-%02d", iteration, index),
					userID:    uint(300 + index),
				}
			}

			results := claimSessionOwnershipConcurrently(repo, claims)
			for _, result := range results {
				if result.err != nil {
					var mysqlErr *mysqlDriver.MySQLError
					if errors.As(result.err, &mysqlErr) && mysqlErr.Number == 1213 {
						t.Fatalf("iteration %d: ClaimSessionOwnership(%q, %d) returned MySQL deadlock 1213",
							iteration, result.claim.sessionID, result.claim.userID)
					}
					t.Fatalf("iteration %d: ClaimSessionOwnership(%q, %d) error = %v, want nil",
						iteration, result.claim.sessionID, result.claim.userID, result.err)
				}
			}

			var ownerCount int64
			if err := db.Model(&models.SessionValidation{}).
				Where("user_id IS NOT NULL").Count(&ownerCount).Error; err != nil {
				t.Fatalf("iteration %d: owner count failed: %v", iteration, err)
			}
			if ownerCount != int64(len(claims)) {
				t.Fatalf("iteration %d: persisted owner count = %d, want %d",
					iteration, ownerCount, len(claims))
			}
		}
	})

	t.Run("ConcurrentSameUser", func(t *testing.T) {
		for iteration := range sessionOwnershipRaceIterations {
			resetSessionOwnershipRows(t, db)
			sessionID := fmt.Sprintf("same-owner-session-%02d", iteration)
			results := claimSessionOwnershipConcurrently(repo, []sessionOwnershipClaim{
				{sessionID: sessionID, userID: 404},
				{sessionID: sessionID, userID: 404},
			})

			for _, result := range results {
				if result.err != nil {
					t.Fatalf("iteration %d: ClaimSessionOwnership(%q, %d) error = %v, want nil",
						iteration, result.claim.sessionID, result.claim.userID, result.err)
				}
			}

			var owners []models.SessionValidation
			if err := db.Where("session_id = ? AND user_id IS NOT NULL", sessionID).Find(&owners).Error; err != nil {
				t.Fatalf("iteration %d: owner lookup failed: %v", iteration, err)
			}
			if len(owners) != 1 || owners[0].UserID == nil || *owners[0].UserID != 404 {
				t.Fatalf("iteration %d: persisted owner = %#v, want one owner 404", iteration, owners)
			}
		}
	})
}

type sessionOwnershipClaim struct {
	sessionID string
	userID    uint
}

type sessionOwnershipClaimResult struct {
	claim sessionOwnershipClaim
	err   error
}

func claimSessionOwnershipConcurrently(
	repo *SessionValidationRepository,
	claims []sessionOwnershipClaim,
) []sessionOwnershipClaimResult {
	start := make(chan struct{})
	results := make(chan sessionOwnershipClaimResult, len(claims))
	var ready sync.WaitGroup
	ready.Add(len(claims))

	for _, claim := range claims {
		claim := claim
		go func() {
			ready.Done()
			<-start
			results <- sessionOwnershipClaimResult{
				claim: claim,
				err:   repo.ClaimSessionOwnership(claim.sessionID, claim.userID),
			}
		}()
	}

	ready.Wait()
	close(start)

	collected := make([]sessionOwnershipClaimResult, 0, len(claims))
	for range claims {
		collected = append(collected, <-results)
	}
	return collected
}

func resetSessionOwnershipRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("TRUNCATE TABLE session_validations").Error; err != nil {
		t.Fatalf("session_validations の初期化に失敗: %v", err)
	}
}

func newSessionOwnershipMySQLTestRepository(
	t *testing.T,
	dsn string,
) (*gorm.DB, *SessionValidationRepository) {
	t.Helper()

	config, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("TEST_MYSQL_DSN の解析に失敗: %v", err)
	}
	config.DBName = ""

	adminDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("MySQL管理接続の作成に失敗: %v", err)
	}
	adminDB.SetConnMaxLifetime(time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		_ = adminDB.Close()
		t.Fatalf("MySQLへの接続に失敗: %v", err)
	}

	randomSuffix := make([]byte, 8)
	if _, err := rand.Read(randomSuffix); err != nil {
		_ = adminDB.Close()
		t.Fatalf("テスト用データベース名の生成に失敗: %v", err)
	}
	databaseName := fmt.Sprintf("claim_session_test_%x", randomSuffix)
	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE `"+databaseName+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = adminDB.Close()
		t.Fatalf("隔離されたテスト用データベースの作成に失敗: %v", err)
	}

	config.DBName = databaseName
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{DSN: config.FormatDSN()}), &gorm.Config{})
	if err != nil {
		_, _ = adminDB.ExecContext(context.Background(), "DROP DATABASE `"+databaseName+"`")
		_ = adminDB.Close()
		t.Fatalf("GORM接続の作成に失敗: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("SQL接続の取得に失敗: %v", err)
	}
	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetMaxIdleConns(32)

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("テスト用DB接続のクローズに失敗: %v", err)
		}
		if _, err := adminDB.ExecContext(context.Background(), "DROP DATABASE `"+databaseName+"`"); err != nil {
			t.Errorf("テスト用データベースの削除に失敗: %v", err)
		}
		if err := adminDB.Close(); err != nil {
			t.Errorf("MySQL管理接続のクローズに失敗: %v", err)
		}
	})

	if err := db.Exec(`
		CREATE TABLE session_validations (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			session_id varchar(255) NOT NULL,
			user_id bigint unsigned DEFAULT NULL,
			invalid_answer_count bigint DEFAULT '0',
			is_terminated tinyint(1) DEFAULT '0',
			last_invalid_answer_time datetime DEFAULT NULL,
			created_at datetime(3) DEFAULT NULL,
			updated_at datetime(3) DEFAULT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY idx_session_validations_session_id (session_id),
			KEY idx_session_validations_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`).Error; err != nil {
		t.Fatalf("session_validations テーブルの作成に失敗: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE chat_messages (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			organization_id bigint unsigned DEFAULT NULL,
			session_id varchar(100) NOT NULL,
			user_id bigint unsigned NOT NULL,
			role varchar(20) NOT NULL,
			content text NOT NULL,
			question_weight_id bigint unsigned DEFAULT NULL,
			weight_category varchar(100) DEFAULT NULL,
			created_at datetime(3) DEFAULT NULL,
			PRIMARY KEY (id),
			KEY idx_chat_messages_session_id (session_id),
			KEY idx_chat_messages_user_id (user_id),
			KEY idx_chat_messages_question_weight_id (question_weight_id),
			KEY idx_chat_messages_organization_id (organization_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`).Error; err != nil {
		t.Fatalf("chat_messages テーブルの作成に失敗: %v", err)
	}

	return db, NewSessionValidationRepository(db)
}
