package admin

import (
	"Backend/domain/repository"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/models"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// 企業の事業タグ（SOCAIAGENT-717）。相関図で分野から企業を探すために使う。

// maxBusinessTagBody は事業タグ更新で受け付けるリクエストボディの上限。
const maxBusinessTagBody = 64 << 10

// SetBusinessTagRepository は事業タグの保存先を注入する。未注入なら事業タグAPIは 501。
func (c *AdminCompanyController) SetBusinessTagRepository(r repository.CompanyBusinessTagRepository) {
	if c != nil {
		c.businessTags = r
	}
}

type businessTagsPayload struct {
	BusinessTags []string `json:"business_tags"`
}

// ListBusinessTags GET /api/admin/business-tags
// 登録前に既存のタグを選べるよう、共通のタグ一覧を返す（表記ゆれ防止）。
func (c *AdminCompanyController) ListBusinessTags(ctx echo.Context) error {
	if c.businessTags == nil {
		return echo.NewHTTPError(http.StatusNotImplemented, "business tags are not configured")
	}
	tags, err := c.businessTags.ListAll(ctx.Request().Context())
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"business_tags": tags})
}

// GetCompanyBusinessTags GET /api/admin/companies/:id/business-tags
func (c *AdminCompanyController) GetCompanyBusinessTags(ctx echo.Context) error {
	id, err := c.businessTagCompanyID(ctx)
	if err != nil {
		return err
	}
	return c.respondCompanyBusinessTags(ctx, id)
}

// UpdateCompanyBusinessTags PUT /api/admin/companies/:id/business-tags
// 本文 {"business_tags": ["クラウド", ...]} で丸ごと置き換える。空配列で全解除。
func (c *AdminCompanyController) UpdateCompanyBusinessTags(ctx echo.Context) error {
	id, err := c.businessTagCompanyID(ctx)
	if err != nil {
		return err
	}
	var payload businessTagsPayload
	body := http.MaxBytesReader(ctx.Response(), ctx.Request().Body, maxBusinessTagBody)
	if err := json.NewDecoder(body).Decode(&payload); err != nil || payload.BusinessTags == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "business_tags is required")
	}
	names, err := models.NormalizeBusinessTagNames(payload.BusinessTags)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.businessTags.ReplaceForCompany(ctx.Request().Context(), id, names); err != nil {
		return httpapi.InternalError(err)
	}
	c.audit.Record(ctx.Request().Header.Get("X-Admin-Email"), "company.business_tags.update", "company", id,
		map[string]any{"business_tags": names})
	return c.respondCompanyBusinessTags(ctx, id)
}

// businessTagCompanyID は企業IDを読み、企業が存在することを確かめる。
func (c *AdminCompanyController) businessTagCompanyID(ctx echo.Context) (uint, error) {
	if c.businessTags == nil {
		return 0, echo.NewHTTPError(http.StatusNotImplemented, "business tags are not configured")
	}
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if _, err := c.repo.FindByID(uint(id)); err != nil {
		return 0, echo.NewHTTPError(http.StatusNotFound, "company not found")
	}
	return uint(id), nil
}

func (c *AdminCompanyController) respondCompanyBusinessTags(ctx echo.Context, id uint) error {
	byID, err := c.businessTags.NamesByCompanyIDs(ctx.Request().Context(), []uint{id})
	if err != nil {
		return httpapi.InternalError(err)
	}
	tags := byID[id]
	if tags == nil {
		tags = []string{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"company_id": id, "business_tags": tags})
}
