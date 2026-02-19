package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reedfamily/reedout/internal/docker"
)

type ServerHandler struct {
	db        *sql.DB
	docker    *docker.Client
	dataDir   string
	templates []docker.GameTemplate
}

type Server struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Game        string            `json:"game"`
	ContainerID string            `json:"container_id,omitempty"`
	Image       string            `json:"image"`
	Ports       []docker.PortMapping `json:"ports"`
	Env         map[string]string `json:"env"`
	Volumes     map[string]string `json:"volumes"`
	MemoryLimit int64             `json:"memory_limit"`
	CPULimit    float64           `json:"cpu_limit"`
	Status      string            `json:"status"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

func NewServerHandler(db *sql.DB, dockerClient *docker.Client, dataDir string, templates []docker.GameTemplate) *ServerHandler {
	return &ServerHandler{
		db:        db,
		docker:    dockerClient,
		dataDir:   dataDir,
		templates: templates,
	}
}

func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, name, game, container_id, image, ports, env, volumes, memory_limit, cpu_limit, status, created_at, updated_at FROM servers ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query servers")
		return
	}
	defer rows.Close()

	servers := []Server{}
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan server")
			return
		}
		servers = append(servers, s)
	}

	// Sync status with Docker concurrently with a 2s timeout
	// Fire-and-forget: if Docker is slow, just return DB status
	if len(servers) > 0 {
		type statusResult struct {
			idx    int
			status string
		}
		ch := make(chan statusResult, len(servers))
		statusCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		pending := 0
		for i, s := range servers {
			if s.ContainerID != "" {
				pending++
				go func(idx int, containerID, serverID string) {
					if status, err := h.docker.ContainerStatus(statusCtx, containerID); err == nil {
						h.db.Exec("UPDATE servers SET status = ? WHERE id = ?", status, serverID)
						ch <- statusResult{idx, status}
					} else {
						ch <- statusResult{idx, ""}
					}
				}(i, s.ContainerID, s.ID)
			}
		}

		for range pending {
			res := <-ch
			if res.status != "" {
				servers[res.idx].Status = res.status
			}
		}
	}

	writeJSON(w, http.StatusOK, servers)
}

func (h *ServerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, err := h.getServer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if s.ContainerID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if status, err := h.docker.ContainerStatus(ctx, s.ContainerID); err == nil {
			s.Status = status
			h.db.Exec("UPDATE servers SET status = ? WHERE id = ?", status, s.ID)
		}
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *ServerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string            `json:"name"`
		TemplateID string            `json:"template_id"`
		Env        map[string]string `json:"env"`
		Memory     string            `json:"memory"`
		CPU        float64           `json:"cpu"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.TemplateID == "" {
		writeError(w, http.StatusBadRequest, "name and template_id required")
		return
	}

	var tmpl *docker.GameTemplate
	for _, t := range h.templates {
		if t.ID == req.TemplateID {
			tmpl = &t
			break
		}
	}
	if tmpl == nil {
		writeError(w, http.StatusBadRequest, "template not found")
		return
	}

	id := uuid.New().String()[:8]
	containerName := fmt.Sprintf("reedout-%s-%s", tmpl.Game, id)

	// Merge template env with overrides
	env := make(map[string]string)
	for k, v := range tmpl.Env {
		env[k] = v
	}
	for k, v := range req.Env {
		env[k] = v
	}

	// Create data directory for server volumes
	serverDataDir := filepath.Join(h.dataDir, "servers", id)
	if err := os.MkdirAll(serverDataDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create data directory")
		return
	}

	// Resolve volumes (replace {data_dir} placeholder)
	volumes := make(map[string]string)
	for hostPath, containerPath := range tmpl.Volumes {
		resolved := hostPath
		if hostPath == "{data_dir}" {
			resolved = serverDataDir
		}
		volumes[resolved] = containerPath
	}

	ports := docker.ParsePortMappings(tmpl.Ports)
	memoryLimit := docker.ParseMemory(req.Memory)
	if memoryLimit == 0 {
		memoryLimit = docker.ParseMemory(tmpl.Memory)
	}
	cpuLimit := req.CPU
	if cpuLimit == 0 {
		cpuLimit = tmpl.CPU
	}

	// Pull image
	log.Printf("Pulling image %s...", tmpl.Image)
	if err := h.docker.PullImage(r.Context(), tmpl.Image); err != nil {
		log.Printf("Warning: failed to pull image (may already exist locally): %v", err)
	}

	// Create container
	containerID, err := h.docker.CreateContainer(r.Context(), docker.ContainerConfig{
		Name:        containerName,
		Image:       tmpl.Image,
		Env:         env,
		Ports:       ports,
		Volumes:     volumes,
		MemoryLimit: memoryLimit,
		CPULimit:    cpuLimit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create container: %v", err))
		return
	}

	// Save to database
	portsJSON, _ := json.Marshal(ports)
	envJSON, _ := json.Marshal(env)
	volumesJSON, _ := json.Marshal(volumes)

	_, err = h.db.Exec(`INSERT INTO servers (id, name, game, container_id, image, ports, env, volumes, memory_limit, cpu_limit, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, tmpl.Game, containerID, tmpl.Image,
		string(portsJSON), string(envJSON), string(volumesJSON),
		memoryLimit, cpuLimit, "created",
	)
	if err != nil {
		h.docker.RemoveContainer(context.Background(), containerID)
		writeError(w, http.StatusInternalServerError, "failed to save server")
		return
	}

	s, _ := h.getServer(id)
	writeJSON(w, http.StatusCreated, s)
}

func (h *ServerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	_, err := h.db.Exec("UPDATE servers SET name = ?, updated_at = ? WHERE id = ?", req.Name, time.Now(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update server")
		return
	}
	s, _ := h.getServer(id)
	writeJSON(w, http.StatusOK, s)
}

func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, err := h.getServer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	if s.ContainerID != "" {
		h.docker.RemoveContainer(r.Context(), s.ContainerID)
	}

	// Remove server data directory
	serverDataDir := filepath.Join(h.dataDir, "servers", id)
	os.RemoveAll(serverDataDir)

	h.db.Exec("DELETE FROM servers WHERE id = ?", id)
	writeJSON(w, http.StatusOK, map[string]string{"message": "server deleted"})
}

func (h *ServerHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, err := h.getServer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err := h.docker.StartContainer(r.Context(), s.ContainerID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start: %v", err))
		return
	}
	h.db.Exec("UPDATE servers SET status = 'running', updated_at = ? WHERE id = ?", time.Now(), id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (h *ServerHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, err := h.getServer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err := h.docker.StopContainer(r.Context(), s.ContainerID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to stop: %v", err))
		return
	}
	h.db.Exec("UPDATE servers SET status = 'exited', updated_at = ? WHERE id = ?", time.Now(), id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "exited"})
}

func (h *ServerHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, err := h.getServer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err := h.docker.RestartContainer(r.Context(), s.ContainerID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to restart: %v", err))
		return
	}
	h.db.Exec("UPDATE servers SET status = 'running', updated_at = ? WHERE id = ?", time.Now(), id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (h *ServerHandler) Templates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.templates)
}

func (h *ServerHandler) getServer(id string) (Server, error) {
	row := h.db.QueryRow(`SELECT id, name, game, container_id, image, ports, env, volumes, memory_limit, cpu_limit, status, created_at, updated_at FROM servers WHERE id = ?`, id)
	return scanServerRow(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanServerRow(row *sql.Row) (Server, error) {
	var s Server
	var portsJSON, envJSON, volumesJSON string
	var containerID sql.NullString
	err := row.Scan(&s.ID, &s.Name, &s.Game, &containerID, &s.Image, &portsJSON, &envJSON, &volumesJSON, &s.MemoryLimit, &s.CPULimit, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, err
	}
	s.ContainerID = containerID.String
	json.Unmarshal([]byte(portsJSON), &s.Ports)
	json.Unmarshal([]byte(envJSON), &s.Env)
	json.Unmarshal([]byte(volumesJSON), &s.Volumes)
	if s.Ports == nil {
		s.Ports = []docker.PortMapping{}
	}
	if s.Env == nil {
		s.Env = map[string]string{}
	}
	if s.Volumes == nil {
		s.Volumes = map[string]string{}
	}
	return s, nil
}

func scanServer(rows *sql.Rows) (Server, error) {
	var s Server
	var portsJSON, envJSON, volumesJSON string
	var containerID sql.NullString
	err := rows.Scan(&s.ID, &s.Name, &s.Game, &containerID, &s.Image, &portsJSON, &envJSON, &volumesJSON, &s.MemoryLimit, &s.CPULimit, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, err
	}
	s.ContainerID = containerID.String
	json.Unmarshal([]byte(portsJSON), &s.Ports)
	json.Unmarshal([]byte(envJSON), &s.Env)
	json.Unmarshal([]byte(volumesJSON), &s.Volumes)
	if s.Ports == nil {
		s.Ports = []docker.PortMapping{}
	}
	if s.Env == nil {
		s.Env = map[string]string{}
	}
	if s.Volumes == nil {
		s.Volumes = map[string]string{}
	}
	return s, nil
}
