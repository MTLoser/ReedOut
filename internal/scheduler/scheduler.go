package scheduler

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/reedfamily/reedout/internal/backup"
	"github.com/reedfamily/reedout/internal/docker"
)

type Schedule struct {
	ID        string `json:"id"`
	ServerID  string `json:"server_id"`
	Name      string `json:"name"`
	CronExpr  string `json:"cron_expr"`
	Action    string `json:"action"` // start, stop, restart, backup
	Enabled   bool   `json:"enabled"`
	LastRun   string `json:"last_run"`
	CreatedAt string `json:"created_at"`
}

type Scheduler struct {
	db     *sql.DB
	docker *docker.Client
	backup *backup.Service
	cancel context.CancelFunc
}

func New(db *sql.DB, dockerClient *docker.Client, backupSvc *backup.Service) *Scheduler {
	return &Scheduler{
		db:     db,
		docker: dockerClient,
		backup: backupSvc,
	}
}

func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		// Check every 60 seconds, aligned to the minute
		for {
			now := time.Now()
			nextMinute := now.Truncate(time.Minute).Add(time.Minute)
			sleepDuration := time.Until(nextMinute)

			select {
			case <-ctx.Done():
				return
			case <-time.After(sleepDuration):
				s.tick(ctx)
			}
		}
	}()

	log.Println("Scheduler started")
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()

	rows, err := s.db.Query(
		`SELECT s.id, s.server_id, s.cron_expr, s.action, srv.container_id
		FROM schedules s
		JOIN servers srv ON s.server_id = srv.id
		WHERE s.enabled = 1`,
	)
	if err != nil {
		log.Printf("scheduler: query: %v", err)
		return
	}
	defer rows.Close()

	type job struct {
		scheduleID  string
		serverID    string
		cronExpr    string
		action      string
		containerID string
	}

	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.scheduleID, &j.serverID, &j.cronExpr, &j.action, &j.containerID); err != nil {
			continue
		}
		jobs = append(jobs, j)
	}

	for _, j := range jobs {
		cron, err := ParseCron(j.cronExpr)
		if err != nil {
			log.Printf("scheduler: invalid cron %q for schedule %s: %v", j.cronExpr, j.scheduleID, err)
			continue
		}

		if !cron.Matches(now) {
			continue
		}

		log.Printf("scheduler: running %s on server %s (schedule %s)", j.action, j.serverID, j.scheduleID)
		s.execute(ctx, j.action, j.serverID, j.containerID)

		// Update last_run
		s.db.Exec("UPDATE schedules SET last_run = ? WHERE id = ?", now, j.scheduleID)
	}
}

func (s *Scheduler) execute(ctx context.Context, action, serverID, containerID string) {
	var err error
	switch action {
	case "start":
		err = s.docker.StartContainer(ctx, containerID)
		if err == nil {
			s.db.Exec("UPDATE servers SET status = 'running', updated_at = ? WHERE id = ?", time.Now(), serverID)
		}
	case "stop":
		err = s.docker.StopContainer(ctx, containerID)
		if err == nil {
			s.db.Exec("UPDATE servers SET status = 'exited', updated_at = ? WHERE id = ?", time.Now(), serverID)
		}
	case "restart":
		err = s.docker.RestartContainer(ctx, containerID)
		if err == nil {
			s.db.Exec("UPDATE servers SET status = 'running', updated_at = ? WHERE id = ?", time.Now(), serverID)
		}
	case "backup":
		_, err = s.backup.Create(serverID)
	default:
		log.Printf("scheduler: unknown action %q", action)
		return
	}

	if err != nil {
		log.Printf("scheduler: %s on %s failed: %v", action, serverID, err)
	}
}
