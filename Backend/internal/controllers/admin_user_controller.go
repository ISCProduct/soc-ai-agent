package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/services"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

const maxAdminUsersOffset = 10000

type AdminUserController struct {
	repo  repository.UserRepository
	audit *services.AuditLogService
}

func NewAdminUserController(repo repository.UserRepository, audit *services.AuditLogService) *AdminUserController {
	return &AdminUserController{repo: repo, audit: audit}
}

type adminUserResponse struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	IsGuest     bool   `json:"is_guest"`
	IsAdmin     bool   `json:"is_admin"`
	TargetLevel string `json:"target_level"`
	SchoolName  string `json:"school_name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type adminUserUpdateRequest struct {
	IsAdmin     *bool   `json:"is_admin"`
	Name        *string `json:"name"`
	TargetLevel *string `json:"target_level"`
	SchoolName  *string `json:"school_name"`
}

func (c *AdminUserController) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 25
	offset := 0
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = min(o, maxAdminUsersOffset)
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	users, total, err := c.repo.ListUsersPaged(limit, offset, query)
	if err != nil {
		http.Error(w, "failed to fetch users", http.StatusInternalServerError)
		return
	}
	resp := make([]adminUserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, adminUserResponse{
			ID:          u.ID,
			Email:       u.Email,
			Name:        u.Name,
			IsGuest:     u.IsGuest,
			IsAdmin:     u.IsAdmin,
			TargetLevel: u.TargetLevel,
			SchoolName:  u.SchoolName,
			CreatedAt:   u.CreatedAt.Format(timeLayout()),
			UpdatedAt:   u.UpdatedAt.Format(timeLayout()),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users":  resp,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (c *AdminUserController) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	path = strings.Trim(path, "/")
	id, err := strconv.ParseUint(path, 10, 32)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	var payload adminUserUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	user, err := c.repo.GetUserByID(uint(id))
	if err != nil || user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if payload.IsAdmin != nil {
		user.IsAdmin = *payload.IsAdmin
	}
	if payload.Name != nil {
		user.Name = strings.TrimSpace(*payload.Name)
	}
	if payload.TargetLevel != nil {
		level := strings.TrimSpace(*payload.TargetLevel)
		if level != "" && level != "新卒" && level != "中途" {
			http.Error(w, "target_level must be 新卒 or 中途", http.StatusBadRequest)
			return
		}
		user.TargetLevel = level
	}
	if payload.SchoolName != nil {
		user.SchoolName = strings.TrimSpace(*payload.SchoolName)
	}
	if err := c.repo.UpdateUser(user); err != nil {
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "user.update", "user", user.ID, map[string]interface{}{
		"is_admin":     user.IsAdmin,
		"target_level": user.TargetLevel,
		"school_name":  user.SchoolName,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(adminUserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		IsGuest:     user.IsGuest,
		IsAdmin:     user.IsAdmin,
		TargetLevel: user.TargetLevel,
		SchoolName:  user.SchoolName,
		CreatedAt:   user.CreatedAt.Format(timeLayout()),
		UpdatedAt:   user.UpdatedAt.Format(timeLayout()),
	})
}

func timeLayout() string {
	return "2006-01-02 15:04:05"
}
