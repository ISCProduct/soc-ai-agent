package admin

import (
	"net/http"
	"time"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/repositories"
	"Backend/internal/services/funnel"

	"github.com/labstack/echo/v4"
)

// FunnelSource はファネルの元データ（対象の学生と段階ごとの到達者）を返す（#2039）。
type FunnelSource interface {
	Students(schoolID *uint) ([]uint, error)
	StageUserIDs(schoolID *uint, p repositories.FunnelPeriod) (map[string][]uint, error)
}

// SetFunnelSource はファネルの集計に使うリポジトリを設定する（#2039）。
func (c *AdminDashboardController) SetFunnelSource(src FunnelSource) {
	c.funnel = src
}

// funnelDateLayout は期間の指定の形式（日付のみ。日本時間の0時で区切る）。
const funnelDateLayout = "2006-01-02"

var funnelLocation = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("JST", 9*60*60)
}()

// parseFunnelPeriod は from / to（YYYY-MM-DD、どちらも含む）を集計期間にする。
// to はその日の終わりまで含めるため、翌日の0時（含まない）にする。
func parseFunnelPeriod(from, to string) (repositories.FunnelPeriod, error) {
	var p repositories.FunnelPeriod
	if from != "" {
		t, err := time.ParseInLocation(funnelDateLayout, from, funnelLocation)
		if err != nil {
			return p, echo.NewHTTPError(http.StatusBadRequest, "from は YYYY-MM-DD で指定してください")
		}
		p.From = &t
	}
	if to != "" {
		t, err := time.ParseInLocation(funnelDateLayout, to, funnelLocation)
		if err != nil {
			return p, echo.NewHTTPError(http.StatusBadRequest, "to は YYYY-MM-DD で指定してください")
		}
		end := t.AddDate(0, 0, 1)
		p.To = &end
	}
	if p.From != nil && p.To != nil && !p.From.Before(*p.To) {
		return p, echo.NewHTTPError(http.StatusBadRequest, "from は to 以前の日付にしてください")
	}
	return p, nil
}

// Funnel は診断 → 面接 → ES → 応募 → 内定のファネルを返す（#2039）。
//
// GET /api/admin/dashboard/funnel?school_id=&from=YYYY-MM-DD&to=YYYY-MM-DD
// 担当校スコープ（EchoAdminSchoolScope）の下に置く。担当校を持つ管理者は school_id が必須で、
// 担当校以外は 403 になる。担当校を持たないシステム管理者だけが school_id なし（全体）で見られる。
func (c *AdminDashboardController) Funnel(ctx echo.Context) error {
	if c.funnel == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "funnel is not configured")
	}
	schoolID, err := httpapi.AdminSchoolFilter(ctx)
	if err != nil {
		return err
	}
	period, err := parseFunnelPeriod(ctx.QueryParam("from"), ctx.QueryParam("to"))
	if err != nil {
		return err
	}
	students, err := c.funnel.Students(schoolID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	reached, err := c.funnel.StageUserIDs(schoolID, period)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"school_id": schoolID,
		"from":      ctx.QueryParam("from"),
		"to":        ctx.QueryParam("to"),
		"funnel":    funnel.Build(students, reached),
	})
}
