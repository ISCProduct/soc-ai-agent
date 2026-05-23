package controllers

import (
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type AdminCrawlController struct {
	service ifaces.CrawlService
	audit   ifaces.AuditLogService
}

func NewAdminCrawlController(service ifaces.CrawlService, audit ifaces.AuditLogService) *AdminCrawlController {
	return &AdminCrawlController{service: service, audit: audit}
}

func (c *AdminCrawlController) Sources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.listSources(w)
	case http.MethodPost:
		c.createSource(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *AdminCrawlController) SourceDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/crawl-sources/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "source id is required", http.StatusBadRequest)
		return
	}
	if strings.HasSuffix(path, "/run") {
		c.runSource(w, r)
		return
	}
	id, err := strconv.ParseUint(path, 10, 32)
	if err != nil {
		http.Error(w, "invalid source id", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.updateSource(w, r, uint(id))
}

func (c *AdminCrawlController) Runs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var sourceID uint
	if value := r.URL.Query().Get("source_id"); value != "" {
		if id, err := strconv.ParseUint(value, 10, 32); err == nil {
			sourceID = uint(id)
		}
	}
	runs, err := c.service.ListRuns(sourceID, 20)
	if err != nil {
		http.Error(w, "failed to fetch runs", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"runs": runs,
	})
}

func (c *AdminCrawlController) listSources(w http.ResponseWriter) {
	sources, err := c.service.ListSources()
	if err != nil {
		http.Error(w, "failed to fetch sources", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sources": sources,
	})
}

func (c *AdminCrawlController) createSource(w http.ResponseWriter, r *http.Request) {
	var payload services.CrawlSourcePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	source, err := c.service.CreateSource(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "crawl_source.create", "crawl_source", source.ID, map[string]interface{}{
		"name": source.Name,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(source)
}

func (c *AdminCrawlController) updateSource(w http.ResponseWriter, r *http.Request, id uint) {
	var payload services.CrawlSourcePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	source, err := c.service.UpdateSource(id, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "crawl_source.update", "crawl_source", source.ID, map[string]interface{}{
		"name": source.Name,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(source)
}

func (c *AdminCrawlController) runSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/crawl-sources/")
	path = strings.TrimSuffix(path, "/run")
	path = strings.Trim(path, "/")
	id, err := strconv.ParseUint(path, 10, 32)
	if err != nil {
		http.Error(w, "invalid source id", http.StatusBadRequest)
		return
	}
	run, err := c.service.RunSource(uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	actor := r.Header.Get("X-Admin-Email")
	c.audit.Record(actor, "crawl_source.run", "crawl_source", uint(id), map[string]interface{}{
		"status": run.Status,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}
