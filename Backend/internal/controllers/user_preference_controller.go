package controllers

import (
	"context"
	"net/http"
	"strings"

	"Backend/internal/models"
	"Backend/internal/repositories"

	"github.com/labstack/echo/v4"
)

// scoutIndexSyncer は学生プロフィールのベクトルを同期する（hr.StudentIndexSyncer）。
type scoutIndexSyncer interface {
	Sync(ctx context.Context, userID uint)
}

// UserPreferenceController は学生本人の希望条件の取得・更新（#1094）。
// 企業向け学生検索のフィルタ軸（希望業界・希望勤務地）を学生が入力する。
type UserPreferenceController struct {
	repo       *repositories.UserPreferenceRepository
	syncer     scoutIndexSyncer
	industries industryLister
}

func NewUserPreferenceController(
	repo *repositories.UserPreferenceRepository,
	syncer scoutIndexSyncer,
	industries industryLister,
) *UserPreferenceController {
	return &UserPreferenceController{repo: repo, syncer: syncer, industries: industries}
}

// Industries GET /api/user/industries
// 学生が希望業界を選ぶための選択肢（企業側の /company-portal/industries と同一マスタ）。
func (c *UserPreferenceController) Industries(ctx echo.Context) error {
	if _, ok := echoUserID(ctx); !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	options, err := c.industries.ListActive()
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": options})
}

type userPreferenceBody struct {
	DesiredIndustryID *uint  `json:"desired_industry_id"`
	DesiredLocation   string `json:"desired_location"`
	Note              string `json:"note"`
	// AllowScoutVisibility は企業への公開同意。nil なら現在値を変更しない。
	AllowScoutVisibility *bool `json:"allow_scout_visibility"`
}

// userPreferenceResponse は希望条件に公開同意フラグを添えて返す。
type userPreferenceResponse struct {
	*models.UserPreference
	AllowScoutVisibility bool `json:"allow_scout_visibility"`
}

// Get GET /api/user/preferences
func (c *UserPreferenceController) Get(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	pref, err := c.repo.FindByUserID(userID)
	if err != nil {
		return echoInternalError(err)
	}
	if pref == nil {
		// 未設定でも 200 で空の希望条件を返し、フロントの分岐を減らす。
		pref = &models.UserPreference{UserID: userID}
	}
	allow, err := c.repo.GetScoutVisibility(userID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, userPreferenceResponse{UserPreference: pref, AllowScoutVisibility: allow})
}

const maxPreferenceLocationLength = 100

// Put PUT /api/user/preferences
func (c *UserPreferenceController) Put(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	var body userPreferenceBody
	if err := ctx.Bind(&body); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	location := strings.TrimSpace(body.DesiredLocation)
	if len([]rune(location)) > maxPreferenceLocationLength {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "希望勤務地が長すぎます")
	}
	pref := &models.UserPreference{
		UserID:            userID,
		DesiredIndustryID: body.DesiredIndustryID,
		DesiredLocation:   location,
		Note:              strings.TrimSpace(body.Note),
	}
	if err := c.repo.Upsert(pref); err != nil {
		return echoInternalError(err)
	}

	if body.AllowScoutVisibility != nil {
		if err := c.repo.SetScoutVisibility(userID, *body.AllowScoutVisibility); err != nil {
			return echoInternalError(err)
		}
	}
	allow, err := c.repo.GetScoutVisibility(userID)
	if err != nil {
		return echoInternalError(err)
	}
	// 希望条件はベクトル化対象なので、保存のたびに検索インデックスを同期する。
	if c.syncer != nil {
		c.syncer.Sync(ctx.Request().Context(), userID)
	}

	return ctx.JSON(http.StatusOK, userPreferenceResponse{UserPreference: pref, AllowScoutVisibility: allow})
}
