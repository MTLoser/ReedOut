package api

import (
	"database/sql"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/reedfamily/reedout/internal/backup"
)

type BackupHandler struct {
	db      *sql.DB
	backups *backup.Service
}

func NewBackupHandler(db *sql.DB, backupSvc *backup.Service) *BackupHandler {
	return &BackupHandler{db: db, backups: backupSvc}
}

// List returns all backups for a server.
func (h *BackupHandler) List(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backups, err := h.backups.List(serverID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}
	writeJSON(w, http.StatusOK, backups)
}

// Create creates a new backup for a server.
func (h *BackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	b, err := h.backups.Create(serverID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// Download sends a backup file to the client.
func (h *BackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backupID := chi.URLParam(r, "backupId")

	path, err := h.backups.FilePath(serverID, backupID)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
	w.Header().Set("Content-Type", "application/gzip")
	http.ServeFile(w, r, path)
}

// Delete removes a backup.
func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backupID := chi.URLParam(r, "backupId")

	if err := h.backups.Delete(serverID, backupID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "backup deleted"})
}

// Restore restores a backup. Server must be stopped first.
func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backupID := chi.URLParam(r, "backupId")

	// Check server is not running
	var status string
	err := h.db.QueryRow("SELECT status FROM servers WHERE id = ?", serverID).Scan(&status)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if status == "running" {
		writeError(w, http.StatusConflict, "stop the server before restoring a backup")
		return
	}

	if err := h.backups.Restore(serverID, backupID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore backup: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "backup restored"})
}
