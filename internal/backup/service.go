package backup

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type Backup struct {
	ID        string `json:"id"`
	ServerID  string `json:"server_id"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

type Service struct {
	db      *sql.DB
	dataDir string
}

func NewService(db *sql.DB, dataDir string) *Service {
	return &Service{db: db, dataDir: dataDir}
}

// backupsDir returns the path where backups are stored for a server.
func (s *Service) backupsDir(serverID string) string {
	return filepath.Join(s.dataDir, "backups", serverID)
}

// serverDataDir returns the path where server data lives.
func (s *Service) serverDataDir(serverID string) string {
	return filepath.Join(s.dataDir, "servers", serverID)
}

// Create creates a tar.gz backup of a server's data directory.
func (s *Service) Create(serverID string) (*Backup, error) {
	srcDir := s.serverDataDir(serverID)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("server data directory not found: %s", srcDir)
	}

	backupDir := s.backupsDir(serverID)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	id := uuid.New().String()[:8]
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.tar.gz", timestamp, id)
	backupPath := filepath.Join(backupDir, filename)

	if err := createTarGz(backupPath, srcDir); err != nil {
		os.Remove(backupPath)
		return nil, fmt.Errorf("create archive: %w", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("stat backup: %w", err)
	}

	backup := &Backup{
		ID:        id,
		ServerID:  serverID,
		Filename:  filename,
		SizeBytes: info.Size(),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	_, err = s.db.Exec(
		`INSERT INTO backups (id, server_id, filename, size_bytes) VALUES (?, ?, ?, ?)`,
		backup.ID, backup.ServerID, backup.Filename, backup.SizeBytes,
	)
	if err != nil {
		os.Remove(backupPath)
		return nil, fmt.Errorf("save backup record: %w", err)
	}

	return backup, nil
}

// List returns all backups for a server.
func (s *Service) List(serverID string) ([]Backup, error) {
	rows, err := s.db.Query(
		`SELECT id, server_id, filename, size_bytes, created_at FROM backups WHERE server_id = ? ORDER BY created_at DESC`,
		serverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backups []Backup
	for rows.Next() {
		var b Backup
		if err := rows.Scan(&b.ID, &b.ServerID, &b.Filename, &b.SizeBytes, &b.CreatedAt); err != nil {
			continue
		}
		backups = append(backups, b)
	}
	if backups == nil {
		backups = []Backup{}
	}
	return backups, nil
}

// FilePath returns the full path to a backup file.
func (s *Service) FilePath(serverID, backupID string) (string, error) {
	var filename string
	err := s.db.QueryRow(
		`SELECT filename FROM backups WHERE id = ? AND server_id = ?`, backupID, serverID,
	).Scan(&filename)
	if err != nil {
		return "", fmt.Errorf("backup not found: %w", err)
	}
	return filepath.Join(s.backupsDir(serverID), filename), nil
}

// Delete removes a backup file and its database record.
func (s *Service) Delete(serverID, backupID string) error {
	path, err := s.FilePath(serverID, backupID)
	if err != nil {
		return err
	}

	os.Remove(path)
	_, err = s.db.Exec(`DELETE FROM backups WHERE id = ? AND server_id = ?`, backupID, serverID)
	return err
}

// Restore extracts a backup archive into the server's data directory.
// The server should be stopped before calling this.
func (s *Service) Restore(serverID, backupID string) error {
	path, err := s.FilePath(serverID, backupID)
	if err != nil {
		return err
	}

	destDir := s.serverDataDir(serverID)

	// Clear existing data
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("clear data directory: %w", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("recreate data directory: %w", err)
	}

	return extractTarGz(path, destDir)
}

func createTarGz(dest, srcDir string) error {
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get path relative to the source directory
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

func extractTarGz(src, destDir string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		// Prevent path traversal
		if !filepath.HasPrefix(target, destDir) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
