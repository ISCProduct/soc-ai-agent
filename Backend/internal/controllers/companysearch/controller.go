// Package companysearch は学生向けの企業検索（SOCAIAGENT-612 / #2024）の API。
//
// 業界・地域（都道府県）・職種・名前で絞り込み、企業名順で返す（推薦や課金で上位に固定しない）。
// 検索と選択肢は既存の企業一覧（GET /api/companies）と同じく認証なしで使える。
// お気に入りの状態だけは本人の推薦に付くので、認証の下に置く。
package companysearch

import (
	"net/http"
	"strconv"
	"strings"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
)

// Store は検索の永続化（repositories.CompanySearchRepository が満たす）。
type Store interface {
	Search(f repositories.CompanySearchFilter) ([]repositories.CompanySearchRow, int64, error)
	SearchOptions() ([]string, []repositories.JobCategoryOption, error)
	FavoriteStates(userID uint, companyIDs []uint) ([]repositories.FavoriteState, error)
}

type Controller struct {
	store Store
}

func NewController(store Store) *Controller {
	return &Controller{store: store}
}

const (
	defaultLimit = 20
	// maxFavoriteIDs はお気に入りの状態を1回で問い合わせられる企業の数（1ページ分より十分大きく）
	maxFavoriteIDs = 100
	maxNameRunes   = 100
	// maxOffset は読み飛ばせる件数の上限（20件ずつで500ページ分）
	maxOffset = 10000
)

func badRequest(msg string) error {
	return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, msg)
}

// Search は GET /api/companies/search?name=&industry=&prefecture=&job_category_id=&limit=&offset=
func (c *Controller) Search(ctx echo.Context) error {
	f := repositories.CompanySearchFilter{
		Name:     strings.TrimSpace(ctx.QueryParam("name")),
		Industry: strings.TrimSpace(ctx.QueryParam("industry")),
		Limit:    httpapi.LimitQuery(ctx, "limit", defaultLimit),
	}
	if len([]rune(f.Name)) > maxNameRunes {
		return badRequest("name が長すぎます")
	}
	if p := strings.TrimSpace(ctx.QueryParam("prefecture")); p != "" {
		if !repositories.IsPrefecture(p) {
			return badRequest("prefecture が不正です")
		}
		f.Prefecture = p
	}
	if raw := ctx.QueryParam("job_category_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			return badRequest("job_category_id が不正です")
		}
		f.JobCategoryID = uint(id)
	}
	if raw := ctx.QueryParam("offset"); raw != "" {
		o, err := strconv.Atoi(raw)
		// 認証なしの公開 API なので、巨大な OFFSET で MySQL に大量の読み飛ばしをさせない
		if err != nil || o < 0 || o > maxOffset {
			return badRequest("offset が不正です")
		}
		f.Offset = o
	}
	rows, total, err := c.store.Search(f)
	if err != nil {
		return httpapi.InternalError(err)
	}
	if rows == nil {
		rows = []repositories.CompanySearchRow{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"companies": rows, "total": total, "limit": f.Limit, "offset": f.Offset})
}

// Options は GET /api/companies/search-options（業界・都道府県・職種の選択肢）。
func (c *Controller) Options(ctx echo.Context) error {
	industries, cats, err := c.store.SearchOptions()
	if err != nil {
		return httpapi.InternalError(err)
	}
	if industries == nil {
		industries = []string{}
	}
	if cats == nil {
		cats = []repositories.JobCategoryOption{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"industries":     industries,
		"prefectures":    repositories.Prefectures,
		"job_categories": cats,
	})
}

// Favorites は GET /api/companies/search/favorites?company_ids=1,2,3（本人の推薦にある企業のお気に入りの状態）。
// お気に入りは推薦（match_id）に付く。切り替えは既存の POST /api/chat/favorite を使う。
func (c *Controller) Favorites(ctx echo.Context) error {
	userID, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || userID == 0 {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	var ids []uint
	seen := map[uint]bool{}
	for _, part := range strings.Split(ctx.QueryParam("company_ids"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil || id == 0 {
			return badRequest("company_ids が不正です")
		}
		if !seen[uint(id)] {
			seen[uint(id)] = true
			ids = append(ids, uint(id))
		}
	}
	if len(ids) > maxFavoriteIDs {
		return badRequest("company_ids が多すぎます")
	}
	states, err := c.store.FavoriteStates(userID, ids)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"favorites": states})
}
