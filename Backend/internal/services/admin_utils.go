package services

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"os"
	"strings"
)

func promoteAdminIfMatched(user *entity.User, repo repository.UserRepository) {
	if user == nil || repo == nil || user.IsAdmin {
		return
	}
	if !isAdminIdentity(user.Email) {
		return
	}
	user.IsAdmin = true
	_ = repo.UpdateUser(user)
}

func isAdminIdentity(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	adminEmails := splitEnvList("ADMIN_EMAILS")
	for _, v := range adminEmails {
		if v == email {
			return true
		}
	}
	return false
}

func splitEnvList(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		val := strings.ToLower(strings.TrimSpace(p))
		if val == "" {
			continue
		}
		result = append(result, val)
	}
	return result
}
