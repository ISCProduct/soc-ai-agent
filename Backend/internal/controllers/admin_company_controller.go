package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AdminCompanyController struct {
	repo         repository.CompanyRepository
	audit        ifaces.AuditLogService
	gbiz         *services.GBizInfoService
	openaiClient *openai.Client
}

func NewAdminCompanyController(repo repository.CompanyRepository, audit ifaces.AuditLogService, gbiz *services.GBizInfoService, openaiClient ...*openai.Client) *AdminCompanyController {
	ctrl := &AdminCompanyController{repo: repo, audit: audit, gbiz: gbiz}
	if len(openaiClient) > 0 {
		ctrl.openaiClient = openaiClient[0]
	}
	return ctrl
}

func (c *AdminCompanyController) ListOrCreate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.list(w, r)
	case http.MethodPost:
		c.create(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *AdminCompanyController) Detail(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/companies/")
	idStr = strings.Trim(idStr, "/")
	if idStr == "" {
		http.Error(w, "company id is required", http.StatusBadRequest)
		return
	}
	if idStr == "search-gbiz" {
		c.searchGBiz(w, r)
		return
	}
	if strings.HasSuffix(idStr, "/gbiz-sync") {
		c.syncGBiz(w, r, strings.TrimSuffix(idStr, "/gbiz-sync"))
		return
	}
	if strings.HasSuffix(idStr, "/tech-stack-search") {
		c.fetchTechStack(w, r, strings.TrimSuffix(idStr, "/tech-stack-search"))
		return
	}
	if strings.HasSuffix(idStr, "/publish") {
		c.publish(w, r, strings.TrimSuffix(idStr, "/publish"))
		return
	}
	if strings.HasSuffix(idStr, "/reject") {
		c.reject(w, r, strings.TrimSuffix(idStr, "/reject"))
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid company id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		c.get(w, uint(id))
	case http.MethodPut:
		c.update(w, r, uint(id))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *AdminCompanyController) publish(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseUint(strings.Trim(idStr, "/"), 10, 32)
	if err != nil {
		http.Error(w, "invalid company id", http.StatusBadRequest)
		return
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		http.Error(w, "company not found", http.StatusNotFound)
		return
	}
	company.DataStatus = "published"
	company.IsProvisional = false
	if err := c.repo.Update(company); err != nil {
		http.Error(w, "failed to publish company", http.StatusInternalServerError)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.publish", "company", company.ID, map[string]interface{}{
		"name": company.Name,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(company)
}

func (c *AdminCompanyController) reject(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseUint(strings.Trim(idStr, "/"), 10, 32)
	if err != nil {
		http.Error(w, "invalid company id", http.StatusBadRequest)
		return
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		http.Error(w, "company not found", http.StatusNotFound)
		return
	}
	company.IsActive = false
	if err := c.repo.Update(company); err != nil {
		http.Error(w, "failed to reject company", http.StatusInternalServerError)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.reject", "company", company.ID, map[string]interface{}{
		"name": company.Name,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})
}

func (c *AdminCompanyController) list(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	companies, err := c.repo.FindAllActive(limit, offset)
	if err != nil {
		http.Error(w, "failed to fetch companies", http.StatusInternalServerError)
		return
	}
	total, _ := c.repo.CountActive()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"companies": companies,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

func (c *AdminCompanyController) create(w http.ResponseWriter, r *http.Request) {
	var payload models.Company
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	applyCompanyDefaults(&payload)
	if err := c.repo.Create(&payload); err != nil {
		http.Error(w, "failed to create company", http.StatusInternalServerError)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.create", "company", payload.ID, map[string]interface{}{
		"name": payload.Name,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func (c *AdminCompanyController) get(w http.ResponseWriter, id uint) {
	company, err := c.repo.FindByID(id)
	if err != nil {
		http.Error(w, "company not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(company)
}

func (c *AdminCompanyController) update(w http.ResponseWriter, r *http.Request, id uint) {
	var payload models.Company
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	company, err := c.repo.FindByID(id)
	if err != nil {
		http.Error(w, "company not found", http.StatusNotFound)
		return
	}

	if err := mergeCompany(company, &payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.repo.Update(company); err != nil {
		http.Error(w, "failed to update company", http.StatusInternalServerError)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.update", "company", company.ID, map[string]interface{}{
		"name": company.Name,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(company)
}

// SearchGBizRoute は専用ルート /api/admin/companies/search-gbiz 用のパブリックハンドラ
func (c *AdminCompanyController) SearchGBizRoute(w http.ResponseWriter, r *http.Request) {
	c.searchGBiz(w, r)
}

func (c *AdminCompanyController) searchGBiz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if c.gbiz == nil {
		http.Error(w, "gbizinfo service not configured", http.StatusServiceUnavailable)
		return
	}
	results, err := c.gbiz.SearchByName(r.Context(), name)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}

func (c *AdminCompanyController) syncGBiz(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr = strings.Trim(idStr, "/")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid company id", http.StatusBadRequest)
		return
	}
	if c.gbiz == nil {
		http.Error(w, "gbizinfo service not configured", http.StatusServiceUnavailable)
		return
	}
	result, err := c.gbiz.SyncCompany(r.Context(), uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.gbiz_sync", "company", uint(id), map[string]interface{}{
		"status": result.Status,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// fetchTechStack はOpenAI WebSearchで企業の技術スタックを取得してDBを更新する
func (c *AdminCompanyController) fetchTechStack(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr = strings.Trim(idStr, "/")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid company id", http.StatusBadRequest)
		return
	}
	if c.openaiClient == nil {
		http.Error(w, "openai client not configured", http.StatusServiceUnavailable)
		return
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		http.Error(w, "company not found", http.StatusNotFound)
		return
	}

	prompt := fmt.Sprintf(
		`「%s」という日本のIT企業の技術スタックを調査してください。以下のJSON形式のみで回答してください（余分な説明は不要）。
{
  "tech_stack": ["言語・フレームワーク名（例: Go, React, TypeScript）"],
  "infra_stack": ["インフラ名（例: AWS, GCP, Azure, オンプレ）"],
  "cicd_tools": ["CI/CDツール名（例: GitHub Actions, Jenkins, CircleCI）"],
  "development_style": "開発手法（例: スクラム, ウォーターフォール, カンバン）"
}`,
		company.Name,
	)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	text, err := c.openaiClient.WebSearchQuery(ctx, prompt)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	// JSON部分を抽出
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		http.Error(w, "failed to parse web search response", http.StatusInternalServerError)
		return
	}

	type techStackResult struct {
		TechStack        []string `json:"tech_stack"`
		InfraStack       []string `json:"infra_stack"`
		CicdTools        []string `json:"cicd_tools"`
		DevelopmentStyle string   `json:"development_style"`
	}
	var result techStackResult
	if err := json.Unmarshal([]byte(text[start:end+1]), &result); err != nil {
		http.Error(w, "failed to parse tech stack json", http.StatusInternalServerError)
		return
	}

	// JSON配列をシリアライズしてDBに保存
	if len(result.TechStack) > 0 {
		if b, err := json.Marshal(result.TechStack); err == nil {
			company.TechStack = string(b)
		}
	}
	if len(result.InfraStack) > 0 {
		if b, err := json.Marshal(result.InfraStack); err == nil {
			company.InfraStack = string(b)
		}
	}
	if len(result.CicdTools) > 0 {
		if b, err := json.Marshal(result.CicdTools); err == nil {
			company.CicdTools = string(b)
		}
	}
	if result.DevelopmentStyle != "" {
		company.DevelopmentStyle = result.DevelopmentStyle
	}

	if err := c.repo.Update(company); err != nil {
		http.Error(w, "failed to update company", http.StatusInternalServerError)
		return
	}

	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.tech_stack_search", "company", company.ID, map[string]interface{}{
		"name": company.Name,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tech_stack":        result.TechStack,
		"infra_stack":       result.InfraStack,
		"cicd_tools":        result.CicdTools,
		"development_style": result.DevelopmentStyle,
	})
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
	existing.IsProvisional = payload.IsProvisional
	return nil
}
