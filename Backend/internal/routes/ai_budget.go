package routes

import (
	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/services/costs"
	"Backend/internal/usagectx"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type aiBudgetStatusProvider interface {
	Status(context.Context, uint) (costs.AIBudgetStatus, error)
	Acquire(context.Context, uint) (func() error, error)
}

// EchoAIBudget はAIを実行する利用者向けAPIの開始時に予算を確認する。
// 面接の発話・終了経路は対象外にし、開始後の体験を予算超過で中断しない。
func EchoAIBudget(service aiBudgetStatusProvider, userSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !requiresAIBudget(c.Request().Method, c.Path()) {
				return next(c)
			}
			if service == nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "AI利用予算の確認機能が設定されていません")
			}
			userID, ok := httpapi.UserID(c)
			if !ok {
				token := strings.TrimSpace(c.Request().Header.Get("X-User-Token"))
				if token != "" && userSecret != "" {
					parsedID, _, err := middleware.ParseJWT(token, userSecret)
					if err != nil {
						return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
					}
					userID = parsedID
					ctx := usagectx.WithActor(c.Request().Context(), userID, 0)
					c.SetRequest(c.Request().WithContext(ctx))
				} else {
					// ESのゲスト利用は従来どおりレート制限で保護し、利用者予算は適用しない。
					return next(c)
				}
			}
			if requiresDeferredAIBudget(c.Request().Method, c.Path()) {
				ctx := usagectx.WithAIBudgetCheck(c.Request().Context(), func(ctx context.Context) (context.Context, error) {
					release, err := service.Acquire(ctx, userID)
					if err != nil {
						return ctx, fmt.Errorf("%w: %v", usagectx.ErrAIBudgetUnavailable, err)
					}
					status, err := service.Status(ctx, userID)
					if err != nil {
						_ = release()
						return ctx, fmt.Errorf("%w: %v", usagectx.ErrAIBudgetUnavailable, err)
					}
					if !status.Allowed {
						_ = release()
						return ctx, usagectx.ErrAIBudgetExceeded
					}
					ctx = usagectx.WithAIBudgetLock(ctx, release)
					ctx = usagectx.WithAIBudgetChecked(ctx)
					if status.School != nil {
						ctx = usagectx.WithSchoolID(ctx, status.School.ScopeID)
					}
					return ctx, nil
				})
				c.SetRequest(c.Request().WithContext(ctx))
				return next(c)
			}
			release, err := service.Acquire(c.Request().Context(), userID)
			if err != nil {
				httpapi.LogError(err)
				return echo.NewHTTPError(http.StatusServiceUnavailable, "AI利用予算を確認できません。時間をおいて再試行してください。")
			}
			defer func() {
				if err := release(); err != nil {
					httpapi.LogError(err)
				}
			}()
			status, err := service.Status(c.Request().Context(), userID)
			if err != nil {
				httpapi.LogError(err)
				return echo.NewHTTPError(http.StatusServiceUnavailable, "AI利用予算を確認できません。時間をおいて再試行してください。")
			}
			if !status.Allowed {
				return budgetExceededResponse(c, status)
			}
			ctx := c.Request().Context()
			ctx = usagectx.WithAIBudgetChecked(ctx)
			if status.School != nil {
				ctx = usagectx.WithSchoolID(ctx, status.School.ScopeID)
			}
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func budgetExceededResponse(c echo.Context, status costs.AIBudgetStatus) error {
	return c.JSON(http.StatusTooManyRequests, map[string]any{
		"error":  "AI利用予算の上限に達しました。翌日または翌月に再度お試しください。",
		"code":   "ai_budget_exceeded",
		"budget": status,
	})
}

func requiresDeferredAIBudget(method, routePath string) bool {
	if method == http.MethodGet && routePath == "/api/companies/web-search" {
		return true
	}
	if method != http.MethodPost {
		return false
	}
	switch routePath {
	case "/api/companies/validate", "/api/github/repo/summarize":
		return true
	default:
		return false
	}
}

func requiresAIBudget(method, routePath string) bool {
	if requiresDeferredAIBudget(method, routePath) {
		return true
	}
	if method == http.MethodGet && (routePath == "/api/chat/analysis" || routePath == "/api/interviews/:id/phrase-suggestions") {
		return true
	}
	if method != http.MethodPost {
		return false
	}
	switch routePath {
	case "/api/chat", "/api/questions/generate",
		"/api/resume/review", "/api/resume/review/stream",
		"/api/es/rewrite", "/api/es/review",
		"/api/realtime/token",
		"/api/interviews/:id/start",
		"/api/interviews/:id/report/regenerate", "/api/interviews/:id/send-report",
		"/api/chat/send-report":
		return true
	default:
		return false
	}
}
