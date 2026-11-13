package company

import (
	"errors"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/email"
)

// 掲載申請(Submit)の、保存する内容の組み立てを固定するテスト。
// 偽の Repository を使い、DB は要らない。
//
// 特に IsGuestEntry と draft は、ゲスト投稿を管理者の承認なしに公開しないための印で、
// 外れると、審査前の投稿が学生・無認証の画面に出てしまう(#1409)。

// entryRepoSpy は repository.CompanyEntryRepository の偽物。呼ばれた内容を記録する。
type entryRepoSpy struct {
	repository.CompanyEntryRepository

	draft         *repository.CompanyEntryDraft
	createErr     error
	submission    *models.CompanyEntrySubmission
	findSubErr    error
	company       *models.Company
	findCompanyEr error
	sentIDs       []uint
}

func (r *entryRepoSpy) CreateDraft(d repository.CompanyEntryDraft) error {
	r.draft = &d
	if r.createErr != nil {
		return r.createErr
	}
	d.Company.ID = 10
	d.Submission.ID = 20
	return nil
}
func (r *entryRepoSpy) FindSubmission(uint) (*models.CompanyEntrySubmission, error) {
	return r.submission, r.findSubErr
}
func (r *entryRepoSpy) FindCompanyName(uint) (*models.Company, error) {
	return r.company, r.findCompanyEr
}
func (r *entryRepoSpy) MarkInviteEmailSent(id uint, _ time.Time) error {
	r.sentIDs = append(r.sentIDs, id)
	return nil
}

func newEntrySubmitService(repo *entryRepoSpy) *CompanyEntryService {
	return &CompanyEntryService{
		entryRepo:   repo,
		userRepo:    entryUserRepoStub{user: &entity.User{ID: 1}}, // 既存ユーザー: 仮登録は作らない
		pendingRepo: entryPendingRepoStub{},
		email:       &email.EmailService{}, // host が空なので送らずにログだけ出し、成功を返す
	}
}

func validEntryInput() CompanyEntryInput {
	return CompanyEntryInput{
		Name:           "  株式会社テスト  ",
		ContactEmail:   " HR@Example.com ",
		ContactName:    " 担当 ",
		PrivacyConsent: true,
		SourceIP:       "203.0.113.5",
	}
}

func TestSubmit_CompanyIsDraftGuestEntry(t *testing.T) {
	repo := &entryRepoSpy{}
	res, err := newEntrySubmitService(repo).Submit(validEntryInput())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	c := repo.draft.Company
	if c.Name != "株式会社テスト" {
		t.Errorf("Name = %q, 前後の空白を除くこと", c.Name)
	}
	if !c.IsGuestEntry {
		t.Error("IsGuestEntry が false。審査前の投稿を隠すための印を外してはいけない(#1409)")
	}
	if c.DataStatus != "draft" || !c.IsProvisional || c.SourceType != "manual" || !c.IsActive {
		t.Errorf("DataStatus=%q IsProvisional=%v SourceType=%q IsActive=%v, want draft/true/manual/true", c.DataStatus, c.IsProvisional, c.SourceType, c.IsActive)
	}
	s := repo.draft.Submission
	if s.ContactEmail != "hr@example.com" || s.ContactName != "担当" {
		t.Errorf("ContactEmail=%q ContactName=%q, 小文字化と空白除去をすること", s.ContactEmail, s.ContactName)
	}
	if s.Status != "submitted" || s.EmailStatus != "pending" || s.SourceIP != "203.0.113.5" || s.PrivacyConsentAt.IsZero() {
		t.Errorf("申請: Status=%q EmailStatus=%q SourceIP=%q consentAt=%v", s.Status, s.EmailStatus, s.SourceIP, s.PrivacyConsentAt)
	}
	if res.CompanyID != 10 || res.SubmissionID != 20 || !res.EmailQueued {
		t.Errorf("結果 = %+v, want CompanyID=10 SubmissionID=20 EmailQueued=true", res)
	}
}

func TestSubmit_JobPositions(t *testing.T) {
	repo := &entryRepoSpy{}
	in := validEntryInput()
	in.JobPositions = []CompanyEntryJobInput{
		{Title: "  エンジニア  ", MinSalary: 300, MaxSalary: 500, RemoteOption: true},
		{Title: "   "}, // 題名が空の求人は作らない
		{Title: "営業"},
	}
	if _, err := newEntrySubmitService(repo).Submit(in); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	jobs := repo.draft.JobPositions
	if len(jobs) != 2 {
		t.Fatalf("求人 %d 件, want 2(題名が空のものは除く)", len(jobs))
	}
	if jobs[0].Title != "エンジニア" || jobs[0].MinSalary != 300 || !jobs[0].RemoteOption {
		t.Errorf("求人[0] = %+v", jobs[0])
	}
	for i, j := range jobs {
		if j.DataStatus != "draft" || !j.IsActive {
			t.Errorf("求人[%d]: DataStatus=%q IsActive=%v, want draft/true(公開は管理者の承認のあと)", i, j.DataStatus, j.IsActive)
		}
	}
}

func TestSubmit_WeightProfileAndGraduates(t *testing.T) {
	repo := &entryRepoSpy{}
	in := validEntryInput()
	in.WeightProfile = &CompanyEntryWeightInput{TechnicalOrientation: 4, CommunicationSkill: 2}
	in.Graduates = []CompanyEntryGraduateInput{
		{GraduateName: " 卒業生A ", GraduationYear: 2024, HiredAt: "2024-04-01"},
		{GraduateName: "卒業生B", HiredAt: "2024/04/01"}, // 形式が違う入社日は無視(エラーにしない)
		{GraduateName: "卒業生C", HiredAt: ""},
	}
	if _, err := newEntrySubmitService(repo).Submit(in); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	w := repo.draft.WeightProfile
	if w == nil || w.TechnicalOrientation != 4 || w.CommunicationSkill != 2 {
		t.Fatalf("重みプロファイル = %+v", w)
	}
	g := repo.draft.Graduates
	if len(g) != 3 {
		t.Fatalf("卒業生 %d 件, want 3", len(g))
	}
	if g[0].GraduateName != "卒業生A" || g[0].HiredAt == nil || g[0].HiredAt.Format("2006-01-02") != "2024-04-01" {
		t.Errorf("卒業生[0] = %+v", g[0])
	}
	if g[1].HiredAt != nil || g[2].HiredAt != nil {
		t.Errorf("入社日が読めない・空のときは nil: %v, %v", g[1].HiredAt, g[2].HiredAt)
	}

	// 重みプロファイルが無い入力では作らない
	repo2 := &entryRepoSpy{}
	if _, err := newEntrySubmitService(repo2).Submit(validEntryInput()); err != nil {
		t.Fatal(err)
	}
	if repo2.draft.WeightProfile != nil || len(repo2.draft.Graduates) != 0 || len(repo2.draft.JobPositions) != 0 {
		t.Errorf("入力に無い行を作っている: %+v", repo2.draft)
	}
}

// 保存に失敗したら、エラーをそのまま返し、メールは送らない(送信済みの記録も付けない)。
func TestSubmit_CreateDraftFailure(t *testing.T) {
	boom := errors.New("db down")
	repo := &entryRepoSpy{createErr: boom}
	_, err := newEntrySubmitService(repo).Submit(validEntryInput())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want db down", err)
	}
	if len(repo.sentIDs) != 0 {
		t.Fatalf("保存に失敗したのに、送信済みを記録している: %v", repo.sentIDs)
	}
}

func TestResendEmail(t *testing.T) {
	t.Run("申請が無ければ ErrCompanyEntrySubmissionNotFound", func(t *testing.T) {
		repo := &entryRepoSpy{findSubErr: repository.ErrNotFound}
		err := newEntrySubmitService(repo).ResendEmail(99)
		if !errors.Is(err, ErrCompanyEntrySubmissionNotFound) {
			t.Fatalf("err = %v, want ErrCompanyEntrySubmissionNotFound", err)
		}
	})
	t.Run("その他の読み込み失敗はそのまま返す", func(t *testing.T) {
		boom := errors.New("db down")
		repo := &entryRepoSpy{findSubErr: boom}
		err := newEntrySubmitService(repo).ResendEmail(1)
		if !errors.Is(err, boom) || errors.Is(err, ErrCompanyEntrySubmissionNotFound) {
			t.Fatalf("err = %v, want db down(見つからない、とは区別する)", err)
		}
	})
	t.Run("企業の取得に失敗したらそのまま返す", func(t *testing.T) {
		boom := errors.New("db down")
		repo := &entryRepoSpy{submission: &models.CompanyEntrySubmission{ID: 1, CompanyID: 3}, findCompanyEr: boom}
		if err := newEntrySubmitService(repo).ResendEmail(1); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want db down", err)
		}
	})
	t.Run("再送できたら、送信済みを記録する", func(t *testing.T) {
		repo := &entryRepoSpy{
			submission: &models.CompanyEntrySubmission{ID: 7, CompanyID: 3, ContactEmail: "hr@example.com"},
			company:    &models.Company{Name: "株式会社テスト"},
		}
		if err := newEntrySubmitService(repo).ResendEmail(7); err != nil {
			t.Fatalf("ResendEmail: %v", err)
		}
		if len(repo.sentIDs) != 1 || repo.sentIDs[0] != 7 {
			t.Fatalf("送信済みの記録 = %v, want [7]", repo.sentIDs)
		}
	})
}
