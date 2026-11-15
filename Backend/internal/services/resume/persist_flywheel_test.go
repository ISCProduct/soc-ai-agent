package resume

// 履歴書レビュー → 診断セッション解決 → 再マッチの配線テスト。
// 実行: cd Backend && go test ./internal/services/resume/ -run PersistReview_Flywheel -v

import (
	"context"
	"sync"
	"testing"
	"time"

	"Backend/internal/models"
)

type stubResumeCrossFeature struct {
	session        string
	resolveErr     error
	updatedSession string
	updatedUser    uint
	contextText    string
}

func (s *stubResumeCrossFeature) ResolveDiagnosisSessionID(uint) (string, error) {
	return s.session, s.resolveErr
}

func (s *stubResumeCrossFeature) UpdateScoresFromResumeReview(
	userID uint, chatSessionID string, _ *models.ResumeReview, _ []models.ResumeReviewItem,
) error {
	s.updatedUser = userID
	s.updatedSession = chatSessionID
	return nil
}

func (s *stubResumeCrossFeature) BuildResumeContextFromUser(uint) string {
	return s.contextText
}

type stubMatchingRunner struct {
	mu      sync.Mutex
	calls   int
	userID  uint
	session string
	done    chan struct{}
}

func (m *stubMatchingRunner) CalculateMatching(_ context.Context, userID uint, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.userID = userID
	m.session = sessionID
	if m.done != nil {
		select {
		case <-m.done:
		default:
			close(m.done)
		}
	}
	return nil
}

func TestPersistReview_Flywheel_診断セッションへ書いて再マッチする(t *testing.T) {
	repo := &fakeResumeRepo{}
	cf := &stubResumeCrossFeature{session: "chat-sess-9"}
	matcher := &stubMatchingRunner{done: make(chan struct{})}
	s := &ResumeService{repo: repo, crossFeature: cf, matchingRunner: matcher}

	score := 80
	doc := &models.ResumeDocument{ID: 7, UserID: 8, OrganizationID: 3, SessionID: "stale-fe-session"}
	err := s.persistReview(doc, &models.ResumeReview{Score: &score}, nil)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if cf.updatedSession != "chat-sess-9" {
		t.Fatalf("書き込み先 session=%q want chat-sess-9 (FE の stale を使ってはいけない)", cf.updatedSession)
	}
	if cf.updatedUser != 8 {
		t.Fatalf("user=%d", cf.updatedUser)
	}

	select {
	case <-matcher.done:
	case <-time.After(2 * time.Second):
		t.Fatal("再マッチが呼ばれなかった")
	}
	matcher.mu.Lock()
	defer matcher.mu.Unlock()
	if matcher.calls != 1 || matcher.session != "chat-sess-9" || matcher.userID != 8 {
		t.Fatalf("rematch calls=%d user=%d session=%q", matcher.calls, matcher.userID, matcher.session)
	}
}

func TestPersistReview_Flywheel_面接スナップショットでは再マッチしない(t *testing.T) {
	repo := &fakeResumeRepo{}
	cf := &stubResumeCrossFeature{session: "interview-8"}
	matcher := &stubMatchingRunner{done: make(chan struct{})}
	s := &ResumeService{repo: repo, crossFeature: cf, matchingRunner: matcher}

	score := 70
	err := s.persistReview(
		&models.ResumeDocument{ID: 1, UserID: 8},
		&models.ResumeReview{Score: &score},
		nil,
	)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if cf.updatedSession != "interview-8" {
		t.Fatalf("session=%q", cf.updatedSession)
	}
	time.Sleep(50 * time.Millisecond)
	matcher.mu.Lock()
	defer matcher.mu.Unlock()
	if matcher.calls != 0 {
		t.Fatalf("スナップショットへの再マッチは禁止: calls=%d", matcher.calls)
	}
}

func TestPersistReview_Flywheel_スコア無しでは再マッチしない(t *testing.T) {
	repo := &fakeResumeRepo{}
	cf := &stubResumeCrossFeature{session: "chat-1"}
	matcher := &stubMatchingRunner{done: make(chan struct{})}
	s := &ResumeService{repo: repo, crossFeature: cf, matchingRunner: matcher}

	err := s.persistReview(
		&models.ResumeDocument{ID: 1, UserID: 8},
		&models.ResumeReview{Score: nil},
		nil,
	)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	matcher.mu.Lock()
	defer matcher.mu.Unlock()
	if matcher.calls != 0 {
		t.Fatalf("スコア無しなのに rematch: %d", matcher.calls)
	}
}

func TestCandidateContextForRAG(t *testing.T) {
	s := &ResumeService{crossFeature: &stubResumeCrossFeature{contextText: "【候補者の強み傾向】\n"}}
	got := s.candidateContextForRAG(1)
	if got == "" {
		t.Fatal("コンテキストが空")
	}
	if s.candidateContextForRAG(0) != "" {
		t.Fatal("userID=0 では空であるべき")
	}
}
