package services

import (
	"Backend/internal/models"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *InterviewService) CreateSession(userID uint, language string, interviewerGender string) (*InterviewSessionResponse, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if language == "" {
		language = "ja"
	}
	if interviewerGender != "male" && interviewerGender != "female" {
		interviewerGender = "female"
	}
	session := &models.InterviewSession{
		UserID:            userID,
		Status:            "ready",
		Language:          language,
		InterviewerGender: interviewerGender,
		TemplateVersion:   getEnv("INTERVIEW_TEMPLATE_VERSION", "v1"),
	}
	if err := s.sessionRepo.Create(session); err != nil {
		return nil, err
	}
	return toSessionResponse(session), nil
}

func (s *InterviewService) StartSession(userID uint, sessionID uint) (*InterviewSessionResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	if session.Status == "finished" {
		return nil, errors.New("session already finished")
	}
	if session.StartedAt == nil {
		now := time.Now()
		session.StartedAt = &now
	}
	session.Status = "in_progress"
	if err := s.sessionRepo.Update(session); err != nil {
		return nil, err
	}
	return toSessionResponse(session), nil
}

func (s *InterviewService) FinishSession(userID uint, sessionID uint) (*InterviewSessionResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	if session.EndedAt == nil {
		now := time.Now()
		session.EndedAt = &now
	}
	session.Status = "finished"
	if s.realtimeUsageService != nil {
		if durationSec, cost, err := s.realtimeUsageService.CloseSession(sessionID, *session.EndedAt, nil); err == nil && durationSec >= 0 {
			session.EstimatedCostUSD = cost
		} else {
			session.EstimatedCostUSD = s.estimateCost(session.StartedAt, session.EndedAt)
		}
	} else {
		session.EstimatedCostUSD = s.estimateCost(session.StartedAt, session.EndedAt)
	}
	if err := s.sessionRepo.Update(session); err != nil {
		return nil, err
	}
	s.jobCh <- sessionID
	return toSessionResponse(session), nil
}

func (s *InterviewService) SaveUtterance(userID uint, sessionID uint, role string, text string) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	if !s.isAllowed(userID, session.UserID) {
		return errors.New("forbidden")
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "user" && role != "ai" {
		return errors.New("invalid role")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty text")
	}
	utter := &models.InterviewUtterance{
		SessionID: sessionID,
		Role:      role,
		Text:      text,
	}
	return s.utterRepo.Create(utter)
}

// ListAllSessionsAdmin lists all interview sessions without performing a user-level admin check.
// The caller (admin middleware) is responsible for ensuring only admins can invoke this.
func (s *InterviewService) ListAllSessionsAdmin(limit int, offset int, schoolID *uint) ([]InterviewSessionResponse, int64, error) {
	total, err := s.sessionRepo.CountAll(schoolID)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := s.sessionRepo.ListAll(limit, offset, schoolID)
	if err != nil {
		return nil, 0, err
	}
	return toSessionResponses(sessions), total, nil
}

func (s *InterviewService) ListSessions(userID uint, all bool, limit int, offset int) ([]InterviewSessionResponse, int64, error) {
	if all {
		user, err := s.userRepo.GetUserByID(userID)
		if err != nil || user == nil || !user.IsAdmin {
			return nil, 0, errors.New("forbidden")
		}
		total, err := s.sessionRepo.CountAll(nil)
		if err != nil {
			return nil, 0, err
		}
		sessions, err := s.sessionRepo.ListAll(limit, offset, nil)
		if err != nil {
			return nil, 0, err
		}
		return toSessionResponses(sessions), total, nil
	}
	total, err := s.sessionRepo.CountByUser(userID)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := s.sessionRepo.ListByUser(userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return toSessionResponses(sessions), total, nil
}

func (s *InterviewService) GetSessionDetail(userID uint, sessionID uint) (*InterviewDetailResponse, error) {
	return s.GetSessionDetailWithRole(userID, sessionID, "student")
}

func (s *InterviewService) GetSessionDetailWithRole(userID uint, sessionID uint, role string) (*InterviewDetailResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	utterances, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	var report *models.InterviewReport
	report, err = s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		report = nil
	}
	// 教員以外には教員用レポートを返さない
	if report != nil && role != "teacher" {
		sanitized := *report
		sanitized.TeacherReportJSON = ""
		report = &sanitized
	}
	return &InterviewDetailResponse{
		Session:    *toSessionResponse(session),
		Utterances: utterances,
		Report:     report,
	}, nil
}
