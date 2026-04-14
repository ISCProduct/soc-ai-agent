package controllers

import (
	"Backend/internal/services"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type ScheduleController struct {
	service *services.ScheduleService
}

func NewScheduleController(service *services.ScheduleService) *ScheduleController {
	return &ScheduleController{service: service}
}

type scheduleRequest struct {
	CompanyName string `json:"company_name"`
	Stage       string `json:"stage"`
	Title       string `json:"title"`
	ScheduledAt string `json:"scheduled_at"`
	Notes       string `json:"notes"`
}

func parseScheduleRequest(r *http.Request) (scheduleRequest, error) {
	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func getUserID(r *http.Request) (uint, error) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		return 0, errors.New("user_id is required")
	}
	id, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid user_id")
	}
	return uint(id), nil
}

func getEventID(r *http.Request, pathPrefix string) (uint, error) {
	idStr := r.URL.Path[len(pathPrefix):]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("invalid event id")
	}
	return uint(id), nil
}

// List GET /api/schedule?user_id=X
func (c *ScheduleController) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	events, err := c.service.List(userID)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// Create POST /api/schedule?user_id=X
func (c *ScheduleController) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := parseScheduleRequest(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		http.Error(w, "Invalid scheduled_at format (RFC3339 expected)", http.StatusBadRequest)
		return
	}
	event, err := c.service.Create(userID, req.CompanyName, req.Stage, req.Title, scheduledAt, req.Notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

// Get GET /api/schedule/{id}?user_id=X
func (c *ScheduleController) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	eventID, err := getEventID(r, "/api/schedule/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	event, err := c.service.Get(userID, eventID)
	if err != nil {
		if err.Error() == "forbidden" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// Update PUT /api/schedule/{id}?user_id=X
func (c *ScheduleController) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	eventID, err := getEventID(r, "/api/schedule/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := parseScheduleRequest(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	var scheduledAt time.Time
	if req.ScheduledAt != "" {
		scheduledAt, err = time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			http.Error(w, "Invalid scheduled_at format (RFC3339 expected)", http.StatusBadRequest)
			return
		}
	}
	event, err := c.service.Update(userID, eventID, req.CompanyName, req.Stage, req.Title, scheduledAt, req.Notes)
	if err != nil {
		if err.Error() == "forbidden" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// Delete DELETE /api/schedule/{id}?user_id=X
func (c *ScheduleController) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	eventID, err := getEventID(r, "/api/schedule/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.service.Delete(userID, eventID); err != nil {
		if err.Error() == "forbidden" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RouteList dispatches /api/schedule by HTTP method
func (c *ScheduleController) RouteList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.List(w, r)
	case http.MethodPost:
		c.Create(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RouteByID dispatches /api/schedule/{id} by HTTP method
func (c *ScheduleController) RouteByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.Get(w, r)
	case http.MethodPut:
		c.Update(w, r)
	case http.MethodDelete:
		c.Delete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ExportICS GET /api/schedule/export/ics?user_id=X
func (c *ScheduleController) ExportICS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ics, err := c.service.ExportICS(userID)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"schedule.ics\"")
	w.Write([]byte(ics))
}
