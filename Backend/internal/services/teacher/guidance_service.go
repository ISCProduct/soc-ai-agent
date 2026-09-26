package teacher

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"Backend/domain/entity"
	"Backend/internal/models"
)

const (
	maxGuidanceMessageRunes = 1000
	maxSuggestedIndustries  = 5
)

var (
	ErrGuidanceStudentNotFound = errors.New("student not found")
	ErrGuidanceInvalidKind     = errors.New("invalid guidance kind")
	ErrGuidanceEmptyMessage    = errors.New("message is required")
	ErrGuidanceMessageTooLong  = errors.New("message too long")
)

// GuidanceStore は案内の永続化面。
type GuidanceStore interface {
	Create(g *models.TeacherStudentGuidance) error
	ListActiveByStudent(studentUserID uint, limit int) ([]models.TeacherStudentGuidance, error)
	FindByIDForStudent(id, studentUserID uint) (*models.TeacherStudentGuidance, error)
	Dismiss(id, studentUserID uint) error
}

// StudentByID は生徒1件の取得面（学校スコープ検証用）。
type StudentByID interface {
	GetUserByID(id uint) (*entity.User, error)
}

type GuidanceService struct {
	store    GuidanceStore
	students StudentByID
}

func NewGuidanceService(store GuidanceStore, students StudentByID) *GuidanceService {
	return &GuidanceService{store: store, students: students}
}

type CreateGuidanceInput struct {
	TeacherUserID       uint
	StudentUserID       uint
	Kind                string
	Message             string
	SuggestedIndustries []string
}

type GuidanceView struct {
	ID                  uint     `json:"id"`
	Kind                string   `json:"kind"`
	Message             string   `json:"message"`
	SuggestedIndustries []string `json:"suggested_industries,omitempty"`
	CreatedAt           string   `json:"created_at"`
}

// ResolveStudentSchool は教員の学校スコープ検証用に生徒の school_id を返す。
func (s *GuidanceService) ResolveStudentSchool(studentUserID uint) (*uint, error) {
	u, err := s.students.GetUserByID(studentUserID)
	if err != nil || u == nil {
		return nil, ErrGuidanceStudentNotFound
	}
	return u.SchoolID, nil
}

func (s *GuidanceService) Create(in CreateGuidanceInput) (*GuidanceView, error) {
	kind := strings.TrimSpace(in.Kind)
	switch kind {
	case models.GuidanceKindLowMatch, models.GuidanceKindResume, models.GuidanceKindInactive:
	default:
		return nil, ErrGuidanceInvalidKind
	}
	msg := strings.TrimSpace(in.Message)
	if msg == "" {
		msg = defaultGuidanceMessage(kind, in.SuggestedIndustries)
	}
	if msg == "" {
		return nil, ErrGuidanceEmptyMessage
	}
	if utf8.RuneCountInString(msg) > maxGuidanceMessageRunes {
		return nil, ErrGuidanceMessageTooLong
	}

	industries := trimIndustries(in.SuggestedIndustries)
	var industriesJSON string
	if len(industries) > 0 {
		b, err := json.Marshal(industries)
		if err != nil {
			return nil, err
		}
		industriesJSON = string(b)
	}

	g := &models.TeacherStudentGuidance{
		StudentUserID:       in.StudentUserID,
		TeacherUserID:       in.TeacherUserID,
		Kind:                kind,
		Message:             msg,
		SuggestedIndustries: industriesJSON,
	}
	if err := s.store.Create(g); err != nil {
		return nil, err
	}
	return toGuidanceView(g), nil
}

func (s *GuidanceService) ListActiveForStudent(studentUserID uint) ([]GuidanceView, error) {
	rows, err := s.store.ListActiveByStudent(studentUserID, 10)
	if err != nil {
		return nil, err
	}
	out := make([]GuidanceView, 0, len(rows))
	for i := range rows {
		out = append(out, *toGuidanceView(&rows[i]))
	}
	return out, nil
}

func (s *GuidanceService) Dismiss(studentUserID, guidanceID uint) error {
	g, err := s.store.FindByIDForStudent(guidanceID, studentUserID)
	if err != nil || g == nil {
		return ErrGuidanceStudentNotFound
	}
	return s.store.Dismiss(guidanceID, studentUserID)
}

func defaultGuidanceMessage(kind string, industries []string) string {
	switch kind {
	case models.GuidanceKindLowMatch:
		base := "マッチ度の低い企業への応募が見られます。向いている業界を確認し、志望の見直しを検討しましょう。"
		if names := trimIndustries(industries); len(names) > 0 {
			return base + " 参考業界: " + strings.Join(names, "、")
		}
		return base
	case models.GuidanceKindResume:
		return "履歴書の完成度が低い、または未提出です。早めに提出・レビューを受けて改善しましょう。"
	case models.GuidanceKindInactive:
		return "就活の進捗が止まっているようです。キャリア面談や診断・面接練習から再開してみましょう。"
	default:
		return ""
	}
}

func trimIndustries(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, raw := range in {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
		if len(out) >= maxSuggestedIndustries {
			break
		}
	}
	return out
}

func toGuidanceView(g *models.TeacherStudentGuidance) *GuidanceView {
	v := &GuidanceView{
		ID:        g.ID,
		Kind:      g.Kind,
		Message:   g.Message,
		CreatedAt: g.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if g.SuggestedIndustries != "" {
		var names []string
		if err := json.Unmarshal([]byte(g.SuggestedIndustries), &names); err == nil {
			v.SuggestedIndustries = names
		}
	}
	return v
}

// FormatCreateError はコントローラ向けの短いエラー文言。
func FormatCreateError(err error) string {
	switch {
	case errors.Is(err, ErrGuidanceInvalidKind):
		return "案内の種類が不正です"
	case errors.Is(err, ErrGuidanceEmptyMessage):
		return "メッセージを入力してください"
	case errors.Is(err, ErrGuidanceMessageTooLong):
		return fmt.Sprintf("メッセージは%d文字以内にしてください", maxGuidanceMessageRunes)
	case errors.Is(err, ErrGuidanceStudentNotFound):
		return "生徒が見つかりません"
	default:
		return "案内の作成に失敗しました"
	}
}
