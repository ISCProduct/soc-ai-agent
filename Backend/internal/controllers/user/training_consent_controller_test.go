package user_test

// 学習利用の同意（#2167）の取得・更新と、同意の履歴（#2168）のテスト。
//
// 同意は opt-in で、明示的な操作でだけ変える。送り忘れ（項目なし）を false として
// 保存しないこと、希望条件（user_preferences）には触れないことを確かめる。
// 保存のたびに、値の更新と履歴の1行（user_id・値・文言の版）を同じトランザクションで書き、
// どちらかが失敗すれば両方とも巻き戻して 500 を返すことを確かめる。
// 画面が表示していた文言の版（consent_text_version）が今の版と違えば、何も保存せずに 409 を返すことを確かめる（#2262 のレビュー）。

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"Backend/internal/controllers/testsupport"
	usercontrollers "Backend/internal/controllers/user"
	"Backend/internal/models"
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
	updateTrainingUse = regexp.QuoteMeta("UPDATE `users` SET `allow_training_use`=?,`updated_at`=? WHERE id = ?")
	// created_at はアプリの時計ではなく DB の時計（NOW(3)）で入れる。引数に日時が無いことも文全体で照合する。
	insertConsentEvent = regexp.QuoteMeta(
		"INSERT INTO `training_consent_events` (`user_id`,`allow_training_use`,`consent_text_version`,`created_at`) VALUES (?,?,?,NOW(3))")
	// 現在値と最後の操作の日時を1つの文（同じスナップショット）で読む。最後の操作は id の最大で決める。
	// 別々の文に分けたり、created_at の順で決めたりすると照合に失敗する。
	selectConsentState = "^" + regexp.QuoteMeta(
		"SELECT u.allow_training_use, e.created_at AS changed_at FROM `users` u "+
			"LEFT JOIN `training_consent_events` e ON e.id = "+
			"(SELECT MAX(id) FROM `training_consent_events` WHERE user_id = u.id) "+
			"WHERE u.id = ?") + "$"
)

// consentChangedAt は、履歴の最後の行の日時として返すテスト用の固定値。
var consentChangedAt = time.Date(2026, 11, 12, 9, 30, 15, 0, time.UTC)

// consentStateRows は、現在値と最後の操作の日時（履歴が無ければ NULL）の1行を返す。
func consentStateRows(allow bool, changedAt *time.Time) *sqlmock.Rows {
	var at driver.Value
	if changedAt != nil {
		at = *changedAt
	}
	return sqlmock.NewRows([]string{"allow_training_use", "changed_at"}).AddRow(allow, at)
}

// consentPutBody は、画面が送る PUT の本文（値と、表示していた文言の版）。
func consentPutBody(allow bool, version string) string {
	return fmt.Sprintf(`{"allow_training_use":%t,"consent_text_version":%q}`, allow, version)
}

// validPutBody は、今の文言の版で同意する本文。
var validPutBody = consentPutBody(true, models.TrainingConsentTextVersion)

func decodeTrainingConsent(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("JSON でない: %q", rec.Body.String())
	}
	return body
}

func putTrainingConsentRequest(body string, userID uint) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/api/user/training-consent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		req = testsupport.WithUserID(req, userID)
	}
	return req
}

// GET は、同意の現在値と、本人が最後に変更した日時を返す。履歴が無ければ updated_at は null。
// users.updated_at（ほかの更新でも変わる）ではなく、履歴の最後の行の日時であること。
func TestGetTrainingConsent(t *testing.T) {
	cases := []struct {
		name      string
		allow     bool
		changedAt *time.Time
		wantAt    any
	}{
		{name: "一度も変更していない（初期値の未同意）", allow: false, changedAt: nil, wantAt: nil},
		{name: "同意済み", allow: true, changedAt: &consentChangedAt, wantAt: consentChangedAt.Format(time.RFC3339)},
		{name: "取り消し済み", allow: false, changedAt: &consentChangedAt, wantAt: consentChangedAt.Format(time.RFC3339)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, mock := newTrainingConsentController(t)
			mock.ExpectQuery(selectConsentState).WithArgs(uint(7)).WillReturnRows(consentStateRows(tc.allow, tc.changedAt))

			req := testsupport.WithUserID(httptest.NewRequest(http.MethodGet, "/api/user/training-consent", nil), 7)
			rec := httptest.NewRecorder()
			testsupport.AssertStatus(t, c.GetTrainingConsent, testsupport.NewCtx(req, rec), http.StatusOK)

			body := decodeTrainingConsent(t, rec)
			assert.Equal(t, tc.allow, body["allow_training_use"])
			at, present := body["updated_at"]
			assert.True(t, present, "updated_at は null でも項目として返す（OpenAPI の required）")
			assert.Equal(t, tc.wantAt, at)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetTrainingConsent_Unauthorized(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	req := httptest.NewRequest(http.MethodGet, "/api/user/training-consent", nil)
	testsupport.AssertStatus(t, c.GetTrainingConsent, testsupport.NewCtx(req, httptest.NewRecorder()), http.StatusUnauthorized)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 読めなければ 500 にする（updated_at を null にして「一度も変更していない」と見せない）。
func TestGetTrainingConsent_ReadError(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	mock.ExpectQuery(selectConsentState).WithArgs(uint(7)).WillReturnError(errors.New("db down"))
	req := testsupport.WithUserID(httptest.NewRequest(http.MethodGet, "/api/user/training-consent", nil), 7)
	testsupport.AssertStatus(t, c.GetTrainingConsent, testsupport.NewCtx(req, httptest.NewRecorder()), http.StatusInternalServerError)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ユーザーの行が無い（通常は起きない）ときは、以前と同じく「同意しない」・日時なしを返す。
func TestGetTrainingConsent_NoUserRow(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	mock.ExpectQuery(selectConsentState).WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"allow_training_use", "changed_at"}))
	req := testsupport.WithUserID(httptest.NewRequest(http.MethodGet, "/api/user/training-consent", nil), 7)
	rec := httptest.NewRecorder()
	testsupport.AssertStatus(t, c.GetTrainingConsent, testsupport.NewCtx(req, rec), http.StatusOK)
	body := decodeTrainingConsent(t, rec)
	assert.Equal(t, false, body["allow_training_use"])
	assert.Nil(t, body["updated_at"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 同意する・取り消す、のどちらも、同意のカラムの更新と履歴の1行（user_id・値・画面が送った文言の版・DB の今の日時）を
// 同じトランザクションで書いてコミットし、更新後の値と最後に変更した日時を返す。
// ほかのクエリ（user_preferences の保存など）が走ると sqlmock が失敗する。
// 今の値を読まずに書くので、同じ値への変更（同意済みの人がもう一度同意する）も必ず1行残る。
func TestPutTrainingConsent_UpdatesConsentAndRecordsEvent(t *testing.T) {
	for _, allow := range []bool{true, false} {
		t.Run(map[bool]string{true: "同意する", false: "取り消す"}[allow], func(t *testing.T) {
			c, mock := newTrainingConsentController(t)
			mock.ExpectBegin()
			mock.ExpectExec(updateTrainingUse).WithArgs(allow, sqlmock.AnyArg(), uint(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(insertConsentEvent).
				WithArgs(uint(7), allow, models.TrainingConsentTextVersion).
				WillReturnResult(sqlmock.NewResult(3, 1))
			mock.ExpectCommit()
			mock.ExpectQuery(selectConsentState).WithArgs(uint(7)).WillReturnRows(consentStateRows(allow, &consentChangedAt))

			rec := httptest.NewRecorder()
			testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(putTrainingConsentRequest(consentPutBody(allow, models.TrainingConsentTextVersion), 7), rec), http.StatusOK)

			got := decodeTrainingConsent(t, rec)
			assert.Equal(t, allow, got["allow_training_use"])
			assert.Equal(t, consentChangedAt.Format(time.RFC3339), got["updated_at"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 文言の版は、画面の文言を変えたら上げる約束の値。空のまま記録すると「どの文言に同意したか」が残らない。
func TestTrainingConsentTextVersion_IsSet(t *testing.T) {
	if _, err := time.Parse("2006-01-02", models.TrainingConsentTextVersion); err != nil {
		t.Errorf("文言の版は日付（YYYY-MM-DD）で持つ: %q", models.TrainingConsentTextVersion)
	}
	if len(models.TrainingConsentTextVersion) > 32 {
		t.Errorf("consent_text_version（varchar(32)）に入らない: %q", models.TrainingConsentTextVersion)
	}
}

// 値・文言の版が無い・null・JSON でないときは 400 にし、DB には触れない（送り忘れを「同意しない」で上書きしない。
// どの文言を見て押したか分からない操作を記録しない）。
func TestPutTrainingConsent_RejectsMissingValue(t *testing.T) {
	v := models.TrainingConsentTextVersion
	for name, body := range map[string]string{
		"項目なし":        `{}`,
		"値が null":     `{"allow_training_use":null,"consent_text_version":"` + v + `"}`,
		"値なし":         `{"consent_text_version":"` + v + `"}`,
		"JSON でない":    `allow_training_use=true`,
		"真偽値でない文字列":   `{"allow_training_use":"yes","consent_text_version":"` + v + `"}`,
		"版なし（古いフロント）": `{"allow_training_use":true}`,
		"版が空":         `{"allow_training_use":true,"consent_text_version":""}`,
		"版が null":     `{"allow_training_use":true,"consent_text_version":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			c, mock := newTrainingConsentController(t)
			testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(putTrainingConsentRequest(body, 7), httptest.NewRecorder()), http.StatusBadRequest)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 画面が表示していた文言の版が今の版と違う（古いタブ・古いフロント、版を上げた直後）なら、
// 何も保存せずに 409 を返す。古い文言を見て押した操作を、新しい文言への同意として記録しない。
// 取り消し（false）も同じに扱う（どちらも「どの文言に対する操作か」を記録するため）。
func TestPutTrainingConsent_RejectsStaleTextVersion(t *testing.T) {
	for name, body := range map[string]string{
		"古い版で同意":   consentPutBody(true, "2026-01-01"),
		"古い版で取り消し": consentPutBody(false, "2026-01-01"),
		"記録の前の版":   consentPutBody(true, models.TrainingConsentLegacyTextVersion),
		"前後に空白":    consentPutBody(true, " "+models.TrainingConsentTextVersion),
	} {
		t.Run(name, func(t *testing.T) {
			c, mock := newTrainingConsentController(t)
			testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(putTrainingConsentRequest(body, 7), httptest.NewRecorder()), http.StatusConflict)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPutTrainingConsent_Unauthorized(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(putTrainingConsentRequest(validPutBody, 0), httptest.NewRecorder()), http.StatusUnauthorized)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 書き込みのどこかで失敗したら、トランザクションを巻き戻して 500 を返し、保存できたように見せない。
//   - 値の更新が失敗：履歴を残さない（操作が成立していないのに記録だけ残さない）
//   - 履歴の追加が失敗：値の更新も巻き戻す（記録の無い同意を残さない）
//   - コミットが失敗：どちらも確定していない
//
// どの場合も、読み直し（200 で保存後の値を返す処理）まで進まない。
func TestPutTrainingConsent_WriteErrors_RollBack(t *testing.T) {
	for _, tc := range writeErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			c, mock := newTrainingConsentController(t)
			tc.expect(mock)
			testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(putTrainingConsentRequest(validPutBody, 7), httptest.NewRecorder()), http.StatusInternalServerError)
			// 期待どおりの順（BEGIN → … → ROLLBACK）で、すべて実行されたこと
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 上のテストは順序どおりに照合するので、失敗の後に余計な SQL が走っても sqlmock のエラーで 500 になり、
// 「失敗を無視して先へ進む」不具合を見分けられない。ここでは順不同で照合し、失敗の後に来うる
// 書き込み・読み直しが成功する期待を置いて、それらが一度も実行されないことを確かめる。
//   - 値の更新が失敗したのに履歴を追加する（記録だけが残る）
//   - 保存に失敗したのに読み直して 200 を返す（保存できたように見える）
func TestPutTrainingConsent_WriteErrors_DoNotContinue(t *testing.T) {
	for _, tc := range writeErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			c, mock := newTrainingConsentController(t)
			mock.MatchExpectationsInOrder(false)
			tc.expect(mock)
			var inserted, readBack []driver.Value
			if tc.updateFails {
				mock.ExpectExec(insertConsentEvent).WithArgs(recordArg{&inserted}, sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(3, 1))
			}
			mock.ExpectQuery(selectConsentState).WithArgs(recordArg{&readBack}).WillReturnRows(consentStateRows(true, &consentChangedAt))

			testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(putTrainingConsentRequest(validPutBody, 7), httptest.NewRecorder()), http.StatusInternalServerError)
			assert.Empty(t, inserted, "値の更新に失敗したのに履歴を追加した")
			assert.Empty(t, readBack, "保存に失敗したのに読み直した")
		})
	}
}

// recordArg は、どの値にも当てはまり、渡された値を記録する（その SQL が実行されたかを後から確かめる）。
type recordArg struct{ got *[]driver.Value }

func (a recordArg) Match(v driver.Value) bool {
	*a.got = append(*a.got, v)
	return true
}

// writeErrorCases は、保存のトランザクションのどこかで失敗する場合の、期待する SQL の並び。
var writeErrorCases = []struct {
	name        string
	updateFails bool
	expect      func(sqlmock.Sqlmock)
}{
	{name: "値の更新が失敗", updateFails: true, expect: func(m sqlmock.Sqlmock) {
		m.ExpectBegin()
		m.ExpectExec(updateTrainingUse).WillReturnError(errors.New("db down"))
		m.ExpectRollback()
	}},
	{name: "履歴の追加が失敗", expect: func(m sqlmock.Sqlmock) {
		m.ExpectBegin()
		m.ExpectExec(updateTrainingUse).WithArgs(true, sqlmock.AnyArg(), uint(7)).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec(insertConsentEvent).WillReturnError(errors.New("db down"))
		m.ExpectRollback()
	}},
	{name: "コミットが失敗", expect: func(m sqlmock.Sqlmock) {
		m.ExpectBegin()
		m.ExpectExec(updateTrainingUse).WithArgs(true, sqlmock.AnyArg(), uint(7)).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec(insertConsentEvent).WillReturnResult(sqlmock.NewResult(3, 1))
		m.ExpectCommit().WillReturnError(errors.New("db down"))
	}},
}

// 保存はできたが読み直しに失敗したときも 500 にする（保存後の値を決め打ちで返さない。画面は取り直す）。
func TestPutTrainingConsent_ReadBackError(t *testing.T) {
	c, mock := newTrainingConsentController(t)
	mock.ExpectBegin()
	mock.ExpectExec(updateTrainingUse).WithArgs(true, sqlmock.AnyArg(), uint(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(insertConsentEvent).WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(selectConsentState).WithArgs(uint(7)).WillReturnError(errors.New("db down"))

	testsupport.AssertStatus(t, c.PutTrainingConsent, testsupport.NewCtx(putTrainingConsentRequest(validPutBody, 7), httptest.NewRecorder()), http.StatusInternalServerError)
	assert.NoError(t, mock.ExpectationsWereMet())
}
