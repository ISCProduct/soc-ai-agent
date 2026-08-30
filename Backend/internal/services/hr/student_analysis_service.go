package hr

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/interfaces"
	"Backend/internal/services/shared"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

const maxInterviewReports = 5

type studentUserReader interface {
	GetUserByID(id uint) (*entity.User, error)
}

type interviewSessionLister interface {
	ListByUser(userID uint, limit int, offset int) ([]models.InterviewSession, error)
	CountByUser(userID uint) (int64, error)
}

type interviewReportFinder interface {
	FindBySessionIDs(sessionIDs []uint) ([]models.InterviewReport, error)
}

// StudentAnalysisService 企業向け学生分析データの集約（#1096）
type StudentAnalysisService struct {
	db                      *gorm.DB
	userRepo                studentUserReader
	crossFeature            interfaces.CrossFeatureIntegrationService
	weightScoreRepo         *repositories.UserWeightScoreRepository
	conversationContextRepo repository.ConversationContextRepository
	interviewSessionRepo    interviewSessionLister
	interviewReportRepo     interviewReportFinder
	resumeRepo              interfaces.ResumeDocumentFinder
}

func NewStudentAnalysisService(
	db *gorm.DB,
	userRepo studentUserReader,
	crossFeature interfaces.CrossFeatureIntegrationService,
	weightScoreRepo *repositories.UserWeightScoreRepository,
	conversationContextRepo repository.ConversationContextRepository,
	interviewSessionRepo interviewSessionLister,
	interviewReportRepo interviewReportFinder,
	resumeRepo interfaces.ResumeDocumentFinder,
) *StudentAnalysisService {
	return &StudentAnalysisService{
		db:                      db,
		userRepo:                userRepo,
		crossFeature:            crossFeature,
		weightScoreRepo:         weightScoreRepo,
		conversationContextRepo: conversationContextRepo,
		interviewSessionRepo:    interviewSessionRepo,
		interviewReportRepo:     interviewReportRepo,
		resumeRepo:              resumeRepo,
	}
}

// GetAnalysis 企業オーナー向けに学生の分析プロファイルを返す
func (s *StudentAnalysisService) GetAnalysis(ownerUserID, companyID, targetUserID uint) (*StudentAnalysisResponse, error) {
	if err := s.requireCompanyOwner(ownerUserID, companyID); err != nil {
		return nil, err
	}
	student, err := s.userRepo.GetUserByID(targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudentNotVisible
		}
		return nil, err
	}
	if student == nil || !student.AllowScoutVisibility {
		return nil, ErrStudentNotVisible
	}

	chatSessionID := s.latestChatSessionID(targetUserID)
	interviewCount := 0
	if count, err := s.interviewSessionRepo.CountByUser(targetUserID); err == nil {
		interviewCount = int(count)
	}

	profile, err := s.crossFeature.BuildIntegratedProfile(
		targetUserID,
		chatSessionID,
		interviewCount,
		s.hasReviewedResume(targetUserID),
	)
	if err != nil {
		return nil, err
	}

	resp := &StudentAnalysisResponse{
		UserID:            targetUserID,
		IntegratedProfile: profile,
		InterviewReports:  []InterviewReportView{},
	}
	if chatSessionID != "" {
		resp.ChatSummary = s.loadChatSummary(chatSessionID)
	}
	resp.InterviewReports = s.loadInterviewReports(targetUserID)
	return resp, nil
}

func (s *StudentAnalysisService) requireCompanyOwner(userID, companyID uint) error {
	if companyID == 0 {
		return shared.ErrForbidden
	}
	owns, err := shared.UserOwnsCompany(s.db, userID, companyID)
	if err != nil {
		return err
	}
	if !owns {
		return shared.ErrForbidden
	}
	return nil
}

func (s *StudentAnalysisService) latestChatSessionID(userID uint) string {
	if s.weightScoreRepo == nil {
		return ""
	}
	scores, err := s.weightScoreRepo.FindLatestByUser(userID)
	if err != nil || len(scores) == 0 {
		return ""
	}
	return scores[0].SessionID
}

func (s *StudentAnalysisService) hasReviewedResume(userID uint) bool {
	if s.resumeRepo == nil {
		return false
	}
	docs, err := s.resumeRepo.FindDocumentsByUserID(userID)
	if err != nil {
		return false
	}
	for _, doc := range docs {
		if doc.Status == "reviewed" {
			return true
		}
	}
	return false
}

func (s *StudentAnalysisService) loadChatSummary(sessionID string) *ChatSummaryView {
	ctx, err := s.conversationContextRepo.GetBySessionID(sessionID)
	if err != nil || ctx == nil || ctx.LlmSummary == "" {
		return nil
	}
	var summary ChatSummaryView
	if err := json.Unmarshal([]byte(ctx.LlmSummary), &summary); err != nil {
		return nil
	}
	if summary.Strengths == "" && summary.Weaknesses == "" && summary.RecommendedWorkingStyle == "" {
		return nil
	}
	return &summary
}

func (s *StudentAnalysisService) loadInterviewReports(userID uint) []InterviewReportView {
	sessions, err := s.interviewSessionRepo.ListByUser(userID, maxInterviewReports, 0)
	if err != nil || len(sessions) == 0 {
		return []InterviewReportView{}
	}
	ids := make([]uint, 0, len(sessions))
	for _, sess := range sessions {
		ids = append(ids, sess.ID)
	}
	reports, err := s.interviewReportRepo.FindBySessionIDs(ids)
	if err != nil || len(reports) == 0 {
		return []InterviewReportView{}
	}
	out := make([]InterviewReportView, 0, len(reports))
	for _, r := range reports {
		out = append(out, InterviewReportView{
			SessionID:        r.SessionID,
			SummaryText:      r.SummaryText,
			StrengthsJSON:    r.StrengthsJSON,
			ImprovementsJSON: r.ImprovementsJSON,
		})
	}
	return out
}
