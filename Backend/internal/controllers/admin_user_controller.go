package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/services"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
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

// List GET /api/admin/users
func (c *AdminUserController) List(ctx echo.Context) error {
	const maxOffset = 10000 // DoS対策: 巨大オフセットによるフルスキャンを防ぐ（#332）
	limit := 25
	offset := 0
	if l, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && o >= 0 {
		if o > maxOffset {
			o = maxOffset
		}
		offset = o
	}
	query := strings.TrimSpace(ctx.QueryParam("q"))

	users, total, err := c.repo.ListUsersPaged(limit, offset, query)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch users")
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
	return ctx.JSON(http.StatusOK, map[string]any{
		"users":  resp,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Update PUT /api/admin/users/:id
func (c *AdminUserController) Update(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	var payload adminUserUpdateRequest
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	user, err := c.repo.GetUserByID(uint(id))
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
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
			return echo.NewHTTPError(http.StatusBadRequest, "target_level must be 新卒 or 中途")
		}
		user.TargetLevel = level
	}
	if payload.SchoolName != nil {
		user.SchoolName = strings.TrimSpace(*payload.SchoolName)
	}
	if err := c.repo.UpdateUser(user); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update user")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "user.update", "user", user.ID, map[string]any{
		"is_admin":     user.IsAdmin,
		"target_level": user.TargetLevel,
		"school_name":  user.SchoolName,
	})
	return ctx.JSON(http.StatusOK, adminUserResponse{
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
