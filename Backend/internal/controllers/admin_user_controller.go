package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/services/auth"
	"Backend/internal/services/interfaces"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const maxAdminUsersOffset = 10000

type AdminUserController struct {
	repo     repository.UserRepository
	audit    interfaces.AuditLogService
	deletion *auth.UserDeletionService
}

func NewAdminUserController(repo repository.UserRepository, audit interfaces.AuditLogService) *AdminUserController {
	return &AdminUserController{repo: repo, audit: audit}
}

// SetDeletionService は管理者によるユーザー削除で使用するカスケードサービスを設定する
func (c *AdminUserController) SetDeletionService(deletion *auth.UserDeletionService) {
	c.deletion = deletion
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

// Delete DELETE /api/admin/users/:id
// 管理者によるユーザー退会（論理削除）。関連データと S3 は保持期間後に物理削除される。
func (c *AdminUserController) Delete(ctx echo.Context) error {
	if c.deletion == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "user deletion is not configured")
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	user, err := c.repo.GetUserByID(uint(id))
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if user.IsWithdrawn() {
		return echo.NewHTTPError(http.StatusConflict, "account already withdrawn")
	}
	actor := strings.TrimSpace(ctx.Request().Header.Get("X-Admin-Email"))
	if err := c.deletion.DeleteUser(uint(id), auth.UserDeletionActor{Kind: "admin", Email: actor}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		if errors.Is(err, auth.ErrAlreadyWithdrawn) {
			return echo.NewHTTPError(http.StatusConflict, "account already withdrawn")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to withdraw user")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"message":         "ユーザーを退会処理しました（保持期間後に完全削除）",
		"id":              uint(id),
		"email":           user.Email,
		"retention_days":  auth.WithdrawalRetentionDays,
	})
}

// PurgeExpired POST /api/admin/users/purge-expired
// 保持期間を過ぎた退会ユーザーを物理削除する。
func (c *AdminUserController) PurgeExpired(ctx echo.Context) error {
	if c.deletion == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "user deletion is not configured")
	}
	n, err := c.deletion.PurgeExpiredWithdrawals(time.Time{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to purge expired withdrawals")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"purged": n,
	})
}

func timeLayout() string {
	return "2006-01-02 15:04:05"
}
