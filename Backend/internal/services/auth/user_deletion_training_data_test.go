package auth

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/repositories"
)

// #2168: 退会のパージで、採点モデルの学習用に書き出し済みのデータ（S3）も消す。

type fakeTrainingDataDeleter struct {
	calls []uint
	n     int
	err   error
}

func (f *fakeTrainingDataDeleter) DeleteUser(_ context.Context, userID uint) (int, error) {
	f.calls = append(f.calls, userID)
	return f.n, f.err
}

type purgeAuditCall struct {
	action string
	target uint
	meta   map[string]any
}

type fakePurgeAudit struct{ calls []purgeAuditCall }

func (a *fakePurgeAudit) Record(_, action, _ string, targetID uint, metadata map[string]any) {
	a.calls = append(a.calls, purgeAuditCall{action: action, target: targetID, meta: metadata})
}

func purgeWithTrainingDeleter(t *testing.T, d TrainingDataDeleter) (*fakePurgeAudit, error) {
	t.Helper()
	p := newPurgeTestDB(t)
	p.mock.ExpectBegin()
	p.expectUser(9)
	p.mock.ExpectCommit()
	p.mock.ExpectBegin()
	p.mock.ExpectCommit()
	p.allowRest()
	audit := &fakePurgeAudit{}
	s := NewUserDeletionService(repositories.NewUserWithdrawalRepository(p.db), nil, audit)
	if d != nil {
		s.SetTrainingDataDeleter(d)
	}
	return audit, s.purgeOne(purgeTarget)
}

// TestPurgeOne_DeletesExportedTrainingData は、パージがその利用者の書き出し済みの学習データを消し、
// 消した数を監査ログに残すことを確かめる（受け入れ条件「退会 → パージ → S3 からも消える」）。
func TestPurgeOne_DeletesExportedTrainingData(t *testing.T) {
	d := &fakeTrainingDataDeleter{n: 3}
	audit, err := purgeWithTrainingDeleter(t, d)
	if err != nil {
		t.Fatalf("purgeOne: %v", err)
	}
	if len(d.calls) != 1 || d.calls[0] != purgeTarget.UserID {
		t.Fatalf("DeleteUser の呼び出し = %v, want [%d]", d.calls, purgeTarget.UserID)
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "user.purge" {
		t.Fatalf("監査ログ = %+v", audit.calls)
	}
	meta := audit.calls[0].meta
	if meta["training_data_deleted_files"] != 3 {
		t.Errorf("training_data_deleted_files = %v, want 3", meta["training_data_deleted_files"])
	}
	if _, ok := meta["training_data_error"]; ok {
		t.Errorf("成功なのに training_data_error がある: %v", meta)
	}
}

// TestPurgeOne_TrainingDataDeleteFailureIsRecorded は、学習データを消せなくてもパージ（DB の物理削除）は成功とし、
// 失敗を監査ログに残すことを確かめる（面接動画などの S3 の削除と同じ best-effort。消し損ねは Sweep が拾う）。
func TestPurgeOne_TrainingDataDeleteFailureIsRecorded(t *testing.T) {
	buf := capturePurgeLog(t)
	d := &fakeTrainingDataDeleter{n: 1, err: errors.New("s3 delete: AccessDenied <subject>")}
	audit, err := purgeWithTrainingDeleter(t, d)
	if err != nil {
		t.Fatalf("purgeOne: %v（学習データの削除の失敗でパージを失敗にしない）", err)
	}
	meta := audit.calls[0].meta
	if meta["training_data_error"] != "s3 delete: AccessDenied <subject>" || meta["training_data_deleted_files"] != 1 {
		t.Errorf("監査ログ = %v", meta)
	}
	if buf.Len() == 0 {
		t.Error("失敗のログが出ていない")
	}
}

// TestPurgeOne_WithoutTrainingDeleter は、書き出しの設定が無い環境（未設定）では消さず、監査ログにも項目を足さないことを確かめる。
func TestPurgeOne_WithoutTrainingDeleter(t *testing.T) {
	audit, err := purgeWithTrainingDeleter(t, nil)
	if err != nil {
		t.Fatalf("purgeOne: %v", err)
	}
	meta := audit.calls[0].meta
	for _, k := range []string{"training_data_deleted_files", "training_data_error"} {
		if _, ok := meta[k]; ok {
			t.Errorf("未設定なのに %s がある: %v", k, meta)
		}
	}
}

// TestPurgeOne_HardDeleteFailureDoesNotDeleteTrainingData は、DB の物理削除に失敗したら学習データも消さないことを確かめる
// （パージはやり直されるので、そのときに消す。DB の行が残ったまま学習データだけ消えた状態を作らない）。
func TestPurgeOne_HardDeleteFailureDoesNotDeleteTrainingData(t *testing.T) {
	p := newPurgeTestDB(t)
	p.mock.ExpectBegin()
	p.expectUser(9)
	p.mock.ExpectExec("DELETE FROM `pending_registrations`").WillReturnError(errors.New("db down"))
	p.mock.ExpectRollback()
	p.allowRest()
	d := &fakeTrainingDataDeleter{}
	s := NewUserDeletionService(repositories.NewUserWithdrawalRepository(p.db), nil, &fakePurgeAudit{})
	s.SetTrainingDataDeleter(d)
	if err := s.purgeOne(purgeTarget); err == nil {
		t.Fatal("物理削除の失敗がエラーになりません")
	}
	if len(d.calls) != 0 {
		t.Errorf("物理削除に失敗したのに学習データを消しました: %v", d.calls)
	}
}
