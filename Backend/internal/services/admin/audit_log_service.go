package admin

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"encoding/json"
	"log"
)

type AuditLogService struct {
	repo repository.AuditLogRepository
}

func NewAuditLogService(repo repository.AuditLogRepository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

func (s *AuditLogService) Record(actorEmail, action, targetType string, targetID uint, metadata map[string]any) {
	if s == nil || s.repo == nil {
		return
	}
	meta := ""
	if metadata != nil {
		raw, err := json.Marshal(metadata)
		if err != nil {
			// 操作自体は残す。metadata が空なのは失敗としてログに出す（#1991）。
			log.Printf("[AuditLog] metadata marshal failed action=%s target_type=%s target_id=%d: %v", action, targetType, targetID, err)
		} else {
			meta = string(raw)
		}
	}
	if err := s.repo.Create(&models.AuditLog{
		ActorEmail: actorEmail,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Metadata:   meta,
	}); err != nil {
		log.Printf("[AuditLog] write failed action=%s target_type=%s target_id=%d: %v", action, targetType, targetID, err)
	}
}

func (s *AuditLogService) List(limit int) ([]models.AuditLog, error) {
	return s.repo.List(limit)
}
