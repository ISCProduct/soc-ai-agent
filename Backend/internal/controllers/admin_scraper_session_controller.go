package controllers

import (
	"Backend/internal/services"
	"Backend/internal/services/interfaces"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type AdminScraperSessionController struct {
	service interfaces.ScraperSessionService
}

func NewAdminScraperSessionController(service interfaces.ScraperSessionService) *AdminScraperSessionController {
	return &AdminScraperSessionController{service: service}
}

// Sessions GET /api/admin/scraper-sessions  POST /api/admin/scraper-sessions
func (c *AdminScraperSessionController) Sessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.list(w)
	case http.MethodPost:
		c.upsert(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// SessionDetail DELETE /api/admin/scraper-sessions/:site_key
func (c *AdminScraperSessionController) SessionDetail(w http.ResponseWriter, r *http.Request) {
	siteKey := strings.TrimPrefix(r.URL.Path, "/api/admin/scraper-sessions/")
	siteKey = strings.Trim(siteKey, "/")
	if siteKey == "" {
		http.Error(w, "site_key is required", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := c.service.Delete(siteKey); err != nil {
		http.Error(w, "failed to delete session", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *AdminScraperSessionController) list(w http.ResponseWriter) {
	sessions, err := c.service.List()
	if err != nil {
		http.Error(w, "failed to fetch sessions", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"sessions": sessions})
}

func (c *AdminScraperSessionController) upsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteKey   string  `json:"site_key"`
		Cookies   string  `json:"cookies"`
		ExpiresAt *string `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	payload := services.ScraperSessionPayload{
		SiteKey: req.SiteKey,
		Cookies: req.Cookies,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			http.Error(w, "invalid expires_at format (use RFC3339)", http.StatusBadRequest)
			return
		}
		payload.ExpiresAt = &t
	}

	session, err := c.service.Upsert(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(session)
}
