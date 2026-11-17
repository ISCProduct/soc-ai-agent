package repositories

import (
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"Backend/internal/models"
)

// captureArg は SQL の引数を順に記録する。引数の数だけ並べて使う。
type captureArg struct{ got *[]driver.Value }

func (c captureArg) Match(v driver.Value) bool { *c.got = append(*c.got, v); return true }

func captureArgs(n int, got *[]driver.Value) []driver.Value {
	args := make([]driver.Value, n)
	for i := range args {
		args[i] = captureArg{got}
	}
	return args
}

func containsTime(vals []driver.Value, want time.Time) bool {
	for _, v := range vals {
		if tv, ok := v.(time.Time); ok && tv.Equal(want) {
			return true
		}
	}
	return false
}

func persistedUser() (*models.User, map[string]time.Time) {
	at := time.Date(2026, 9, 13, 1, 2, 3, 0, time.UTC)
	exp := time.Date(2026, 11, 14, 4, 5, 6, 0, time.UTC)
	verified := time.Date(2026, 11, 13, 7, 8, 9, 0, time.UTC)
	u := &models.User{
		ID: 7, OrganizationID: 3, Email: "u@example.com", Name: "n", Role: models.UserRoleStaff,
		AdminTokenNotBefore:      &at,
		EmailVerificationExpires: &exp,
		EmailVerifiedAt:          &verified,
		AllowCollectiveInsight:   true, AllowScoutVisibility: true, AllowTrainingUse: true,
	}
	return u, map[string]time.Time{"admin_token_not_before": at, "email_verification_expires": exp, "email_verified_at": verified}
}

// UpdateUser は Save によるフルロー更新。ユーザーの項目が1つでも書き込みから落ちると、
// プロフィール更新や再ログインのたびにその値が初期値へ戻る。
// 失効時刻(#1155)・同意フラグ(#2180)・メール認証の期限(#330)を、書き込みの引数で固定する。
func TestUpdateUser_全カラムを書き込む(t *testing.T) {
	db, mock := newMockDB(t)
	u, times := persistedUser()

	cols := userColumnsInSaveOrder(t, db)
	var got []driver.Value
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET").
		WithArgs(captureArgs(len(cols)+1, &got)...). // 全カラム + WHERE id
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, NewUserRepository(db).UpdateUser(u))
	require.NoError(t, mock.ExpectationsWereMet())

	byCol := map[string]driver.Value{}
	for i, c := range cols {
		byCol[c] = got[i]
	}
	for col, want := range times {
		tv, ok := byCol[col].(time.Time)
		require.Truef(t, ok && tv.Equal(want), "%s が書き込まれていない: %v", col, byCol[col])
	}
	for _, col := range []string{"allow_collective_insight", "allow_scout_visibility", "allow_training_use"} {
		require.Equalf(t, true, byCol[col], "%s が書き込まれていない", col)
	}
	require.Equal(t, models.UserRoleStaff, byCol["role"])
	require.EqualValues(t, 7, got[len(cols)], "WHERE id")
}

// 呼び出し元の構造体は Save で書き換えない(Save は UpdatedAt を詰め直す)。
func TestUpdateUser_引数の構造体を書き換えない(t *testing.T) {
	db, mock := newMockDB(t)
	u, _ := persistedUser()
	var got []driver.Value
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET").
		WithArgs(captureArgs(len(userColumnsInSaveOrder(t, db))+1, &got)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, NewUserRepository(db).UpdateUser(u))
	require.True(t, u.UpdatedAt.IsZero(), "UpdatedAt が呼び出し元に書き戻された")
}

func TestCreateUser_期限と同意を書き込みIDを返す(t *testing.T) {
	db, mock := newMockDB(t)
	u, times := persistedUser()
	u.ID, u.OrganizationID = 0, 0 // 既定の組織へ所属する

	var got []driver.Value
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `users`").
		WithArgs(captureArgs(len(userColumnsInSaveOrder(t, db)), &got)...).
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("INSERT INTO `organization_memberships`").
		WithArgs(models.DefaultOrganizationID, uint(42), models.OrgRoleMember, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, NewUserRepository(db).CreateUser(u))
	require.NoError(t, mock.ExpectationsWereMet())
	for col, want := range times {
		require.Truef(t, containsTime(got, want), "%s が INSERT に含まれていない", col)
	}
	require.EqualValues(t, 42, u.ID)
	require.Equal(t, models.DefaultOrganizationID, u.OrganizationID)
}

// 組織メンバーの作成に失敗してロールバックしたとき、呼び出し元の構造体は元のまま。
func TestCreateUser_失敗したら引数の構造体を変えない(t *testing.T) {
	db, mock := newMockDB(t)
	u, _ := persistedUser()
	u.ID, u.OrganizationID = 0, 0

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `users`").WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("INSERT INTO `organization_memberships`").WillReturnError(errors.New("boom"))
	mock.ExpectRollback()

	require.Error(t, NewUserRepository(db).CreateUser(u))
	require.Zero(t, u.ID, "ロールバックしたのに ID が書き戻された")
	require.Zero(t, u.OrganizationID)
}

// Save が更新するカラムの順(主キーを除くスキーマ順)。INSERT も、ID が 0 のとき同じ並びになる。
func userColumnsInSaveOrder(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	stmt := &gorm.Statement{DB: db}
	require.NoError(t, stmt.Parse(&models.User{}))
	var cols []string
	for _, c := range stmt.Schema.DBNames {
		if c != "id" {
			cols = append(cols, c)
		}
	}
	return cols
}

// 管理者として作るユーザーは、組織のオーナーとして所属する。
func TestCreateUser_管理者は組織のオーナーになる(t *testing.T) {
	db, mock := newMockDB(t)
	u, _ := persistedUser()
	u.ID, u.IsAdmin = 0, true

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `users`").WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("INSERT INTO `organization_memberships`").
		WithArgs(u.OrganizationID, uint(42), models.OrgRoleOwner, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, NewUserRepository(db).CreateUser(u))
	require.NoError(t, mock.ExpectationsWereMet())
}
