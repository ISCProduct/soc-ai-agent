// admin_company_catalog_controller.go — L1 カタログ (Seed / Coverage / Warm) と
// バッチ不足取得エンドポイント（AdminCompanyController のメソッド）。

package controllers

import (
	"Backend/internal/services/company"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// SeedL1Catalog POST /api/admin/companies/seed-l1
// multipart file=csv または body に CSV テキスト。未指定時は sample CSV を読む。
func (c *AdminCompanyController) SeedL1Catalog(ctx echo.Context) error {
	var reader io.Reader
	file, err := ctx.FormFile("file")
	if err == nil && file != nil {
		f, openErr := file.Open()
		if openErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, openErr.Error())
		}
		defer f.Close()
		reader = f
	} else if body := ctx.Request().Body; body != nil && ctx.Request().ContentLength > 0 &&
		strings.Contains(ctx.Request().Header.Get("Content-Type"), "text/csv") {
		reader = body
	} else {
		path := strings.TrimSpace(ctx.QueryParam("path"))
		if path == "" {
			path = "config/l1_catalog_seed.sample.csv"
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			// Backend 作業ディレクトリ差を吸収
			alt := filepath.Join("Backend", path)
			f2, err2 := os.Open(alt)
			if err2 != nil {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("seed file not found: %v", openErr))
			}
			defer f2.Close()
			reader = f2
		} else {
			defer f.Close()
			reader = f
		}
	}
	rows, err := company.ParseL1SeedCSV(reader)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	result, err := company.ImportL1Seed(c.repo, rows)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.seed_l1", "company", 0, map[string]any{
		"total":   result.Total,
		"created": result.Created,
		"updated": result.Updated,
	})
	return ctx.JSON(http.StatusOK, result)
}

// GetL1Coverage GET /api/admin/companies/l1-coverage
func (c *AdminCompanyController) GetL1Coverage(ctx echo.Context) error {
	if c.catalogWarm == nil {
		c.catalogWarm = company.NewCatalogWarmService(c.repo, c.infoFetcher, c.jobFetcher)
	}
	cov, err := c.catalogWarm.Coverage(ctx.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, cov)
}

// WarmL1Catalog POST /api/admin/companies/warm-l1
// Body: { "limit": 100, "dry_run": true, "force": false, "include_info": true, "include_persona": true, "concurrency": 6 }
// 企業間はconcurrency上限で並列処理する（1社内のinfo/personaは直列）。
func (c *AdminCompanyController) WarmL1Catalog(ctx echo.Context) error {
	if c.catalogWarm == nil {
		c.catalogWarm = company.NewCatalogWarmService(c.repo, c.infoFetcher, c.jobFetcher)
	}
	var opts company.L1WarmOptions
	opts.IncludeInfo = true
	opts.IncludePersona = true
	if err := ctx.Bind(&opts); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	// Bind で false が欠落した場合の既定は normalize 側でも補完する
	result, err := c.catalogWarm.WarmL1(ctx.Request().Context(), opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.warm_l1", "company", 0, map[string]any{
		"dry_run":    result.DryRun,
		"limit":      result.Limit,
		"processed":  result.Processed,
		"info_ok":    result.InfoOK,
		"persona_ok": result.PersonaOK,
		"errors":     result.Errors,
	})
	return ctx.JSON(http.StatusOK, result)
}

// FetchMissingBatch POST /api/admin/companies/fetch-missing-batch
// Body: { "limit": 30, "dry_run": true, "primary_only": true, "concurrency": 6 }
// アクティブ企業のうち不足フィールドだけを上限付きで埋める（企業間は並列）。
// primary_only=true のときは主3種（基本・技術・関係）のみ（求人除外）。
func (c *AdminCompanyController) FetchMissingBatch(ctx echo.Context) error {
	if c.missingBatch == nil {
		c.missingBatch = company.NewCompanyMissingBatchService(
			c.repo, c.infoFetcher, c.jobFetcher, c.techFetcher, c.relationsFetcher,
		)
	}
	var opts company.MissingBatchOptions
	if err := ctx.Bind(&opts); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	result, err := c.missingBatch.Run(ctx.Request().Context(), opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_missing_batch", "company", 0, map[string]any{
		"dry_run":      result.DryRun,
		"limit":        result.Limit,
		"primary_only": result.PrimaryOnly,
		"concurrency":  result.Concurrency,
		"candidate_n":  result.CandidateN,
		"processed":    result.Processed,
		"info_ok":      result.InfoOK,
		"jobs_ok":      result.JobsOK,
		"tech_ok":      result.TechOK,
		"relations_ok": result.RelationsOK,
		"errors":       result.Errors,
		"stop_reason":  result.StopReason,
		"failures":     result.FailureSamples(20),
	})
	return ctx.JSON(http.StatusOK, result)
}
