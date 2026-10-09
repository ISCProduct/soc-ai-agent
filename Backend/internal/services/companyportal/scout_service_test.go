package companyportal

import (
	"errors"
	"sync"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/shared"

	"gorm.io/gorm"
)

type fakeScoutStore struct {
	mu        sync.Mutex
	templates []models.ScoutTemplate
	scouts    []models.Scout
	blocks    map[uint]map[uint]bool
	nextID    uint
}

func newFakeScoutStore() *fakeScoutStore {
	return &fakeScoutStore{
		blocks: map[uint]map[uint]bool{},
		nextID: 1,
	}
}

func (f *fakeScoutStore) CreateTemplate(t *models.ScoutTemplate) error {
	t.ID = f.nextID
	f.nextID++
	f.templates = append(f.templates, *t)
	return nil
}

func (f *fakeScoutStore) UpdateTemplate(companyID, id uint, title, body string) error {
	for i := range f.templates {
		if f.templates[i].ID == id && f.templates[i].CompanyID == companyID {
			f.templates[i].Title = title
			f.templates[i].Body = body
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeScoutStore) DeleteTemplate(companyID, id uint) error {
	for i := range f.templates {
		if f.templates[i].ID == id && f.templates[i].CompanyID == companyID {
			f.templates = append(f.templates[:i], f.templates[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeScoutStore) ListTemplates(companyID uint) ([]models.ScoutTemplate, error) {
	out := []models.ScoutTemplate{}
	for _, t := range f.templates {
		if t.CompanyID == companyID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeScoutStore) FindTemplate(companyID, id uint) (*models.ScoutTemplate, error) {
	for i := range f.templates {
		if f.templates[i].ID == id && f.templates[i].CompanyID == companyID {
			return &f.templates[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeScoutStore) CreateScoutWithinCooldown(s *models.Scout, cooldown time.Duration, now time.Time) (time.Duration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	last, err := f.latestBetween(s.CompanyID, s.UserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	if err == nil {
		elapsed := now.Sub(last.CreatedAt)
		if elapsed < cooldown {
			return cooldown - elapsed, nil
		}
	}
	s.ID = f.nextID
	f.nextID++
	s.CreatedAt = now
	f.scouts = append(f.scouts, *s)
	return 0, nil
}

func (f *fakeScoutStore) ListByCompany(companyID uint, _, _ int) ([]models.Scout, int64, error) {
	out := []models.Scout{}
	for _, s := range f.scouts {
		if s.CompanyID == companyID {
			out = append(out, s)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeScoutStore) ListByUser(userID uint, _, _ int) ([]models.Scout, int64, error) {
	out := []models.Scout{}
	for _, s := range f.scouts {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeScoutStore) FindByIDForUser(userID, id uint) (*models.Scout, error) {
	for i := range f.scouts {
		if f.scouts[i].ID == id && f.scouts[i].UserID == userID {
			return &f.scouts[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeScoutStore) UpdateStatusForUser(userID, id uint, fromStatuses []string, toStatus string) error {
	for i := range f.scouts {
		if f.scouts[i].ID != id || f.scouts[i].UserID != userID {
			continue
		}
		for _, st := range fromStatuses {
			if f.scouts[i].Status == st {
				f.scouts[i].Status = toStatus
				return nil
			}
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeScoutStore) LatestBetween(companyID, userID uint) (*models.Scout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.latestBetween(companyID, userID)
}

func (f *fakeScoutStore) latestBetween(companyID, userID uint) (*models.Scout, error) {
	var latest *models.Scout
	for i := range f.scouts {
		s := &f.scouts[i]
		if s.CompanyID == companyID && s.UserID == userID {
			if latest == nil || s.CreatedAt.After(latest.CreatedAt) || (s.CreatedAt.Equal(latest.CreatedAt) && s.ID > latest.ID) {
				cp := *s
				latest = &cp
			}
		}
	}
	if latest == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return latest, nil
}

func (f *fakeScoutStore) IsBlocked(userID, companyID uint) (bool, error) {
	return f.blocks[userID][companyID], nil
}

func (f *fakeScoutStore) BlockCompany(userID, companyID uint) error {
	if f.blocks[userID] == nil {
		f.blocks[userID] = map[uint]bool{}
	}
	f.blocks[userID][companyID] = true
	return nil
}

func (f *fakeScoutStore) ListBlockedCompanyIDs(userID uint) ([]uint, error) {
	out := []uint{}
	for id := range f.blocks[userID] {
		out = append(out, id)
	}
	return out, nil
}

type fakeVisible struct{ ok bool }

func (f fakeVisible) IsVisible(uint) (bool, error) { return f.ok, nil }

func (f fakeVisible) VisibleStudentNames(uint, []uint) (map[uint]string, error) {
	return map[uint]string{}, nil
}

type fakeUserReader struct{ user *entity.User }

func (f fakeUserReader) GetUserByID(uint) (*entity.User, error) { return f.user, nil }

type fakeCompanyReader struct{ company *models.Company }

func (f fakeCompanyReader) FindByID(uint) (*models.Company, error) { return f.company, nil }

type fakeMailer struct {
	mu   sync.Mutex
	sent int
}

func (f *fakeMailer) SendScoutOfferEmail(_, _, _, _, _ string) error {
	f.mu.Lock()
	f.sent++
	f.mu.Unlock()
	return nil
}

func TestScoutService_Send_クールダウンとブロック(t *testing.T) {
	store := newFakeScoutStore()
	_ = store.CreateTemplate(&models.ScoutTemplate{
		CompanyID: 1, Title: "案内", Body: "{{学生名}} さん / {{企業名}}", CreatedBy: 9,
	})
	mailer := &fakeMailer{}
	svc := NewScoutService(
		store,
		fakeVisible{ok: true},
		fakeUserReader{user: &entity.User{ID: 101, Name: "山田", Email: "a@example.com"}},
		fakeCompanyReader{company: &models.Company{ID: 1, Name: "デモ株式会社"}},
		mailer,
		"http://localhost:3000",
	)
	now := time.UnixMilli(1_700_000_000_000)
	svc.now = func() time.Time { return now }

	first, err := svc.Send(1, 9, SendScoutInput{UserID: 101, TemplateID: 1})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Scout.Message != "山田 さん / デモ株式会社" {
		t.Fatalf("差し込み失敗: %q", first.Scout.Message)
	}
	if mailer.sent != 1 {
		t.Fatalf("mail sent=%d", mailer.sent)
	}

	_, err = svc.Send(1, 9, SendScoutInput{UserID: 101, TemplateID: 1})
	if !errors.Is(err, ErrScoutCooldown) {
		t.Fatalf("cooldown want %v got %v", ErrScoutCooldown, err)
	}

	_ = store.BlockCompany(101, 1)
	svc.now = func() time.Time { return now.Add(ScoutCooldown + time.Minute) }
	_, err = svc.Send(1, 9, SendScoutInput{UserID: 101, TemplateID: 1})
	if !errors.Is(err, ErrScoutBlocked) {
		t.Fatalf("blocked want %v got %v", ErrScoutBlocked, err)
	}
}

func TestScoutService_Send_並行送信は1件だけ保存してメールも1通(t *testing.T) {
	store := newFakeScoutStore()
	_ = store.CreateTemplate(&models.ScoutTemplate{
		CompanyID: 1, Title: "案内", Body: "{{学生名}} さん", CreatedBy: 9,
	})
	mailer := &fakeMailer{}
	svc := NewScoutService(
		store,
		fakeVisible{ok: true},
		fakeUserReader{user: &entity.User{ID: 101, Name: "山田", Email: "a@example.com"}},
		fakeCompanyReader{company: &models.Company{ID: 1, Name: "デモ株式会社"}},
		mailer,
		"http://localhost:3000",
	)
	now := time.UnixMilli(1_700_000_000_000)
	svc.now = func() time.Time { return now }

	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount := 0
	cooldownCount := 0
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Send(1, 9, SendScoutInput{UserID: 101, TemplateID: 1})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				okCount++
			case errors.Is(err, ErrScoutCooldown):
				cooldownCount++
			default:
				t.Errorf("並行送信: %v", err)
			}
		}()
	}
	wg.Wait()

	if okCount != 1 || cooldownCount != 1 {
		t.Fatalf("成功=%d クールダウン=%d、同時送信は1件だけ通る", okCount, cooldownCount)
	}
	mailer.mu.Lock()
	sent := mailer.sent
	mailer.mu.Unlock()
	if sent != 1 {
		t.Fatalf("メールは1通であるべき、実際は %d", sent)
	}
	if len(store.scouts) != 1 {
		t.Fatalf("保存件数=%d", len(store.scouts))
	}
}

func TestScoutService_CreateTemplate_バリデーション(t *testing.T) {
	svc := NewScoutService(newFakeScoutStore(), fakeVisible{ok: true}, nil, nil, nil, "")
	_, err := svc.CreateTemplate(1, 2, ScoutTemplateInput{Title: "", Body: "x"})
	var ve *shared.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("validation expected, got %v", err)
	}
}

func TestInterpolateScoutBody(t *testing.T) {
	got := InterpolateScoutBody("{{学生名}}@{{企業名}}", "A", "B")
	if got != "A@B" {
		t.Fatalf("got %q", got)
	}
}
