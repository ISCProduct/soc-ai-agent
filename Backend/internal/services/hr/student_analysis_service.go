package hr

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/interfaces"
	"Backend/internal/services/shared"
	"encoding/json"
	"errors"
)

const maxInterviewReports = 5

// ScoutVisibleRole は企業へ公開する対象のロール。
const ScoutVisibleRole = "student"

// IsScoutVisible は企業へ公開してよい学生かを判定する。
// repositories.StudentSearchRepository.visibleStudents のSQL条件と同一の意味を持ち、
// 一覧・セマンティック検索・タグ付与・詳細のすべてが同じ条件を通るようにする。
// 片方だけ条件を変えると認可が不一致になるため、変更時は必ず両方を揃えること。
func IsScoutVisible(u *models.User) bool {
	return u != nil &&
		u.AllowScoutVisibility &&
		!u.IsWithdrawn() &&
		u.Role == ScoutVisibleRole &&
		!u.IsGuest
}

type studentUserReader interface {
	GetUserByID(id uint) (*models.User, error)
}

type interviewSessionLister interface {
	ListByUser(userID uint, limit int, offset int) ([]models.InterviewSession, error)
	CountByUser(userID uint) (int64, error)
}

type interviewReportFinder interface {
	FindBySessionIDs(sessionIDs []uint) ([]models.InterviewReport, error)
}

// identityRevealer は、その企業が学生のスカウトを承諾済みかを返す。
// 未設定のときは氏名を出さない（旧HR APIが承諾チェックを飛ばせないようにする）。
type identityRevealer interface {
	IdentityRevealed(companyID, userID uint) (bool, error)
}

// StudentAnalysisService 企業向け学生分析データの集約（#1096）
type StudentAnalysisService struct {
	access                  shared.CompanyAccessReader
	userRepo                studentUserReader
	crossFeature            interfaces.CrossFeatureIntegrationService
	weightScoreRepo         *repositories.UserWeightScoreRepository
	conversationContextRepo repository.ConversationContextRepository
	interviewSessionRepo    interviewSessionLister
	interviewReportRepo     interviewReportFinder
	resumeRepo              interfaces.ResumeDocumentFinder
	identity                identityRevealer
}

func NewStudentAnalysisService(
	access shared.CompanyAccessReader,
	userRepo studentUserReader,
	crossFeature interfaces.CrossFeatureIntegrationService,
	weightScoreRepo *repositories.UserWeightScoreRepository,
	conversationContextRepo repository.ConversationContextRepository,
	interviewSessionRepo interviewSessionLister,
	interviewReportRepo interviewReportFinder,
	resumeRepo interfaces.ResumeDocumentFinder,
) *StudentAnalysisService {
	return &StudentAnalysisService{
		access:                  access,
		userRepo:                userRepo,
		crossFeature:            crossFeature,
		weightScoreRepo:         weightScoreRepo,
		conversationContextRepo: conversationContextRepo,
		interviewSessionRepo:    interviewSessionRepo,
		interviewReportRepo:     interviewReportRepo,
		resumeRepo:              resumeRepo,
	}
}

// SetIdentityRevealer はスカウト承諾の有無で氏名を出す判定を入れる。
func (s *StudentAnalysisService) SetIdentityRevealer(r identityRevealer) {
	s.identity = r
}

// GetAnalysis 企業オーナー向けに学生の分析プロファイルを返す（company_ownerships 経由）
func (s *StudentAnalysisService) GetAnalysis(ownerUserID, companyID, targetUserID uint) (*StudentAnalysisResponse, error) {
	if err := s.requireCompanyOwner(ownerUserID, companyID); err != nil {
		return nil, err
	}
	resp, err := s.buildAnalysis(targetUserID)
	if err != nil {
		return nil, err
	}
	if err := s.redactNameUnlessRevealed(resp, companyID, targetUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetAnalysisForVisibleStudent は認可済みの呼び出し元向けに分析プロファイルを返す（#1094 企業ポータル）。
// 企業ポータルJWTは company_id を直接持つため company_ownerships の確認は不要だが、
// 学生の公開同意チェック（buildAnalysis 内）と、承諾するまでの氏名非公開は共通で必ず通る。
func (s *StudentAnalysisService) GetAnalysisForVisibleStudent(companyID, targetUserID uint) (*StudentAnalysisResponse, error) {
	resp, err := s.buildAnalysis(targetUserID)
	if err != nil {
		return nil, err
	}
	if err := s.redactNameUnlessRevealed(resp, companyID, targetUserID); err != nil {
		return nil, err
	}
	return resp, nil
}

// redactNameUnlessRevealed は、自社スカウトを承諾していない学生の氏名を空にする。
// 判定を持たない組み立ては氏名を出さない。旧HR APIがこのチェックを飛ばせないようにする。
func (s *StudentAnalysisService) redactNameUnlessRevealed(resp *StudentAnalysisResponse, companyID, userID uint) error {
	if resp == nil {
		return nil
	}
	if s.identity == nil || companyID == 0 {
		resp.Name = ""
		return nil
	}
	revealed, err := s.identity.IdentityRevealed(companyID, userID)
	if err != nil {
		return err
	}
	if !revealed {
		resp.Name = ""
	}
	return nil
}

// buildAnalysis は認可後の共通組み立て処理。公開同意していない学生は ErrStudentNotVisible。
func (s *StudentAnalysisService) buildAnalysis(targetUserID uint) (*StudentAnalysisResponse, error) {
	student, err := s.userRepo.GetUserByID(targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrStudentNotVisible
		}
		return nil, err
	}
	if !IsScoutVisible(student) {
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
		Name:              student.Name,
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
	owns, err := shared.UserOwnsCompany(s.access, userID, companyID)
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
