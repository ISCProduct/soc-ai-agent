package admin

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/company"
	"Backend/internal/services/gbizinfo"
	ifaces "Backend/internal/services/interfaces"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type AdminCompanyController struct {
	repo             repository.CompanyRepository
	audit            ifaces.AuditLogService
	gbiz             *gbizinfo.GBizInfoService
	openaiClient     *openai.Client
	infoFetcher      *company.CompanyInfoFetcher
	relationsFetcher *company.CompanyRelationsFetcher
	jobFetcher       *company.JobFetchService
	techFetcher      *company.TechStackFetcher
	catalogWarm      *company.CatalogWarmService
	missingBatch     *company.CompanyMissingBatchService
	// schoolRestricted は「担当校つき管理者か」を返す。公開状態の変更可否の判定に使う。
	schoolRestricted func(adminUserID uint) (bool, error)
}

func NewAdminCompanyController(repo repository.CompanyRepository, audit ifaces.AuditLogService, gbiz *gbizinfo.GBizInfoService, openaiClient ...*openai.Client) *AdminCompanyController {
	ctrl := &AdminCompanyController{repo: repo, audit: audit, gbiz: gbiz}
	if len(openaiClient) > 0 {
		ctrl.openaiClient = openaiClient[0]
		ctrl.infoFetcher = company.NewCompanyInfoFetcher(repo, openaiClient[0], gbiz)
		ctrl.jobFetcher = company.NewJobFetchService(repo, openaiClient[0])
		ctrl.techFetcher = company.NewTechStackFetcher(repo, openaiClient[0])
		ctrl.catalogWarm = company.NewCatalogWarmService(repo, ctrl.infoFetcher, ctrl.jobFetcher)
		ctrl.missingBatch = company.NewCompanyMissingBatchService(
			repo, ctrl.infoFetcher, ctrl.jobFetcher, ctrl.techFetcher, nil,
		)
	}
	return ctrl
}

// SetSchoolRestrictionChecker は担当校つき管理者かどうかの判定を注入する。
// 未注入だと公開状態の変更を止められないので、必ず main で渡すこと。
func (c *AdminCompanyController) SetSchoolRestrictionChecker(f func(adminUserID uint) (bool, error)) {
	if c != nil {
		c.schoolRestricted = f
	}
}

// SetInfoFetcher は基本情報取得サービスを差し替える。
//
// コンストラクタは openaiClient を受け取ると infoFetcher を自前生成する。
// その生成物には main.go 側で行う SetSharedSearch が掛かっていないため、
// 管理画面と fetch-missing-batch は検索結果の共有(#1124)が効かないまま
// 系統ごとに web_search を発行していた。件数が出るのはこのバッチ経路なので、
// 共有済みのインスタンスをここで渡し直す。
//
// infoFetcher を差し替えたら、それを抱えている catalogWarm と missingBatch も
// 作り直す必要がある。片方だけ直すと古い fetcher が残る。
func (c *AdminCompanyController) SetInfoFetcher(fetcher *company.CompanyInfoFetcher) {
	if c == nil || fetcher == nil {
		return
	}
	c.infoFetcher = fetcher
	c.catalogWarm = company.NewCatalogWarmService(c.repo, fetcher, c.jobFetcher)
	c.missingBatch = company.NewCompanyMissingBatchService(
		c.repo, fetcher, c.jobFetcher, c.techFetcher, c.relationsFetcher,
	)
}

// SetRelationsFetcher は企業関係・市場情報取得サービスを注入する（#633 Phase 2）。
func (c *AdminCompanyController) SetRelationsFetcher(fetcher *company.CompanyRelationsFetcher) {
	if c != nil {
		c.relationsFetcher = fetcher
		if c.missingBatch != nil || c.infoFetcher != nil {
			c.missingBatch = company.NewCompanyMissingBatchService(
				c.repo, c.infoFetcher, c.jobFetcher, c.techFetcher, fetcher,
			)
		}
	}
}

// SetCompanySearchGuards は FirstTouch Search の予算・singleflight を注入する（#587）。
func (c *AdminCompanyController) SetCompanySearchGuards(budget companyfetch.SearchBudget, flight *company.CompanySearchFlight) {
	if c == nil {
		return
	}
	if c.infoFetcher != nil {
		c.infoFetcher.SetSearchBudget(budget)
		c.infoFetcher.SetSearchFlight(flight)
	}
	if c.jobFetcher != nil {
		c.jobFetcher.SetSearchBudget(budget)
		c.jobFetcher.SetSearchFlight(flight)
	}
	if c.techFetcher != nil {
		c.techFetcher.SetSearchBudget(budget)
		c.techFetcher.SetSearchFlight(flight)
	}
	if c.relationsFetcher != nil {
		c.relationsFetcher.SetSearchBudget(budget)
		c.relationsFetcher.SetSearchFlight(flight)
	}
}

// List GET /api/admin/companies
func (c *AdminCompanyController) List(ctx echo.Context) error {
	// 他の管理系コントローラー(user/school/organization)と同じ 100 で頭打ちにする。
	// 上限が無いと limit=1000000 で DB と JSON 生成にタスクが詰まる(#1412)。
	const maxListLimit = 100
	limit := 50
	offset := 0
	if v, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && v > 0 {
		limit = min(v, maxListLimit)
	}
	if v, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && v >= 0 {
		offset = v
	}
	name := strings.TrimSpace(ctx.QueryParam("name"))
	status := strings.TrimSpace(ctx.QueryParam("status"))
	industry := strings.TrimSpace(ctx.QueryParam("industry"))
	readiness := strings.TrimSpace(ctx.QueryParam("readiness"))
	orderBy := strings.TrimSpace(ctx.QueryParam("order"))
	// 企業カタログは共有のため school_id は「承認済みだけ見る」任意の絞り込み(閲覧制限ではない)。
	var schoolID *uint
	if raw := ctx.QueryParam("school_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			v := uint(id)
			schoolID = &v
		}
	}
	companies, total, err := c.repo.ListActiveFiltered(limit, offset, name, status, industry, readiness, orderBy, schoolID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch companies")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"companies": companies,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
		"name":      name,
		"status":    status,
		"industry":  industry,
		"readiness": readiness,
		"order":     orderBy,
	})
}

// Industries GET /api/admin/companies/industries
// アクティブ企業に付いている業界名の一覧を返す（絞り込み用）。
func (c *AdminCompanyController) Industries(ctx echo.Context) error {
	industries, err := c.repo.ListActiveIndustries()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch industries")
	}
	if industries == nil {
		industries = []string{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"industries": industries})
}

// Create POST /api/admin/companies
func (c *AdminCompanyController) Create(ctx echo.Context) error {
	var payload models.Company
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if strings.TrimSpace(payload.Name) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	applyCompanyDefaults(&payload)
	// AI プレビュー経由で作成した場合は取得メタを残し、公開後も再取得判断できるようにする
	if strings.TrimSpace(payload.LastModelUsed) != "" && payload.InfoFetchedAt == nil {
		now := time.Now()
		payload.InfoFetchedAt = &now
	}
	if err := c.repo.Create(&payload); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create company")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.create", "company", payload.ID, map[string]any{
		"name": payload.Name,
	})
	return ctx.JSON(http.StatusOK, payload)
}

// Get GET /api/admin/companies/:id
func (c *AdminCompanyController) Get(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}
	return ctx.JSON(http.StatusOK, company)
}

// maxCompanyUpdateBody は企業更新で受け付けるリクエストボディの上限。
// 企業1件ぶんのテキストなので 1MiB あれば足りる。
const maxCompanyUpdateBody = 1 << 20

// Update PUT /api/admin/companies/:id
func (c *AdminCompanyController) Update(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	// 送られてこなかった項目を「false / 空」で上書きしないため、キーの有無も見る。
	// ctx.Bind と違い全量をメモリに載せるので上限を付ける。
	body, err := io.ReadAll(http.MaxBytesReader(ctx.Response(), ctx.Request().Body, maxCompanyUpdateBody))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	var payload models.Company
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	var sentFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &sentFields); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	_, provisionalSent := sentFields["is_provisional"]

	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}

	// data_status / is_provisional は学校をまたいで共有される。PATCH /publish と同じ権限で守らないと
	// 「編集」からの公開で platform 限定を素通りできてしまう。
	publicationChanged := changesPublication(company, &payload, provisionalSent)
	if publicationChanged {
		if err := c.requirePlatformAdmin(ctx); err != nil {
			return err
		}
		// 公開に進めるなら Publish と同じ前提を満たすこと。重み付けプロファイルが無い企業を
		// 公開すると、マッチングが既定値(全軸50)で計算されて「なぜか高得点の企業」になる。
		// なお Publish が行う draft 求人の一括公開はここでは行わない
		// （求人が published にならないだけで、過剰公開の方向には倒れない）。
		if payload.DataStatus == "published" && company.DataStatus != "published" {
			if err := c.requireWeightProfile(uint(id)); err != nil {
				return err
			}
		}
	}

	if err := mergeCompany(company, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if provisionalSent {
		company.IsProvisional = payload.IsProvisional
	}
	if err := c.repo.Update(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update company")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	meta := map[string]any{"name": company.Name}
	if publicationChanged {
		// 「誰がいつ公開したか」を company.update の中に埋もれさせない
		meta["data_status"] = company.DataStatus
		meta["is_provisional"] = company.IsProvisional
	}
	c.audit.Record(actor, "company.update", "company", company.ID, meta)
	return ctx.JSON(http.StatusOK, company)
}

// changesPublication は編集内容が掲載状態（公開／暫定フラグ）を動かすかを見る。
// provisionalSent はリクエストボディに is_provisional が含まれていたか。
// bool のゼロ値と「未送信」は区別できないので、送られた場合だけ比較する。
func changesPublication(existing, payload *models.Company, provisionalSent bool) bool {
	if payload.DataStatus != "" && payload.DataStatus != existing.DataStatus {
		return true
	}
	return provisionalSent && payload.IsProvisional != existing.IsProvisional
}

// requireWeightProfile は公開前に重み付けプロファイルがあることを確かめる（Publish と同じ前提）。
func (c *AdminCompanyController) requireWeightProfile(companyID uint) error {
	profile, err := c.repo.GetWeightProfile(companyID, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusBadRequest, "weight profile is required before publish")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load weight profile")
	}
	if profile == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "weight profile is required before publish")
	}
	return nil
}

// requirePlatformAdmin は担当校つき管理者を 403 で弾く（ルートの platform ミドルウェアと同じ判定）。
func (c *AdminCompanyController) requirePlatformAdmin(ctx echo.Context) error {
	if c.schoolRestricted == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "school access checker is not configured")
	}
	adminUserID, ok := middleware.AdminUserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	restricted, err := c.schoolRestricted(adminUserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve school access")
	}
	if restricted {
		return echo.NewHTTPError(http.StatusForbidden, "platform admin only")
	}
	return nil
}

// Publish PATCH /api/admin/companies/:id/publish
func (c *AdminCompanyController) Publish(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	companyID := uint(id)
	company, err := c.repo.FindByID(companyID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}
	profile, err := c.repo.GetWeightProfile(companyID, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusBadRequest, "weight profile is required before publish")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load weight profile")
	}
	if profile == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "weight profile is required before publish")
	}
	company.DataStatus = "published"
	company.IsProvisional = false
	if err := c.repo.Update(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to publish company")
	}
	cid := companyID
	jobs, err := c.repo.ListJobPositions(&cid, nil, 1000)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list job positions")
	}
	for i := range jobs {
		if jobs[i].DataStatus != "draft" {
			continue
		}
		jobs[i].DataStatus = "published"
		if err := c.repo.UpdateJobPosition(&jobs[i]); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to publish job positions")
		}
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.publish", "company", company.ID, map[string]any{
		"name": company.Name,
	})
	return ctx.JSON(http.StatusOK, company)
}

// Reject PATCH /api/admin/companies/:id/reject
func (c *AdminCompanyController) Reject(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}
	company.IsActive = false
	if err := c.repo.Update(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reject company")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.reject", "company", company.ID, map[string]any{
		"name": company.Name,
	})
	return ctx.JSON(http.StatusOK, map[string]string{"status": "rejected"})
}

// SearchGBiz GET /api/admin/companies/search-gbiz?name=xxx
func (c *AdminCompanyController) SearchGBiz(ctx echo.Context) error {
	name := strings.TrimSpace(ctx.QueryParam("name"))
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	if c.gbiz == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gbizinfo service not configured")
	}
	results, err := c.gbiz.SearchByName(ctx.Request().Context(), name)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"results": results})
}

// SyncGBiz POST /api/admin/companies/:id/gbiz-sync
func (c *AdminCompanyController) SyncGBiz(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.gbiz == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gbizinfo service not configured")
	}
	result, err := c.gbiz.SyncCompany(ctx.Request().Context(), uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.gbiz_sync", "company", uint(id), map[string]any{
		"status": result.Status,
	})
	return ctx.JSON(http.StatusOK, result)
}

func applyCompanyDefaults(company *models.Company) {
	if strings.TrimSpace(company.SourceType) == "" {
		company.SourceType = "manual"
	}
	if strings.TrimSpace(company.DataStatus) == "" {
		company.DataStatus = "draft"
	}
	if company.SourceFetchedAt == nil {
		now := time.Now()
		company.SourceFetchedAt = &now
	}
	if company.IsProvisional == false && strings.TrimSpace(company.SourceURL) == "" {
		company.IsProvisional = true
	}
	if strings.TrimSpace(company.Name) != "" && company.IsProvisional == false {
		company.IsVerified = true
	}
}

func mergeCompany(existing *models.Company, payload *models.Company) error {
	if strings.TrimSpace(payload.Name) != "" {
		existing.Name = payload.Name
	}
	if strings.TrimSpace(payload.Description) != "" {
		existing.Description = payload.Description
	}
	if strings.TrimSpace(payload.Industry) != "" {
		existing.Industry = payload.Industry
	}
	if strings.TrimSpace(payload.Location) != "" {
		existing.Location = payload.Location
	}
	if payload.EmployeeCount > 0 {
		existing.EmployeeCount = payload.EmployeeCount
	}
	if basis := models.NormalizeEmployeeCountBasis(payload.EmployeeCountBasis); basis != "" {
		existing.EmployeeCountBasis = basis
	}
	if payload.FoundedYear > 0 {
		existing.FoundedYear = payload.FoundedYear
	}
	if strings.TrimSpace(payload.WebsiteURL) != "" {
		existing.WebsiteURL = payload.WebsiteURL
	}
	if strings.TrimSpace(payload.LogoURL) != "" {
		existing.LogoURL = payload.LogoURL
	}
	if strings.TrimSpace(payload.CorporateNumber) != "" {
		existing.CorporateNumber = payload.CorporateNumber
	}
	if strings.TrimSpace(payload.MainBusiness) != "" {
		existing.MainBusiness = payload.MainBusiness
	}
	if strings.TrimSpace(payload.Culture) != "" {
		existing.Culture = payload.Culture
	}
	if strings.TrimSpace(payload.WorkStyle) != "" {
		existing.WorkStyle = payload.WorkStyle
	}
	if strings.TrimSpace(payload.WelfareDetails) != "" {
		existing.WelfareDetails = payload.WelfareDetails
	}
	if strings.TrimSpace(payload.TechStack) != "" {
		existing.TechStack = payload.TechStack
	}
	if strings.TrimSpace(payload.InfraStack) != "" {
		existing.InfraStack = payload.InfraStack
	}
	if strings.TrimSpace(payload.CicdTools) != "" {
		existing.CicdTools = payload.CicdTools
	}
	if strings.TrimSpace(payload.DevelopmentStyle) != "" {
		existing.DevelopmentStyle = payload.DevelopmentStyle
	}
	if strings.TrimSpace(payload.SourceType) != "" {
		existing.SourceType = payload.SourceType
	}
	if strings.TrimSpace(payload.SourceURL) != "" {
		existing.SourceURL = payload.SourceURL
	}
	if payload.SourceFetchedAt != nil {
		existing.SourceFetchedAt = payload.SourceFetchedAt
	}
	if payload.DataStatus != "" {
		if payload.DataStatus != "draft" && payload.DataStatus != "published" {
			return errors.New("data_status must be draft or published")
		}
		existing.DataStatus = payload.DataStatus
	}
	// IsProvisional は呼び出し側が「送られてきた場合だけ」反映する。
	return nil
}
