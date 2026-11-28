package user_test

// 希望条件（#1094）の取得・保存のテスト。
// 呼ぶ SQL の順（希望条件 → 公開同意）と、未設定時に空の希望条件を 200 で返すことを確かめる。

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"Backend/internal/controllers/testsupport"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

var (
	selectPreference = regexp.QuoteMeta("SELECT * FROM `user_preferences` WHERE user_id = ?")
	selectScoutAllow = regexp.QuoteMeta("SELECT `allow_scout_visibility` FROM `users` WHERE id = ?")
	upsertPreference = regexp.QuoteMeta("INSERT INTO `user_preferences`")
	updateScoutAllow = regexp.QuoteMeta("UPDATE `users` SET `allow_scout_visibility`=?,`updated_at`=? WHERE id = ?")
)

func decodePreference(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("JSON でない: %q", rec.Body.String())
	}
	return body
}

func getPreferenceRequest(userID uint) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/user/preferences", nil)
	if userID != 0 {
		req = testsupport.WithUserID(req, userID)
	}
	return req
}

func putPreferenceRequest(body string, userID uint) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/api/user/preferences", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		req = testsupport.WithUserID(req, userID)
	}
	return req
}

func TestGetPreference(t *testing.T) {
	t.Run("設定済みの希望条件と公開同意を返す", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		mock.ExpectQuery(selectPreference).WithArgs(uint(7), 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "desired_location", "note"}).AddRow(3, 7, "テスト県", "メモ"))
		mock.ExpectQuery(selectScoutAllow).WithArgs(uint(7)).
			WillReturnRows(sqlmock.NewRows([]string{"allow_scout_visibility"}).AddRow(true))

		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.Get, testsupport.NewCtx(getPreferenceRequest(7), rec), http.StatusOK)
		body := decodePreference(t, rec)
		assert.Equal(t, "テスト県", body["desired_location"])
		assert.Equal(t, "メモ", body["note"])
		assert.Equal(t, true, body["allow_scout_visibility"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("未設定なら空の希望条件を200で返す", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		mock.ExpectQuery(selectPreference).WithArgs(uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectQuery(selectScoutAllow).WithArgs(uint(7)).
			WillReturnRows(sqlmock.NewRows([]string{"allow_scout_visibility"}).AddRow(false))

		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.Get, testsupport.NewCtx(getPreferenceRequest(7), rec), http.StatusOK)
		body := decodePreference(t, rec)
		assert.Equal(t, float64(7), body["user_id"])
		assert.Equal(t, "", body["desired_location"])
		assert.Equal(t, false, body["allow_scout_visibility"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("希望条件が読めなければ500", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		mock.ExpectQuery(selectPreference).WithArgs(uint(7), 1).WillReturnError(errors.New("db down"))
		testsupport.AssertStatus(t, c.Get, testsupport.NewCtx(getPreferenceRequest(7), httptest.NewRecorder()), http.StatusInternalServerError)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("公開同意が読めなければ500", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		mock.ExpectQuery(selectPreference).WithArgs(uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectQuery(selectScoutAllow).WithArgs(uint(7)).WillReturnError(errors.New("db down"))
		testsupport.AssertStatus(t, c.Get, testsupport.NewCtx(getPreferenceRequest(7), httptest.NewRecorder()), http.StatusInternalServerError)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("未認証は401", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		testsupport.AssertStatus(t, c.Get, testsupport.NewCtx(getPreferenceRequest(0), httptest.NewRecorder()), http.StatusUnauthorized)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPutPreference(t *testing.T) {
	t.Run("公開同意を指定すれば希望条件→同意の更新→同意の読み直しの順に呼ぶ", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		mock.ExpectBegin()
		mock.ExpectExec(upsertPreference).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec(updateScoutAllow).WithArgs(true, sqlmock.AnyArg(), uint(7)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(selectScoutAllow).WithArgs(uint(7)).
			WillReturnRows(sqlmock.NewRows([]string{"allow_scout_visibility"}).AddRow(true))

		rec := httptest.NewRecorder()
		body := `{"desired_location":"  テスト県  ","note":" メモ ","allow_scout_visibility":true}`
		testsupport.AssertStatus(t, c.Put, testsupport.NewCtx(putPreferenceRequest(body, 7), rec), http.StatusOK)
		got := decodePreference(t, rec)
		assert.Equal(t, "テスト県", got["desired_location"], "前後の空白は落とす")
		assert.Equal(t, "メモ", got["note"])
		assert.Equal(t, true, got["allow_scout_visibility"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("公開同意を省略すれば同意は更新せず現在値を返す", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		mock.ExpectBegin()
		mock.ExpectExec(upsertPreference).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(selectScoutAllow).WithArgs(uint(7)).
			WillReturnRows(sqlmock.NewRows([]string{"allow_scout_visibility"}).AddRow(false))

		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.Put, testsupport.NewCtx(putPreferenceRequest(`{"desired_location":"テスト県"}`, 7), rec), http.StatusOK)
		assert.Equal(t, false, decodePreference(t, rec)["allow_scout_visibility"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("保存に失敗すれば500で同意には触れない", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		mock.ExpectBegin()
		mock.ExpectExec(upsertPreference).WillReturnError(errors.New("db down"))
		mock.ExpectRollback()
		testsupport.AssertStatus(t, c.Put, testsupport.NewCtx(putPreferenceRequest(`{"allow_scout_visibility":true}`, 7), httptest.NewRecorder()), http.StatusInternalServerError)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("希望勤務地が長すぎれば400で何も保存しない", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		body := `{"desired_location":"` + strings.Repeat("あ", 101) + `"}`
		testsupport.AssertStatus(t, c.Put, testsupport.NewCtx(putPreferenceRequest(body, 7), httptest.NewRecorder()), http.StatusBadRequest)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("未認証は401", func(t *testing.T) {
		c, mock := newTrainingConsentController(t)
		testsupport.AssertStatus(t, c.Put, testsupport.NewCtx(putPreferenceRequest(`{}`, 0), httptest.NewRecorder()), http.StatusUnauthorized)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
