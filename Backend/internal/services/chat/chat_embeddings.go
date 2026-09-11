package chat

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	maxEmbeddingChars   = 6000
	maxUserMessageCount = 60
)

func (s *ChatService) ensureEmbeddings(ctx context.Context, userID uint, sessionID string, jobCategoryID uint) error {
	if s.aiClient == nil {
		return nil
	}
	var errs []string

	if err := s.ensureUserEmbedding(ctx, userID, sessionID); err != nil {
		errs = append(errs, err.Error())
	}
	if jobCategoryID != 0 {
		if err := s.ensureJobCategoryEmbedding(ctx, jobCategoryID); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (s *ChatService) ensureUserEmbedding(ctx context.Context, userID uint, sessionID string) error {
	if s.userEmbeddingRepo == nil || s.chatMessageRepo == nil {
		return nil
	}
	if userID == 0 || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if _, err := s.userEmbeddingRepo.FindByUserAndSession(userID, sessionID); err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	history, err := s.chatMessageRepo.FindBySessionID(sessionID)
	if err != nil {
		return fmt.Errorf("load chat history: %w", err)
	}

	var user *entity.User
	if s.userRepo != nil {
		user, err = s.userRepo.GetUserByID(userID)
		if err != nil {
			return fmt.Errorf("load user: %w", err)
		}
	}

	text := buildUserEmbeddingText(user, history)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	ctxReq, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	vector, err := s.aiClient.Embedding(ctxReq, text)
	if err != nil {
		return fmt.Errorf("create user embedding: %w", err)
	}

	embeddingJSON, err := marshalEmbedding(vector)
	if err != nil {
		return fmt.Errorf("encode user embedding: %w", err)
	}

	return s.userEmbeddingRepo.Upsert(userID, sessionID, text, embeddingJSON)
}

func (s *ChatService) ensureJobCategoryEmbedding(ctx context.Context, jobCategoryID uint) error {
	if s.jobEmbeddingRepo == nil || s.jobCategoryRepo == nil {
		return nil
	}
	if jobCategoryID == 0 {
		return nil
	}
	if _, err := s.jobEmbeddingRepo.FindByJobCategoryID(jobCategoryID); err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	category, err := s.jobCategoryRepo.FindByID(jobCategoryID)
	if err != nil {
		return fmt.Errorf("load job category: %w", err)
	}
	if category == nil {
		return nil
	}

	text := buildJobCategoryEmbeddingText(category)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	ctxReq, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	vector, err := s.aiClient.Embedding(ctxReq, text)
	if err != nil {
		return fmt.Errorf("create job category embedding: %w", err)
	}
	embeddingJSON, err := marshalEmbedding(vector)
	if err != nil {
		return fmt.Errorf("encode job category embedding: %w", err)
	}

	return s.jobEmbeddingRepo.Upsert(jobCategoryID, text, embeddingJSON)
}

func buildUserEmbeddingText(user *entity.User, history []models.ChatMessage) string {
	var b strings.Builder
	hasContent := false
	b.WriteString("User profile\n")
	if user != nil {
		if strings.TrimSpace(user.Name) != "" {
			b.WriteString("Name: ")
			b.WriteString(strings.TrimSpace(user.Name))
			b.WriteString("\n")
			hasContent = true
		}
		if strings.TrimSpace(user.TargetLevel) != "" {
			b.WriteString("Target level: ")
			b.WriteString(strings.TrimSpace(user.TargetLevel))
			b.WriteString("\n")
			hasContent = true
		}
		if strings.TrimSpace(user.SchoolName) != "" {
			b.WriteString("School: ")
			b.WriteString(strings.TrimSpace(user.SchoolName))
			b.WriteString("\n")
			hasContent = true
		}
		if strings.TrimSpace(user.CertificationsAcquired) != "" {
			b.WriteString("Certifications: ")
			b.WriteString(strings.TrimSpace(user.CertificationsAcquired))
			b.WriteString("\n")
			hasContent = true
		}
		if strings.TrimSpace(user.CertificationsInProgress) != "" {
			b.WriteString("In progress: ")
			b.WriteString(strings.TrimSpace(user.CertificationsInProgress))
			b.WriteString("\n")
			hasContent = true
		}
	}

	userMessages := make([]string, 0)
	for _, msg := range history {
		if msg.Role != "user" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		userMessages = append(userMessages, content)
	}
	if len(userMessages) == 0 {
		if !hasContent {
			return ""
		}
		return trimToMaxChars(strings.TrimSpace(b.String()), maxEmbeddingChars)
	}
	if len(userMessages) > maxUserMessageCount {
		userMessages = userMessages[len(userMessages)-maxUserMessageCount:]
	}

	b.WriteString("Chat responses\n")
	for _, content := range userMessages {
		b.WriteString("- ")
		b.WriteString(content)
		b.WriteString("\n")
	}

	return trimToMaxChars(strings.TrimSpace(b.String()), maxEmbeddingChars)
}

func buildJobCategoryEmbeddingText(category *models.JobCategory) string {
	if category == nil {
		return ""
	}
	var b strings.Builder
	hasContent := false
	b.WriteString("Job category\n")
	if strings.TrimSpace(category.Name) != "" {
		b.WriteString("Name: ")
		b.WriteString(strings.TrimSpace(category.Name))
		b.WriteString("\n")
		hasContent = true
	}
	if strings.TrimSpace(category.NameEn) != "" {
		b.WriteString("English name: ")
		b.WriteString(strings.TrimSpace(category.NameEn))
		b.WriteString("\n")
		hasContent = true
	}
	if strings.TrimSpace(category.Description) != "" {
		b.WriteString("Description: ")
		b.WriteString(strings.TrimSpace(category.Description))
		b.WriteString("\n")
		hasContent = true
	}
	if strings.TrimSpace(category.Path) != "" {
		b.WriteString("Path: ")
		b.WriteString(strings.TrimSpace(category.Path))
		b.WriteString("\n")
		hasContent = true
	}
	if strings.TrimSpace(category.Code) != "" {
		b.WriteString("Code: ")
		b.WriteString(strings.TrimSpace(category.Code))
		b.WriteString("\n")
		hasContent = true
	}
	if !hasContent {
		return ""
	}
	return trimToMaxChars(strings.TrimSpace(b.String()), maxEmbeddingChars)
}

func trimToMaxChars(text string, max int) string {
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}

func marshalEmbedding(vector []float32) (string, error) {
	raw, err := json.Marshal(vector)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
