package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CompanyBriefReader は面接・レビュー用の共有企業キャッシュ参照の最小面。
type CompanyBriefReader interface {
	FindByID(id uint) (*models.Company, error)
	FindByName(name string) (*models.Company, error)
	GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error)
}

type InterviewService struct {
	sessionRepo          repository.InterviewSessionRepository
	utterRepo            repository.InterviewUtteranceRepository
	reportRepo           repository.InterviewReportRepository
	userRepo             repository.UserRepository
	emailService         *EmailService
	openaiClient         *openai.Client
	realtimeUsageService *RealtimeUsageService
	crossFeature         *CrossFeatureIntegrationService
	companyQuestionRepo  repository.InterviewCompanyQuestionRepository
	questionStateRepo    repository.InterviewQuestionStateRepository
	skillScoreRepo       SkillScoreReader
	companyRepo          CompanyBriefReader
	jobCh                chan uint
	workerOnce           sync.Once
}

// SkillScoreReader はGitHubスキルスコア取得の最小インターフェース。
type SkillScoreReader interface {
	GetScores(userID uint) ([]models.SkillScore, error)
}

func NewInterviewService(
	sessionRepo repository.InterviewSessionRepository,
	utterRepo repository.InterviewUtteranceRepository,
	reportRepo repository.InterviewReportRepository,
	userRepo repository.UserRepository,
	emailService *EmailService,
	openaiClient *openai.Client,
	realtimeUsageService *RealtimeUsageService,
) *InterviewService {
	return &InterviewService{
		sessionRepo:          sessionRepo,
		utterRepo:            utterRepo,
		reportRepo:           reportRepo,
		userRepo:             userRepo,
		emailService:         emailService,
		openaiClient:         openaiClient,
		realtimeUsageService: realtimeUsageService,
		jobCh:                make(chan uint, 100),
	}
}

// SetCompanyQuestionRepo 企業別質問リポジトリを注入する（オプション）
func (s *InterviewService) SetCompanyQuestionRepo(r repository.InterviewCompanyQuestionRepository) {
	s.companyQuestionRepo = r
}

// SetQuestionStateRepo 面接質問キューリポジトリを注入する（オプション）
func (s *InterviewService) SetQuestionStateRepo(r repository.InterviewQuestionStateRepository) {
	s.questionStateRepo = r
}

// SetSkillScoreRepo GitHubスキルスコアリポジトリを注入する（オプション）
func (s *InterviewService) SetSkillScoreRepo(r SkillScoreReader) {
	s.skillScoreRepo = r
}

// SetCompanyRepo 企業共有キャッシュ参照用リポジトリを注入する（オプション）
func (s *InterviewService) SetCompanyRepo(r CompanyBriefReader) {
	s.companyRepo = r
}

// SetCrossFeatureService 機能間連携サービスを注入する（オプション）
func (s *InterviewService) SetCrossFeatureService(cf *CrossFeatureIntegrationService) {
	s.crossFeature = cf
}

type InterviewSessionResponse struct {
	ID                uint       `json:"id"`
	UserID            uint       `json:"user_id"`
	Status            string     `json:"status"`
	Language          string     `json:"language"`
	InterviewerGender string     `json:"interviewer_gender"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	EstimatedCostUSD  float64    `json:"estimated_cost_usd"`
	TemplateVersion   string     `json:"template_version"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type InterviewDetailResponse struct {
	Session    InterviewSessionResponse    `json:"session"`
	Utterances []models.InterviewUtterance `json:"utterances"`
	Report     *models.InterviewReport     `json:"report,omitempty"`
}

// PhraseSuggestion は面接回答中の弱い表現と言い換え候補を表す。
type PhraseSuggestion struct {
	Original    string   `json:"original"`
	Suggestions []string `json:"suggestions"`
}

// InterviewTrendPoint は面接トレンド分析の1データポイント。
type InterviewTrendPoint struct {
	SessionID     uint      `json:"session_id"`
	CreatedAt     time.Time `json:"created_at"`
	Logic         *float64  `json:"logic"`
	Specificity   *float64  `json:"specificity"`
	Ownership     *float64  `json:"ownership"`
	Communication *float64  `json:"communication"`
	Enthusiasm    *float64  `json:"enthusiasm"`
}

// TurnResult は1ターンの結果（AIテキスト + TTS音声バイト列）
type TurnResult struct {
	UserText               string
	AIText                 string
	Audio                  []byte
	QuestionSource         string
	QuestionCategory       string
	IsDeepening            bool
	ResolvedCompanyID      uint
	CustomQuestionsEnabled bool
}

func (s *InterviewService) estimateCost(start, end *time.Time) float64 {
	if start == nil || end == nil {
		return 0
	}
	minutes := end.Sub(*start).Minutes()
	if minutes < 0 {
		return 0
	}
	rate := getFloatEnv("INTERVIEW_COST_PER_MIN_USD", 0.18)
	return minutes * rate
}

func (s *InterviewService) isAllowed(actorID uint, ownerID uint) bool {
	if actorID == ownerID {
		return true
	}
	user, err := s.userRepo.GetUserByID(actorID)
	if err != nil || user == nil {
		return false
	}
	return user.IsAdmin
}

func toSessionResponse(session *models.InterviewSession) *InterviewSessionResponse {
	lang := session.Language
	if lang == "" {
		lang = "ja"
	}
	gender := session.InterviewerGender
	if gender == "" {
		gender = "female"
	}
	return &InterviewSessionResponse{
		ID:                session.ID,
		UserID:            session.UserID,
		Status:            session.Status,
		Language:          lang,
		InterviewerGender: gender,
		StartedAt:         session.StartedAt,
		EndedAt:           session.EndedAt,
		EstimatedCostUSD:  session.EstimatedCostUSD,
		TemplateVersion:   session.TemplateVersion,
		CreatedAt:         session.CreatedAt,
		UpdatedAt:         session.UpdatedAt,
	}
}

func toSessionResponses(sessions []models.InterviewSession) []InterviewSessionResponse {
	out := make([]InterviewSessionResponse, 0, len(sessions))
	for i := range sessions {
		out = append(out, *toSessionResponse(&sessions[i]))
	}
	return out
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func getIntEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getFloatEnv(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}
