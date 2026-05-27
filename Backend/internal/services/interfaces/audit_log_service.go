package interfaces

import "Backend/internal/models"

// AuditLogService 監査ログサービスのインターフェース
type AuditLogService interface {
	Record(actorEmail, action, targetType string, targetID uint, metadata map[string]interface{})
	List(limit int) ([]models.AuditLog, error)
}
