package api

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/reedfamily/reedout/internal/stats"
)

type StatsHandler struct {
	db        *sql.DB
	collector *stats.Collector
}

func NewStatsHandler(db *sql.DB, collector *stats.Collector) *StatsHandler {
	return &StatsHandler{db: db, collector: collector}
}

// Latest returns the most recent stats row for a server.
func (h *StatsHandler) Latest(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	var s stats.Stats
	err := h.db.QueryRow(
		`SELECT id, server_id, cpu_percent, memory_bytes, memory_limit, disk_bytes, network_rx, network_tx, recorded_at
		FROM stats WHERE server_id = ? ORDER BY recorded_at DESC LIMIT 1`, serverID,
	).Scan(&s.ID, &s.ServerID, &s.CPUPercent, &s.MemoryBytes, &s.MemoryLimit, &s.DiskBytes, &s.NetworkRx, &s.NetworkTx, &s.RecordedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "no stats available")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query stats")
		return
	}

	writeJSON(w, http.StatusOK, s)
}

// History returns stats rows for a time range.
func (h *StatsHandler) History(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1h"
	}

	duration, err := parsePeriod(period)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid period: use format like 1h, 6h, 24h")
		return
	}

	since := time.Now().Add(-duration).UTC().Format("2006-01-02 15:04:05")

	rows, err := h.db.Query(
		`SELECT id, server_id, cpu_percent, memory_bytes, memory_limit, disk_bytes, network_rx, network_tx, recorded_at
		FROM stats WHERE server_id = ? AND recorded_at >= ? ORDER BY recorded_at ASC`, serverID, since,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query stats")
		return
	}
	defer rows.Close()

	result := []stats.Stats{}
	for rows.Next() {
		var s stats.Stats
		if err := rows.Scan(&s.ID, &s.ServerID, &s.CPUPercent, &s.MemoryBytes, &s.MemoryLimit, &s.DiskBytes, &s.NetworkRx, &s.NetworkTx, &s.RecordedAt); err != nil {
			continue
		}
		result = append(result, s)
	}

	writeJSON(w, http.StatusOK, result)
}

// Live pushes stats via WebSocket every time the collector produces a new reading.
func (h *StatsHandler) Live(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("stats websocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	ch := h.collector.Subscribe(serverID)
	defer h.collector.Unsubscribe(serverID, ch)

	// Send latest immediately if available
	if latest := h.collector.Latest(serverID); latest != nil {
		if err := conn.WriteJSON(latest); err != nil {
			return
		}
	}

	// Read from client to detect disconnect
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case s, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(s); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func parsePeriod(s string) (time.Duration, error) {
	// Support simple formats: 1h, 6h, 24h, 30m
	return time.ParseDuration(s)
}
