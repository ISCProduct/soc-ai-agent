package user

import (
	"context"
	"net/http"
	"strings"
	"time"

	"Backend/internal/controllers/httpapi"
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
// industryLister は業種マスタの取得のみを使う。Go の慣習に従い利用側で宣言する
// （company/student_controller.go にも同名の宣言があるが、そちらは企業側の利用分）。
type industryLister interface {
	ListActive() ([]repositories.IndustryOption, error)
}

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
	if _, ok := httpapi.UserID(ctx); !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	options, err := c.industries.ListActive()
	if err != nil {
		return httpapi.InternalError(err)
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
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	pref, err := c.repo.FindByUserID(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	if pref == nil {
		// 未設定でも 200 で空の希望条件を返し、フロントの分岐を減らす。
		pref = &models.UserPreference{UserID: userID}
	}
	allow, err := c.repo.GetScoutVisibility(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, userPreferenceResponse{UserPreference: pref, AllowScoutVisibility: allow})
}

const maxPreferenceLocationLength = 100

// Put PUT /api/user/preferences
func (c *UserPreferenceController) Put(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	var body userPreferenceBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	location := strings.TrimSpace(body.DesiredLocation)
	if len([]rune(location)) > maxPreferenceLocationLength {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "希望勤務地が長すぎます")
	}
	pref := &models.UserPreference{
		UserID:            userID,
		DesiredIndustryID: body.DesiredIndustryID,
		DesiredLocation:   location,
		Note:              strings.TrimSpace(body.Note),
	}
	if err := c.repo.Upsert(pref); err != nil {
		return httpapi.InternalError(err)
	}

	if body.AllowScoutVisibility != nil {
		if err := c.repo.SetScoutVisibility(userID, *body.AllowScoutVisibility); err != nil {
			return httpapi.InternalError(err)
		}
	}
	allow, err := c.repo.GetScoutVisibility(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	// 希望条件はベクトル化対象なので、保存のたびに検索インデックスを同期する。
	if c.syncer != nil {
		c.syncer.Sync(ctx.Request().Context(), userID)
	}

	return ctx.JSON(http.StatusOK, userPreferenceResponse{UserPreference: pref, AllowScoutVisibility: allow})
}

// trainingConsentBody は学習利用の同意の更新内容。
// AllowTrainingUse の未指定（nil）は 400 にする。同意は明示的な操作でだけ変えるため、送り忘れを false として扱わない。
// ConsentTextVersion は、画面が表示していた同意の文言の版（#2168）。必須で、今の版と一致しなければ 409 にする。
// サーバーが自分の版を記録するだけだと、古いタブ・古いフロントで古い文言を見て押した操作も、
// 新しい文言への同意として記録してしまうため。
type trainingConsentBody struct {
	AllowTrainingUse   *bool  `json:"allow_training_use"`
	ConsentTextVersion string `json:"consent_text_version"`
}

// trainingConsentResponse は同意の現在値と、本人が最後に変更した日時（#2168）を返す。
// UpdatedAt は同意の履歴の最後の行の日時で、一度も変更していなければ null。
// users.updated_at はほかの更新でも上書きされるので、同意の変更日時としては使わない。
type trainingConsentResponse struct {
	AllowTrainingUse bool       `json:"allow_training_use"`
	UpdatedAt        *time.Time `json:"updated_at"`
}

// trainingConsentResponseFor は、同意の現在値と最後に変更した日時を（1つのクエリで）読み、応答を組み立てる。
func (c *UserPreferenceController) trainingConsentResponseFor(userID uint) (trainingConsentResponse, error) {
	state, err := c.repo.GetTrainingConsent(userID)
	if err != nil {
		return trainingConsentResponse{}, err
	}
	return trainingConsentResponse{AllowTrainingUse: state.AllowTrainingUse, UpdatedAt: state.ChangedAt}, nil
}

// GetTrainingConsent GET /api/user/training-consent
//
// 面接データを採点モデルの学習に使うことへの同意（#2167）。希望条件（PUT /preferences）とは
// 別の口にする。PUT /preferences は希望条件をまるごと保存するため、同意だけを変える画面から
// 呼ぶと、希望条件を空で上書きしてしまう。
func (c *UserPreferenceController) GetTrainingConsent(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	resp, err := c.trainingConsentResponseFor(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// PutTrainingConsent PUT /api/user/training-consent
func (c *UserPreferenceController) PutTrainingConsent(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	var body trainingConsentBody
	if err := ctx.Bind(&body); err != nil || body.AllowTrainingUse == nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "allow_training_use を指定してください")
	}
	if body.ConsentTextVersion == "" {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "consent_text_version を指定してください")
	}
	// 表示していた文言が今の文言と違う（古いタブ・古いフロント）なら、何も保存せずに拒否する。
	// 画面は再読み込みを促し、今の文言を見せ直してから操作してもらう。
	if body.ConsentTextVersion != models.TrainingConsentTextVersion {
		return httpapi.NewAPIError(http.StatusConflict, httpapi.ErrCodeConflict, "同意の文言が更新されています。画面を再読み込みしてください")
	}
	// 値の更新と同意の履歴（どの文言の版に対する操作か）を同じトランザクションで保存する（#2168）。
	// どちらかが失敗すれば両方とも巻き戻るので、500 を返して保存できたように見せない。
	if err := c.repo.SetTrainingUse(userID, *body.AllowTrainingUse, body.ConsentTextVersion); err != nil {
		return httpapi.InternalError(err)
	}
	resp, err := c.trainingConsentResponseFor(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}
