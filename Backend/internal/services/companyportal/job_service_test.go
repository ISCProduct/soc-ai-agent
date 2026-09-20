package companyportal

// 企業ポータルの求人管理のテスト（#1321）。
// 実行: cd Backend && go test ./internal/services/companyportal/ -v
//
// 見たいのは「他社の求人に触れないこと」と
// 「企業本体が未公開なら求人を公開できないこと」。
// 後者を黙って通すと、公開したつもりで学生に見えていない状態になる。

import (
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

type fakeJobRepo struct {
	company *models.Company
	jobs    map[uint]*models.CompanyJobPosition
	listed  []models.CompanyJobPosition
	created *models.CompanyJobPosition
	updated *models.CompanyJobPosition
	nextID  uint
	findErr error
}

func newFakeJobRepo() *fakeJobRepo {
	return &fakeJobRepo{
		company: &models.Company{ID: 5, DataStatus: "published", IsActive: true},
		jobs:    map[uint]*models.CompanyJobPosition{},
		nextID:  100,
	}
}

func (r *fakeJobRepo) FindByID(id uint) (*models.Company, error) {
	if r.company != nil && r.company.ID == id {
		return r.company, nil
	}
	return nil, errors.New("not found")
}

func (r *fakeJobRepo) FindJobPositionByID(id uint) (*models.CompanyJobPosition, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	j, ok := r.jobs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return j, nil
}

func (r *fakeJobRepo) ListJobPositions(companyID, _ *uint, _ int) ([]models.CompanyJobPosition, error) {
	if companyID == nil {
		return r.listed, nil
	}
	out := []models.CompanyJobPosition{}
	for _, j := range r.jobs {
		if j.CompanyID == *companyID {
			out = append(out, *j)
		}
	}
	return out, nil
}

func (r *fakeJobRepo) CreateJobPosition(p *models.CompanyJobPosition) error {
	p.ID = r.nextID
	r.nextID++
	r.jobs[p.ID] = p
	r.created = p
	return nil
}

func (r *fakeJobRepo) UpdateJobPosition(p *models.CompanyJobPosition) error {
	r.jobs[p.ID] = p
	r.updated = p
	return nil
}

func validInput() JobInput {
	return JobInput{Title: "バックエンドエンジニア", MinSalary: 400, MaxSalary: 600}
}

func TestCreate_下書きとして作る(t *testing.T) {
	repo := newFakeJobRepo()
	s := NewJobService(repo)

	job, err := s.Create(5, validInput())
	if err != nil {
		t.Fatalf("作成に失敗: %v", err)
	}
	// 作成した瞬間に公開すると、書きかけが学生に見える。
	if job.DataStatus != JobStatusDraft {
		t.Errorf("下書きで作られていない: %s", job.DataStatus)
	}
	if job.CompanyID != 5 {
		t.Errorf("company_id が違う: %d", job.CompanyID)
	}
}

func TestCreate_companyIDが0なら403(t *testing.T) {
	s := NewJobService(newFakeJobRepo())
	_, err := s.Create(0, validInput())
	if !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("403 を返すべき: %v", err)
	}
}

func TestValidate_入力の検査(t *testing.T) {
	tests := []struct {
		name    string
		in      JobInput
		wantErr bool
	}{
		// 必須は title だけ。入力負担を上げると求人が0件のままになる。
		{"タイトルのみで通る", JobInput{Title: "エンジニア"}, false},
		{"タイトルが空", JobInput{Title: "   "}, true},
		{"年収が負", JobInput{Title: "A", MinSalary: -1}, true},
		{"最低が最高を上回る", JobInput{Title: "A", MinSalary: 700, MaxSalary: 500}, true},
		{"片方だけなら通る", JobInput{Title: "A", MinSalary: 500}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdate_他社の求人は403(t *testing.T) {
	repo := newFakeJobRepo()
	repo.jobs[1] = &models.CompanyJobPosition{ID: 1, CompanyID: 99, Title: "他社の求人"}
	s := NewJobService(repo)

	_, err := s.Update(1, 5, validInput())
	if !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("403 を返すべき: %v", err)
	}
	if repo.updated != nil {
		t.Error("他社の求人が更新された")
	}
}

func TestUpdate_存在しないIDも403(t *testing.T) {
	// 404 と分けると、そのIDが存在するかを総当たりで調べられる。
	s := NewJobService(newFakeJobRepo())
	_, err := s.Update(12345, 5, validInput())
	if !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("403 を返すべき: %v", err)
	}
}

func TestSetPublished_企業が未公開でも公開自体は通す(t *testing.T) {
	// 現状 842社すべてが draft / provisional(#1077 / #1078)。
	// ここで拒否すると求人機能がまったく使えなくなる。
	// 公開は通し、学生に見えない可能性を呼び出し元へ伝える。
	repo := newFakeJobRepo()
	repo.company = &models.Company{ID: 5, DataStatus: "draft", IsActive: true}
	repo.jobs[1] = &models.CompanyJobPosition{ID: 1, CompanyID: 5, DataStatus: JobStatusDraft}
	s := NewJobService(repo)

	got, err := s.SetPublished(1, 5, true)
	if err != nil {
		t.Fatalf("公開は通すべき: %v", err)
	}
	if got.Job.DataStatus != JobStatusPublished {
		t.Errorf("求人が公開になっていない: %s", got.Job.DataStatus)
	}
	// 「公開したつもりで見えていない」を UI で黙らせないための情報。
	if got.CompanyPublished {
		t.Error("企業が未公開なのに公開済みとして返している")
	}
}

func TestSetPublished_企業が非アクティブなら見えない扱い(t *testing.T) {
	repo := newFakeJobRepo()
	repo.company = &models.Company{ID: 5, DataStatus: "published", IsActive: false}
	repo.jobs[1] = &models.CompanyJobPosition{ID: 1, CompanyID: 5, DataStatus: JobStatusDraft}
	s := NewJobService(repo)

	got, err := s.SetPublished(1, 5, true)
	if err != nil {
		t.Fatalf("公開は通すべき: %v", err)
	}
	if got.CompanyPublished {
		t.Error("is_active=false なのに公開済みとして返している")
	}
}

func TestSetPublished_企業が公開済みなら公開できる(t *testing.T) {
	repo := newFakeJobRepo()
	repo.jobs[1] = &models.CompanyJobPosition{ID: 1, CompanyID: 5, DataStatus: JobStatusDraft}
	s := NewJobService(repo)

	got, err := s.SetPublished(1, 5, true)
	if err != nil {
		t.Fatalf("公開に失敗: %v", err)
	}
	// 学生側の GetJobPositionsByCompany は is_active かつ published で絞る。
	// 両方揃っていないと公開したのに見えない。
	if got.Job.DataStatus != JobStatusPublished || !got.Job.IsActive {
		t.Errorf("学生に見える状態になっていない: status=%s active=%v", got.Job.DataStatus, got.Job.IsActive)
	}
	if !got.CompanyPublished {
		t.Error("企業が公開済みなのに見えない扱いになっている")
	}
}

func TestSetPublished_非公開化できる(t *testing.T) {
	// 削除は提供しない。応募が紐づくため非公開化で対応する。
	repo := newFakeJobRepo()
	repo.jobs[1] = &models.CompanyJobPosition{ID: 1, CompanyID: 5, DataStatus: JobStatusPublished, IsActive: true}
	s := NewJobService(repo)

	got, err := s.SetPublished(1, 5, false)
	if err != nil {
		t.Fatalf("非公開化に失敗: %v", err)
	}
	if got.Job.DataStatus != JobStatusDraft {
		t.Errorf("非公開になっていない: %s", got.Job.DataStatus)
	}
}

func TestSetPublished_他社の求人は403(t *testing.T) {
	repo := newFakeJobRepo()
	repo.jobs[1] = &models.CompanyJobPosition{ID: 1, CompanyID: 99, DataStatus: JobStatusDraft}
	s := NewJobService(repo)

	_, err := s.SetPublished(1, 5, true)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("403 を返すべき: %v", err)
	}
	if repo.jobs[1].DataStatus != JobStatusDraft {
		t.Error("他社の求人の状態が変わった")
	}
}

func TestList_自社の下書きも返す(t *testing.T) {
	// 下書きが見えないと編集できない。学生向けの取得とは条件が違う。
	repo := newFakeJobRepo()
	repo.jobs[1] = &models.CompanyJobPosition{ID: 1, CompanyID: 5, DataStatus: JobStatusDraft}
	repo.jobs[2] = &models.CompanyJobPosition{ID: 2, CompanyID: 5, DataStatus: JobStatusPublished}
	repo.jobs[3] = &models.CompanyJobPosition{ID: 3, CompanyID: 99, DataStatus: JobStatusPublished}
	s := NewJobService(repo)

	got, err := s.List(5)
	if err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("自社の求人だけを返すべき: %d件", len(got))
	}
	for _, j := range got {
		if j.CompanyID != 5 {
			t.Errorf("他社の求人が混ざっている: company=%d", j.CompanyID)
		}
	}
}
