package user_test

// 学習利用の同意（#2167）の取得・更新のテスト。
//
// 同意は opt-in で、明示的な操作でだけ変える。送り忘れ（項目なし）を false として
// 保存しないこと、希望条件（user_preferences）には触れないことを確かめる。

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"Backend/internal/controllers/testsupport"
	usercontrollers "Backend/internal/controllers/user"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTrainingConsentController(t *testing.T) (*usercontrollers.UserPreferenceController, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return usercontrollers.NewUserPreferenceController(repositories.NewUserPreferenceRepository(db), nil, nil), mock
}

var (
	selectTrainingUse = regexp.QuoteMeta("SELECT `allow_training_use` FROM `users` WHERE id = ?")
	updateTrainingUse = regexp.QuoteMeta("UPDATE `users` SET `allow_training_use`=?,`updated_at`=? WHERE id = ?")
)

func trainingUseRows(allow bool) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"allow_training_use"}).AddRow(allow)
}

func decodeTrainingConsent(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("JSON でない: %q", rec.Body.String())
	}
	return body
}

func TestGetTrainingConsent(t *testing.T) {
	for _, allow := range []bool{false, true} {
		c, mock := newTrainingConsentController(t)
		mock.ExpectQuery(selectTrainingUse).WithArgs(uint(7)).WillReturnRows(trainingUseRows(allow))

		req := testsupport.WithUserID(httptest.NewRequest(http.MethodGet, "/api/user/training-consent", nil), 7)
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.GetTrainingConsent, testsupport.NewCtx(req, rec), http.StatusOK)

		assert.Equal(t, allow, decodeTrainingConsent(t, rec)["allow_training_use"])
		assert.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestGetTrainingConsent_Unauthorized(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	req := httptest.NewRequest(http.MethodGet, "/api/user/training-consent", nil)
	testsupport.AssertStatus(t, c.GetTrainingConsent, testsupport.NewCtx(req, httptest.NewRecorder()), http.StatusUnauthorized)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 同意する・取り消す、のどちらも、同意のカラムだけを更新して更新後の値を返す。
// ほかのクエリ（user_preferences の保存など）が走ると sqlmock が失敗する。
func TestPutTrainingConsent_UpdatesOnlyTheConsentColumn(t *testing.T) {
	for _, allow := range []bool{true, false} {
		c, mock := newTrainingConsentController(t)
		mock.ExpectBegin()
		mock.ExpectExec(updateTrainingUse).WithArgs(allow, sqlmock.AnyArg(), uint(7)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(selectTrainingUse).WithArgs(uint(7)).WillReturnRows(trainingUseRows(allow))

		body := `{"allow_training_use":false}`
		if allow {
			body = `{"allow_training_use":true}`
		}
		req := httptest.NewRequest(http.MethodPut, "/api/user/training-consent", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = testsupport.WithUserID(req, 7)
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(req, rec), http.StatusOK)

		assert.Equal(t, allow, decodeTrainingConsent(t, rec)["allow_training_use"])
		assert.NoError(t, mock.ExpectationsWereMet())
	}
}

// 項目が無い・null・JSON でないときは 400 にし、DB には触れない（送り忘れを「同意しない」で上書きしない）。
func TestPutTrainingConsent_RejectsMissingValue(t *testing.T) {
	for name, body := range map[string]string{
		"項目なし":      `{}`,
		"null":      `{"allow_training_use":null}`,
		"JSON でない":  `allow_training_use=true`,
		"真偽値でない文字列": `{"allow_training_use":"yes"}`,
	} {
		t.Run(name, func(t *testing.T) {
			c, mock := newTrainingConsentController(t)
			req := httptest.NewRequest(http.MethodPut, "/api/user/training-consent", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = testsupport.WithUserID(req, 7)
			testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(req, httptest.NewRecorder()), http.StatusBadRequest)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPutTrainingConsent_Unauthorized(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	req := httptest.NewRequest(http.MethodPut, "/api/user/training-consent", strings.NewReader(`{"allow_training_use":true}`))
	req.Header.Set("Content-Type", "application/json")
	testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(req, httptest.NewRecorder()), http.StatusUnauthorized)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 保存に失敗したら 500 を返し、保存できたように見せない。
func TestPutTrainingConsent_UpdateError(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	mock.ExpectBegin()
	mock.ExpectExec(updateTrainingUse).WillReturnError(errors.New("db down"))
	mock.ExpectRollback()
	// 失敗を握りつぶして読み直すと、ここで「同意済み」が返って 200 になってしまう。読み直しまで進まないこと
	mock.ExpectQuery(selectTrainingUse).WithArgs(uint(7)).WillReturnRows(trainingUseRows(true))

	req := httptest.NewRequest(http.MethodPut, "/api/user/training-consent", strings.NewReader(`{"allow_training_use":true}`))
	req.Header.Set("Content-Type", "application/json")
	req = testsupport.WithUserID(req, 7)
	testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(req, httptest.NewRecorder()), http.StatusInternalServerError)
	assert.Error(t, mock.ExpectationsWereMet(), "保存に失敗したのに読み直している")
}
