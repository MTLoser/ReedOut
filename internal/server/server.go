package server

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/reedfamily/reedout/internal/api"
	"github.com/reedfamily/reedout/internal/auth"
	"github.com/reedfamily/reedout/internal/backup"
	"github.com/reedfamily/reedout/internal/config"
	"github.com/reedfamily/reedout/internal/docker"
	"github.com/reedfamily/reedout/internal/scheduler"
	"github.com/reedfamily/reedout/internal/stats"

	// Register game adapters
	_ "github.com/reedfamily/reedout/internal/game/minecraft"
	_ "github.com/reedfamily/reedout/internal/game/vintagestory"
)

type Server struct {
	cfg       *config.Config
	db        *sql.DB
	router    chi.Router
	collector *stats.Collector
	scheduler *scheduler.Scheduler
}

func New(cfg *config.Config, db *sql.DB) (*Server, error) {
	// Initialize auth
	authSvc := auth.NewService(db)
	if err := authSvc.EnsureDefaultUser(cfg.DefaultUser, cfg.DefaultPass); err != nil {
		return nil, fmt.Errorf("ensure default user: %w", err)
	}

	// Initialize Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	// Load templates
	templates, err := docker.LoadTemplates(cfg.TemplatePath)
	if err != nil {
		log.Printf("Warning: failed to load templates: %v", err)
		templates = []docker.GameTemplate{}
	}

	// Start stats collector
	collector := stats.NewCollector(db, dockerClient)
	collector.Start()

	// Initialize backup service
	backupSvc := backup.NewService(db, cfg.DataDir)

	// Start scheduler
	sched := scheduler.New(db, dockerClient, backupSvc)
	sched.Start()

	// Create handlers
	authHandler := api.NewAuthHandler(authSvc)
	serverHandler := api.NewServerHandler(db, dockerClient, cfg.DataDir, templates)
	consoleHandler := api.NewConsoleHandler(db, dockerClient)
	statsHandler := api.NewStatsHandler(db, collector)
	backupHandler := api.NewBackupHandler(db, backupSvc)
	scheduleHandler := api.NewScheduleHandler(db)

	// Build router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:8080", "http://192.168.1.*:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Post("/auth/login", authHandler.Login)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(api.AuthMiddleware(authSvc))

			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/auth/me", authHandler.Me)

			r.Get("/templates", serverHandler.Templates)

			r.Route("/servers", func(r chi.Router) {
				r.Get("/", serverHandler.List)
				r.Post("/", serverHandler.Create)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", serverHandler.Get)
					r.Put("/", serverHandler.Update)
					r.Delete("/", serverHandler.Delete)
					r.Post("/start", serverHandler.Start)
					r.Post("/stop", serverHandler.Stop)
					r.Post("/restart", serverHandler.Restart)

					// Stats
					r.Get("/stats", statsHandler.Latest)
					r.Get("/stats/history", statsHandler.History)

					// Backups
					r.Get("/backups", backupHandler.List)
					r.Post("/backups", backupHandler.Create)
					r.Get("/backups/{backupId}/download", backupHandler.Download)
					r.Delete("/backups/{backupId}", backupHandler.Delete)
					r.Post("/backups/{backupId}/restore", backupHandler.Restore)

					// Schedules
					r.Get("/schedules", scheduleHandler.List)
					r.Post("/schedules", scheduleHandler.Create)
					r.Put("/schedules/{scheduleId}", scheduleHandler.Update)
					r.Delete("/schedules/{scheduleId}", scheduleHandler.Delete)
				})
			})
		})

		// WebSocket routes (auth via query param)
		r.Get("/servers/{id}/console", consoleHandler.Handle)
		r.Get("/servers/{id}/stats/live", statsHandler.Live)
	})

	// Serve frontend static files from web/dist if it exists
	if distDir := "web/dist"; dirExists(distDir) {
		fileServer := http.FileServer(http.Dir(distDir))
		r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the static file; fall back to index.html for SPA routing
			path := distDir + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, distDir+"/index.html")
				return
			}
			fileServer.ServeHTTP(w, r)
		}))
		log.Println("Serving frontend from web/dist/")
	}

	return &Server{cfg: cfg, db: db, router: r, collector: collector, scheduler: sched}, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) Stop() {
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.scheduler != nil {
		s.scheduler.Stop()
	}
}

// ServeEmbeddedFrontend adds the embedded frontend static file serving.
// Called from main when the embedded FS is available (production build).
func (s *Server) ServeEmbeddedFrontend(frontendFS fs.FS) {
	fileServer := http.FileServer(http.FS(frontendFS))
	s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file. If it doesn't exist, serve index.html for SPA routing.
		fileServer.ServeHTTP(w, r)
	}))
}
