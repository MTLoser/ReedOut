package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reedfamily/reedout/internal/scheduler"
)

type ScheduleHandler struct {
	db *sql.DB
}

func NewScheduleHandler(db *sql.DB) *ScheduleHandler {
	return &ScheduleHandler{db: db}
}

// List returns all schedules for a server.
func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	rows, err := h.db.Query(
		`SELECT id, server_id, name, cron_expr, action, enabled, COALESCE(last_run, ''), created_at
		FROM schedules WHERE server_id = ? ORDER BY created_at DESC`, serverID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}
	defer rows.Close()

	schedules := []scheduler.Schedule{}
	for rows.Next() {
		var s scheduler.Schedule
		var enabled int
		if err := rows.Scan(&s.ID, &s.ServerID, &s.Name, &s.CronExpr, &s.Action, &enabled, &s.LastRun, &s.CreatedAt); err != nil {
			continue
		}
		s.Enabled = enabled == 1
		schedules = append(schedules, s)
	}

	writeJSON(w, http.StatusOK, schedules)
}

// Create adds a new schedule.
func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	var req struct {
		Name     string `json:"name"`
		CronExpr string `json:"cron_expr"`
		Action   string `json:"action"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.CronExpr == "" || req.Action == "" {
		writeError(w, http.StatusBadRequest, "name, cron_expr, and action required")
		return
	}

	// Validate cron expression
	if _, err := scheduler.ParseCron(req.CronExpr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
		return
	}

	// Validate action
	switch req.Action {
	case "start", "stop", "restart", "backup":
		// valid
	default:
		writeError(w, http.StatusBadRequest, "action must be one of: start, stop, restart, backup")
		return
	}

	id := uuid.New().String()[:8]

	_, err := h.db.Exec(
		`INSERT INTO schedules (id, server_id, name, cron_expr, action) VALUES (?, ?, ?, ?, ?)`,
		id, serverID, req.Name, req.CronExpr, req.Action,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create schedule")
		return
	}

	// Return the created schedule
	var s scheduler.Schedule
	var enabled int
	h.db.QueryRow(
		`SELECT id, server_id, name, cron_expr, action, enabled, COALESCE(last_run, ''), created_at FROM schedules WHERE id = ?`, id,
	).Scan(&s.ID, &s.ServerID, &s.Name, &s.CronExpr, &s.Action, &enabled, &s.LastRun, &s.CreatedAt)
	s.Enabled = enabled == 1

	writeJSON(w, http.StatusCreated, s)
}

// Update modifies an existing schedule.
func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	scheduleID := chi.URLParam(r, "scheduleId")

	var req struct {
		Name     *string `json:"name"`
		CronExpr *string `json:"cron_expr"`
		Action   *string `json:"action"`
		Enabled  *bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CronExpr != nil {
		if _, err := scheduler.ParseCron(*req.CronExpr); err != nil {
			writeError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
			return
		}
	}

	if req.Action != nil {
		switch *req.Action {
		case "start", "stop", "restart", "backup":
		default:
			writeError(w, http.StatusBadRequest, "action must be one of: start, stop, restart, backup")
			return
		}
	}

	// Build dynamic update
	if req.Name != nil {
		h.db.Exec("UPDATE schedules SET name = ? WHERE id = ? AND server_id = ?", *req.Name, scheduleID, serverID)
	}
	if req.CronExpr != nil {
		h.db.Exec("UPDATE schedules SET cron_expr = ? WHERE id = ? AND server_id = ?", *req.CronExpr, scheduleID, serverID)
	}
	if req.Action != nil {
		h.db.Exec("UPDATE schedules SET action = ? WHERE id = ? AND server_id = ?", *req.Action, scheduleID, serverID)
	}
	if req.Enabled != nil {
		enabled := 0
		if *req.Enabled {
			enabled = 1
		}
		h.db.Exec("UPDATE schedules SET enabled = ? WHERE id = ? AND server_id = ?", enabled, scheduleID, serverID)
	}

	// Return updated schedule
	var s scheduler.Schedule
	var enabled int
	err := h.db.QueryRow(
		`SELECT id, server_id, name, cron_expr, action, enabled, COALESCE(last_run, ''), created_at FROM schedules WHERE id = ? AND server_id = ?`,
		scheduleID, serverID,
	).Scan(&s.ID, &s.ServerID, &s.Name, &s.CronExpr, &s.Action, &enabled, &s.LastRun, &s.CreatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	s.Enabled = enabled == 1

	writeJSON(w, http.StatusOK, s)
}

// Delete removes a schedule.
func (h *ScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	scheduleID := chi.URLParam(r, "scheduleId")

	result, err := h.db.Exec("DELETE FROM schedules WHERE id = ? AND server_id = ?", scheduleID, serverID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete schedule")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "schedule deleted"})
}
