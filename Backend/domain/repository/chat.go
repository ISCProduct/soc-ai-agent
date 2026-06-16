package repository

import "Backend/internal/models"

// ChatMessageRepository はチャットメッセージの永続化インターフェース。
type ChatMessageRepository interface {
	Create(msg *models.ChatMessage) error
	FindBySessionID(sessionID string) ([]models.ChatMessage, error)
	FindByUserID(userID uint) ([]models.ChatMessage, error)
	FindRecentBySessionID(sessionID string, limit int) ([]models.ChatMessage, error)
	GetUsedQuestionIDs(sessionID string) ([]uint, error)
	GetUserSessions(userID uint) ([]models.ChatSession, error)
}

// AIGeneratedQuestionRepository はAI生成質問の永続化インターフェース。
type AIGeneratedQuestionRepository interface {
	Create(q *models.AIGeneratedQuestion) error
	FindBySessionID(sessionID string) ([]models.AIGeneratedQuestion, error)
	FindByUserAndSession(userID uint, sessionID string) ([]models.AIGeneratedQuestion, error)
	GetAskedQuestionIDs(userID uint, sessionID string) ([]uint, error)
	UpdateAnswer(id uint, answerText string, answerScore int) error
	FindUnansweredBySession(sessionID string) (*models.AIGeneratedQuestion, error)
}

// PredefinedQuestionRepository は事前定義質問の永続化インターフェース。
type PredefinedQuestionRepository interface {
	FindByCategory(category string, targetLevel string) ([]*models.PredefinedQuestion, error)
	FindActiveQuestions(targetLevel string, industryID *uint, jobCategoryID *uint, currentPhase string) ([]*models.PredefinedQuestion, error)
	FindByID(id uint) (*models.PredefinedQuestion, error)
	Create(question *models.PredefinedQuestion) error
	Update(question *models.PredefinedQuestion) error
	GetNextQuestion(askedQuestionIDs []uint, targetLevel string, industryID *uint, jobCategoryID *uint, prioritizeCategory string, currentPhase string) (*models.PredefinedQuestion, error)
	CountByCategory(category string) (int64, error)
}

// QuestionWeightRepository は質問重み設定の永続化インターフェース。
type QuestionWeightRepository interface {
	CheckDuplicate(question string, weightCategory string) (bool, error)
	Create(qw *models.QuestionWeight) error
	FindByID(id uint) (*models.QuestionWeight, error)
	FindActiveByCategory(category string) ([]models.QuestionWeight, error)
	FindActiveByIndustryAndJob(industryID, jobCategoryID uint) ([]models.QuestionWeight, error)
	GetRandomQuestion(industryID, jobCategoryID uint) (*models.QuestionWeight, error)
	GetRandomQuestionExcluding(industryID, jobCategoryID uint, excludeIDs []uint) (*models.QuestionWeight, error)
	GetRandomQuestionByCategory(category string, excludeIDs []uint) (*models.QuestionWeight, error)
}

// ConversationContextRepository は会話コンテキストの永続化インターフェース。
type ConversationContextRepository interface {
	GetBySessionID(sessionID string) (*models.ConversationContext, error)
	GetOrCreate(userID uint, sessionID string) (*models.ConversationContext, error)
	SetJobCategoryID(userID uint, sessionID string, jobCategoryID uint) error
	GetJobCategoryID(sessionID string) (uint, error)
	// SetSessionSummary はセッション単位で LLM による診断サマリを保存します。
	SetSessionSummary(userID uint, sessionID string, summary string) error
}

// SessionValidationRepository はセッション検証情報の永続化インターフェース。
type SessionValidationRepository interface {
	GetOrCreate(sessionID string) (*models.SessionValidation, error)
	IncrementInvalidCount(sessionID string) (*models.SessionValidation, error)
	ResetInvalidCount(sessionID string) error
	TerminateSession(sessionID string) error
	IsTerminated(sessionID string) (bool, error)
}
