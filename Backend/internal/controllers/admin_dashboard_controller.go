package controllers

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"fmt"
	"net/http"
	"strings"
	"time"

	"encoding/json"

	"github.com/labstack/echo/v4"
)

// AdminDashboardController provides admin endpoints for the user score dashboard.
type AdminDashboardController struct {
	userRepo    *repositories.UserRepository
	sessionRepo *repositories.InterviewSessionRepository
	reportRepo  *repositories.InterviewReportRepository
}

func NewAdminDashboardController(
	userRepo *repositories.UserRepository,
	sessionRepo *repositories.InterviewSessionRepository,
	reportRepo *repositories.InterviewReportRepository,
) *AdminDashboardController {
	return &AdminDashboardController{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		reportRepo:  reportRepo,
	}
}

type UserScoreSummary struct {
	UserID        uint       `json:"user_id"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	RegisteredAt  time.Time  `json:"registered_at"`
	SessionCount  int64      `json:"session_count"`
	LastSessionAt *time.Time `json:"last_session_at,omitempty"`
	AvgScore      *float64   `json:"avg_score,omitempty"`
}

type SessionScoreEntry struct {
	SessionID uint               `json:"session_id"`
	EndedAt   *time.Time         `json:"ended_at,omitempty"`
	AvgScore  *float64           `json:"avg_score,omitempty"`
	Scores    map[string]float64 `json:"scores,omitempty"`
}

// avgScoresJSON parses ScoresJSON like {"logic":5,"specificity":4,...} and returns the mean.
func avgScoresJSON(scoresJSON string) (map[string]float64, *float64) {
	if scoresJSON == "" {
		return nil, nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(scoresJSON), &raw); err != nil {
		return nil, nil
	}
	scores := make(map[string]float64, len(raw))
	var sum float64
	var count int
	for k, v := range raw {
		switch val := v.(type) {
		case float64:
			scores[k] = val
			sum += val
			count++
		}
	}
	if count == 0 {
		return scores, nil
	}
	avg := sum / float64(count)
	return scores, &avg
}

// ListUsers handles GET /api/admin/dashboard/users
func (c *AdminDashboardController) ListUsers(ctx echo.Context) error {
	limit := echoIntQuery(ctx, "limit", 25)
	offset := (echoIntQuery(ctx, "page", 1) - 1) * limit
	query := ctx.QueryParam("query")
	sort := ctx.QueryParam("sort") // avg_score_asc | avg_score_desc | session_count_desc | registered_desc

	users, total, err := c.userRepo.ListUsersPaged(limit, offset, query)
	if err != nil {
		return echoInternalError(err)
	}

	userIDs := make([]uint, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	statMap, err := c.sessionRepo.GetUserStatsBatch(userIDs)
	if err != nil {
		return echoInternalError(err)
	}

	// Collect all finished session IDs to batch-fetch reports
	var allSessionIDs []uint
	sessionToUser := map[uint]uint{}
	for _, u := range users {
		ids, err := c.sessionRepo.ListFinishedSessionIDsByUser(u.ID)
		if err != nil {
			continue
		}
		for _, id := range ids {
			allSessionIDs = append(allSessionIDs, id)
			sessionToUser[id] = u.ID
		}
	}

	reports, err := c.reportRepo.FindBySessionIDs(allSessionIDs)
	if err != nil {
		return echoInternalError(err)
	}

	// Compute per-user avg scores
	userScoreSum := map[uint]float64{}
	userScoreCount := map[uint]int{}
	for _, rep := range reports {
		_, avg := avgScoresJSON(rep.ScoresJSON)
		if avg != nil {
			uid := sessionToUser[rep.SessionID]
			userScoreSum[uid] += *avg
			userScoreCount[uid]++
		}
	}

	summaries := make([]UserScoreSummary, 0, len(users))
	for _, u := range users {
		stat := statMap[u.ID]
		var avgScore *float64
		if cnt := userScoreCount[u.ID]; cnt > 0 {
			v := userScoreSum[u.ID] / float64(cnt)
			avgScore = &v
		}
		summaries = append(summaries, UserScoreSummary{
			UserID:        u.ID,
			Name:          u.Name,
			Email:         u.Email,
			Role:          u.TargetLevel,
			RegisteredAt:  u.CreatedAt,
			SessionCount:  stat.SessionCount,
			LastSessionAt: stat.LastSessionAt,
			AvgScore:      avgScore,
		})
	}

	// Client-side sort on the current page
	switch sort {
	case "avg_score_desc":
		stableSort(summaries, func(a, b UserScoreSummary) bool {
			av := scoreVal(a.AvgScore)
			bv := scoreVal(b.AvgScore)
			return av > bv
		})
	case "avg_score_asc":
		stableSort(summaries, func(a, b UserScoreSummary) bool {
			return scoreVal(a.AvgScore) < scoreVal(b.AvgScore)
		})
	case "session_count_desc":
		stableSort(summaries, func(a, b UserScoreSummary) bool {
			return a.SessionCount > b.SessionCount
		})
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"users": summaries,
		"total": total,
	})
}

func scoreVal(p *float64) float64 {
	if p == nil {
		return -1
	}
	return *p
}

// stableSort is a simple insertion sort (small slices only)
func stableSort(s []UserScoreSummary, less func(a, b UserScoreSummary) bool) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// UserSessions handles GET /api/admin/dashboard/users/:id/sessions
func (c *AdminDashboardController) UserSessions(ctx echo.Context) error {
	var userID uint
	if _, err := fmt.Sscanf(ctx.Param("id"), "%d", &userID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	sessionIDs, err := c.sessionRepo.ListFinishedSessionIDsByUser(userID)
	if err != nil {
		return echoInternalError(err)
	}

	reports, err := c.reportRepo.FindBySessionIDs(sessionIDs)
	if err != nil {
		return echoInternalError(err)
	}
	reportBySession := map[uint]*models.InterviewReport{}
	for i := range reports {
		reportBySession[reports[i].SessionID] = &reports[i]
	}

	sessions, err := c.sessionRepo.ListFinishedByUser(userID, 0)
	if err != nil {
		return echoInternalError(err)
	}

	entries := make([]SessionScoreEntry, 0, len(sessions))
	for _, s := range sessions {
		entry := SessionScoreEntry{SessionID: s.ID, EndedAt: s.EndedAt}
		if rep, ok := reportBySession[s.ID]; ok {
			scores, avg := avgScoresJSON(rep.ScoresJSON)
			entry.Scores = scores
			entry.AvgScore = avg
		}
		entries = append(entries, entry)
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{"sessions": entries})
}

// ExportCSV handles GET /api/admin/dashboard/export/csv
func (c *AdminDashboardController) ExportCSV(ctx echo.Context) error {
	users, _, err := c.userRepo.ListUsersPaged(10000, 0, "")
	if err != nil {
		return echoInternalError(err)
	}
	userIDs := make([]uint, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	statMap, _ := c.sessionRepo.GetUserStatsBatch(userIDs)

	var allSessionIDs []uint
	sessionToUser := map[uint]uint{}
	for _, u := range users {
		ids, _ := c.sessionRepo.ListFinishedSessionIDsByUser(u.ID)
		for _, id := range ids {
			allSessionIDs = append(allSessionIDs, id)
			sessionToUser[id] = u.ID
		}
	}
	reports, _ := c.reportRepo.FindBySessionIDs(allSessionIDs)
	userScoreSum := map[uint]float64{}
	userScoreCount := map[uint]int{}
	for _, rep := range reports {
		_, avg := avgScoresJSON(rep.ScoresJSON)
		if avg != nil {
			uid := sessionToUser[rep.SessionID]
			userScoreSum[uid] += *avg
			userScoreCount[uid]++
		}
	}

	w := ctx.Response().Writer
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"user_scores.csv\"")

	fmt.Fprint(w, "\xef\xbb\xbf") // Excel用BOM
	fmt.Fprintln(w, "ユーザーID,名前,メール,ロール,登録日,練習回数,最終練習日,平均スコア")
	for _, u := range users {
		stat := statMap[u.ID]
		avgScore := ""
		if cnt := userScoreCount[u.ID]; cnt > 0 {
			avgScore = fmt.Sprintf("%.2f", userScoreSum[u.ID]/float64(cnt))
		}
		lastSession := ""
		if stat.LastSessionAt != nil {
			lastSession = stat.LastSessionAt.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%d,%s,%s,%s,%s,%d,%s,%s\n",
			u.ID,
			csvEscape(u.Name),
			csvEscape(u.Email),
			u.TargetLevel,
			u.CreatedAt.Format("2006-01-02"),
			stat.SessionCount,
			lastSession,
			avgScore,
		)
	}
	return nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
