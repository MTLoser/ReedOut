package api

import (
	"database/sql"
	"encoding/binary"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/reedfamily/reedout/internal/docker"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ConsoleHandler struct {
	db     *sql.DB
	docker *docker.Client
}

func NewConsoleHandler(db *sql.DB, dockerClient *docker.Client) *ConsoleHandler {
	return &ConsoleHandler{db: db, docker: dockerClient}
}

func (h *ConsoleHandler) Handle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var containerID string
	err := h.db.QueryRow("SELECT container_id FROM servers WHERE id = ?", id).Scan(&containerID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	// Check if container uses TTY (determines whether logs have stream headers)
	inspect, err := h.docker.InspectContainer(r.Context(), containerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to inspect container")
		return
	}
	isTTY := inspect.Config.Tty

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Stream logs
	logReader, err := h.docker.ContainerLogs(r.Context(), containerID, "100")
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer logReader.Close()

	// Attach for stdin
	attach, err := h.docker.ContainerAttach(r.Context(), containerID)
	if err != nil {
		log.Printf("attach error: %v", err)
	}

	// Read from WebSocket -> container stdin
	if attach.Conn != nil {
		defer attach.Close()
		go func() {
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				attach.Conn.Write(append(msg, '\n'))
			}
		}()
	}

	// Stream container logs -> WebSocket
	// For non-TTY containers, Docker multiplexes stdout/stderr with an 8-byte header:
	//   [stream_type(1)][0][0][0][size(4 big-endian)]
	if isTTY {
		// TTY mode: raw stream, no headers
		buf := make([]byte, 4096)
		for {
			n, err := logReader.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("log read error: %v", err)
				}
				return
			}
		}
	} else {
		// Non-TTY mode: strip 8-byte stream headers
		header := make([]byte, 8)
		for {
			if _, err := io.ReadFull(logReader, header); err != nil {
				if err != io.EOF {
					log.Printf("log header read error: %v", err)
				}
				return
			}

			size := binary.BigEndian.Uint32(header[4:8])
			if size == 0 {
				continue
			}

			payload := make([]byte, size)
			if _, err := io.ReadFull(logReader, payload); err != nil {
				if err != io.EOF {
					log.Printf("log payload read error: %v", err)
				}
				return
			}

			if writeErr := conn.WriteMessage(websocket.TextMessage, payload); writeErr != nil {
				return
			}
		}
	}
}
