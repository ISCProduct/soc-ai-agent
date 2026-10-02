package companyportal

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/shared"

	"gorm.io/gorm"
)

// 同一企業→同一学生への再送クールダウン（#1095）。
const ScoutCooldown = 24 * time.Hour

var (
	ErrScoutCooldown   = errors.New("scout cooldown active")
	ErrScoutBlocked    = errors.New("scout blocked by student")
	ErrScoutNotVisible = errors.New("student not visible for scout")
	ErrScoutNotFound   = shared.ErrNotFound
)

type scoutStore interface {
	CreateTemplate(t *models.ScoutTemplate) error
	UpdateTemplate(companyID, id uint, title, body string) error
	DeleteTemplate(companyID, id uint) error
	ListTemplates(companyID uint) ([]models.ScoutTemplate, error)
	FindTemplate(companyID, id uint) (*models.ScoutTemplate, error)
	CreateScout(s *models.Scout) error
	ListByCompany(companyID uint, limit, offset int) ([]models.Scout, int64, error)
	ListByUser(userID uint, limit, offset int) ([]models.Scout, int64, error)
	FindByIDForUser(userID, id uint) (*models.Scout, error)
	UpdateStatusForUser(userID, id uint, fromStatuses []string, toStatus string) error
	LatestBetween(companyID, userID uint) (*models.Scout, error)
	IsBlocked(userID, companyID uint) (bool, error)
	BlockCompany(userID, companyID uint) error
	ListBlockedCompanyIDs(userID uint) ([]uint, error)
}

type scoutVisibility interface {
	IsVisible(userID uint) (bool, error)
	VisibleStudentNames(companyID uint, userIDs []uint) (map[uint]string, error)
}

type scoutUserReader interface {
	GetUserByID(id uint) (*entity.User, error)
}

type scoutCompanyReader interface {
	FindByID(id uint) (*models.Company, error)
}

type scoutMailer interface {
	SendScoutOfferEmail(toEmail, studentName, companyName, message, appURL string) error
}

// ScoutService は企業ポータルのスカウト送信と学生側の受信操作（#1095）。
type ScoutService struct {
	store     scoutStore
	visible   scoutVisibility
	users     scoutUserReader
	companies scoutCompanyReader
	mailer    scoutMailer
	now       func() time.Time
	appURL    string
}

func NewScoutService(
	store scoutStore,
	visible scoutVisibility,
	users scoutUserReader,
	companies scoutCompanyReader,
	mailer scoutMailer,
	appURL string,
) *ScoutService {
	return &ScoutService{
		store:     store,
		visible:   visible,
		users:     users,
		companies: companies,
		mailer:    mailer,
		now:       time.Now,
		appURL:    strings.TrimRight(strings.TrimSpace(appURL), "/"),
	}
}

type ScoutTemplateInput struct {
	Title string
	Body  string
}

func (in ScoutTemplateInput) Validate() error {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" {
		return &shared.ValidationError{Message: "タイトルは必須です"}
	}
	if body == "" {
		return &shared.ValidationError{Message: "本文は必須です"}
	}
	if utf8.RuneCountInString(title) > models.MaxScoutTemplateTitleLength {
		return &shared.ValidationError{Message: "タイトルが長すぎます"}
	}
	if utf8.RuneCountInString(body) > models.MaxScoutTemplateBodyLength {
		return &shared.ValidationError{Message: "本文が長すぎます"}
	}
	return nil
}

func InterpolateScoutBody(body, studentName, companyName string) string {
	r := strings.NewReplacer(
		"{{学生名}}", studentName,
		"{{企業名}}", companyName,
		"{{name}}", studentName,
		"{{company}}", companyName,
	)
	return r.Replace(body)
}

func (s *ScoutService) ListTemplates(companyID uint) ([]models.ScoutTemplate, error) {
	if companyID == 0 {
		return nil, shared.ErrForbidden
	}
	return s.store.ListTemplates(companyID)
}

func (s *ScoutService) CreateTemplate(companyID, companyUserID uint, in ScoutTemplateInput) (*models.ScoutTemplate, error) {
	if companyID == 0 || companyUserID == 0 {
		return nil, shared.ErrForbidden
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	t := &models.ScoutTemplate{
		CompanyID: companyID,
		Title:     strings.TrimSpace(in.Title),
		Body:      strings.TrimSpace(in.Body),
		CreatedBy: companyUserID,
	}
	if err := s.store.CreateTemplate(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ScoutService) UpdateTemplate(companyID, id uint, in ScoutTemplateInput) (*models.ScoutTemplate, error) {
	if companyID == 0 {
		return nil, shared.ErrForbidden
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if err := s.store.UpdateTemplate(companyID, id, strings.TrimSpace(in.Title), strings.TrimSpace(in.Body)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScoutNotFound
		}
		return nil, err
	}
	return s.store.FindTemplate(companyID, id)
}

func (s *ScoutService) DeleteTemplate(companyID, id uint) error {
	if companyID == 0 {
		return shared.ErrForbidden
	}
	if err := s.store.DeleteTemplate(companyID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrScoutNotFound
		}
		return err
	}
	return nil
}

type SendScoutInput struct {
	UserID     uint
	TemplateID uint
	// Message を渡すとテンプレート差し込み結果を上書きできる（プレビュー確定文）。
	Message string
}

type SendScoutResult struct {
	Scout       *models.Scout
	RemainingMs int64
}

func (s *ScoutService) CooldownRemaining(companyID, userID uint) (time.Duration, error) {
	last, err := s.store.LatestBetween(companyID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	elapsed := s.now().Sub(last.CreatedAt)
	if elapsed >= ScoutCooldown {
		return 0, nil
	}
	return ScoutCooldown - elapsed, nil
}

func (s *ScoutService) Send(companyID, companyUserID uint, in SendScoutInput) (*SendScoutResult, error) {
	if companyID == 0 || companyUserID == 0 {
		return nil, shared.ErrForbidden
	}
	if in.UserID == 0 {
		return nil, &shared.ValidationError{Message: "送信先の学生を指定してください"}
	}
	if in.TemplateID == 0 && strings.TrimSpace(in.Message) == "" {
		return nil, &shared.ValidationError{Message: "テンプレートか文面を指定してください"}
	}

	visible, err := s.visible.IsVisible(in.UserID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrScoutNotVisible
	}

	blocked, err := s.store.IsBlocked(in.UserID, companyID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrScoutBlocked
	}

	remaining, err := s.CooldownRemaining(companyID, in.UserID)
	if err != nil {
		return nil, err
	}
	if remaining > 0 {
		return &SendScoutResult{RemainingMs: remaining.Milliseconds()}, ErrScoutCooldown
	}

	company, err := s.companies.FindByID(companyID)
	if err != nil || company == nil {
		return nil, fmt.Errorf("company lookup: %w", err)
	}
	student, err := s.users.GetUserByID(in.UserID)
	if err != nil || student == nil {
		return nil, ErrScoutNotVisible
	}

	var templateID *uint
	message := strings.TrimSpace(in.Message)
	if in.TemplateID != 0 {
		tpl, err := s.store.FindTemplate(companyID, in.TemplateID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, &shared.ValidationError{Message: "テンプレートが見つかりません"}
			}
			return nil, err
		}
		id := tpl.ID
		templateID = &id
		if message == "" {
			message = InterpolateScoutBody(tpl.Body, student.Name, company.Name)
		}
	}
	if message == "" {
		return nil, &shared.ValidationError{Message: "文面が空です"}
	}
	if utf8.RuneCountInString(message) > models.MaxScoutMessageLength {
		return nil, &shared.ValidationError{Message: "文面が長すぎます"}
	}

	scout := &models.Scout{
		CompanyID:  companyID,
		UserID:     in.UserID,
		TemplateID: templateID,
		SentBy:     companyUserID,
		Message:    message,
		Status:     models.ScoutStatusSent,
	}
	if err := s.store.CreateScout(scout); err != nil {
		return nil, err
	}

	if s.mailer != nil && student.Email != "" && !student.IsGuest {
		appURL := s.appURL
		if appURL == "" {
			appURL = "http://localhost:3000"
		}
		if err := s.mailer.SendScoutOfferEmail(student.Email, student.Name, company.Name, message, appURL); err != nil {
			log.Printf("[ScoutService] メール送信失敗 scout_id=%d: %v", scout.ID, err)
		}
	}

	return &SendScoutResult{Scout: scout}, nil
}

type CompanyScoutView struct {
	models.Scout
	StudentName string `json:"student_name"`
}

func (s *ScoutService) ListCompanyScouts(companyID uint, limit, offset int) ([]CompanyScoutView, int64, error) {
	if companyID == 0 {
		return nil, 0, shared.ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	rows, total, err := s.store.ListByCompany(companyID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].UserID)
	}
	names := map[uint]string{}
	if s.visible != nil && len(ids) > 0 {
		names, err = s.visible.VisibleStudentNames(companyID, ids)
		if err != nil {
			return nil, 0, err
		}
	}
	out := make([]CompanyScoutView, 0, len(rows))
	for i := range rows {
		out = append(out, CompanyScoutView{
			Scout:       rows[i],
			StudentName: names[rows[i].UserID],
		})
	}
	return out, total, nil
}

type StudentScoutView struct {
	models.Scout
	CompanyName string `json:"company_name"`
}

func (s *ScoutService) ListStudentScouts(userID uint, limit, offset int) ([]StudentScoutView, int64, error) {
	if userID == 0 {
		return nil, 0, shared.ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	rows, total, err := s.store.ListByUser(userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]StudentScoutView, 0, len(rows))
	for i := range rows {
		name := ""
		if c, err := s.companies.FindByID(rows[i].CompanyID); err == nil && c != nil {
			name = c.Name
		}
		out = append(out, StudentScoutView{Scout: rows[i], CompanyName: name})
	}
	return out, total, nil
}

func (s *ScoutService) MarkViewed(userID, scoutID uint) (*models.Scout, error) {
	scout, err := s.store.FindByIDForUser(userID, scoutID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScoutNotFound
		}
		return nil, err
	}
	if scout.Status == models.ScoutStatusSent {
		if err := s.store.UpdateStatusForUser(userID, scoutID, []string{models.ScoutStatusSent}, models.ScoutStatusViewed); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		} else {
			scout.Status = models.ScoutStatusViewed
		}
	}
	return scout, nil
}

func (s *ScoutService) Decline(userID, scoutID uint) (*models.Scout, error) {
	if err := s.store.UpdateStatusForUser(
		userID, scoutID,
		[]string{models.ScoutStatusSent, models.ScoutStatusViewed, models.ScoutStatusAccepted},
		models.ScoutStatusDeclined,
	); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScoutNotFound
		}
		return nil, err
	}
	return s.store.FindByIDForUser(userID, scoutID)
}

func (s *ScoutService) BlockCompany(userID, companyID uint) error {
	if userID == 0 || companyID == 0 {
		return &shared.ValidationError{Message: "企業が指定されていません"}
	}
	return s.store.BlockCompany(userID, companyID)
}

func (s *ScoutService) ListBlockedCompanyIDs(userID uint) ([]uint, error) {
	return s.store.ListBlockedCompanyIDs(userID)
}
