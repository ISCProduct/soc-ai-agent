package interview

import (
	"errors"
	"testing"

	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// roleUserRepoStub は actor ごとの User を返すだけのスタブ（#1814）。
// 登録のない ID は DB エラー扱いにして fail-close を検証できるようにする。
type roleUserRepoStub struct {
	repository.UserRepository
	users map[uint]*entity.User
}

func (r roleUserRepoStub) GetUserByID(id uint) (*entity.User, error) {
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("user not found")
}

// TestTeacherReportVisibility は教員用レポートを返すかを「サーバー側の実ロール」だけで決める
// 回帰テスト（#1814）。クライアントが role を送れる経路は無いので、呼び出し元の
// userID だけで結果が決まることを、詳細取得とレポート単体取得の両方で確認する。
func TestTeacherReportVisibility(t *testing.T) {
	t.Parallel()

	const (
		ownerID   = uint(1)
		sessionID = uint(10)
		secret    = `{"coaching":"secret"}`
	)
	owner := &entity.User{ID: ownerID, Role: models.UserRoleStudent}
	staffOwner := &entity.User{ID: 2, Role: models.UserRoleStaff}
	admin := &entity.User{ID: 3, Role: models.UserRoleStudent, IsAdmin: true}

	tests := []struct {
		name       string
		session    uint // セッション所有者
		actor      uint
		users      map[uint]*entity.User
		wantSecret bool
		wantErr    error
	}{
		{"生徒本人には教員用レポートを返さない", ownerID, ownerID, map[uint]*entity.User{ownerID: owner}, false, nil},
		{"職員本人には教員用レポートを返す", 2, 2, map[uint]*entity.User{2: staffOwner}, true, nil},
		{"管理者には教員用レポートを返す", ownerID, 3, map[uint]*entity.User{ownerID: owner, 3: admin}, true, nil},
		{"ユーザーを引けない本人には返さない(fail-close)", ownerID, ownerID, map[uint]*entity.User{}, false, nil},
		{"他人のセッションは403", ownerID, 9, map[uint]*entity.User{9: {ID: 9, Role: models.UserRoleStudent}}, false, shared.ErrForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: sessionID, UserID: tt.session})
			reportRepo := &reportRepoStub{last: &models.InterviewReport{SessionID: sessionID, TeacherReportJSON: secret}}
			svc := NewInterviewService(sessionRepo, &utterRepoStub{}, reportRepo, roleUserRepoStub{users: tt.users}, nil, nil, nil)

			detail, derr := svc.GetSessionDetail(tt.actor, sessionID)
			report, rerr := svc.GetReport(tt.actor, sessionID)

			if tt.wantErr != nil {
				if !errors.Is(derr, tt.wantErr) || !errors.Is(rerr, tt.wantErr) {
					t.Fatalf("detail err=%v report err=%v want %v", derr, rerr, tt.wantErr)
				}
				return
			}
			if derr != nil || rerr != nil {
				t.Fatalf("unexpected err detail=%v report=%v", derr, rerr)
			}
			if got := detail.Report.TeacherReportJSON == secret; got != tt.wantSecret {
				t.Errorf("詳細取得: 教員用レポート開示=%v want %v", got, tt.wantSecret)
			}
			if got := report.TeacherReportJSON == secret; got != tt.wantSecret {
				t.Errorf("レポート単体取得: 教員用レポート開示=%v want %v", got, tt.wantSecret)
			}
			// 取り除いてもリポジトリ側(共有ポインタ)を書き換えない
			if reportRepo.last.TeacherReportJSON != secret {
				t.Errorf("元のレポートを破壊している")
			}
		})
	}
}
