package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionExpired     = errors.New("session expired")
)

type Service struct {
	db *sql.DB
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) EnsureDefaultUser(username, password string) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
	return err
}

func (s *Service) Login(username, password string) (string, error) {
	var id int64
	var hash string
	err := s.db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(7 * 24 * time.Hour)
	_, err = s.db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, id, expires)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) ValidateSession(token string) (*User, error) {
	var user User
	var expiresAt time.Time
	err := s.db.QueryRow(`
		SELECT u.id, u.username, s.expires_at
		FROM sessions s JOIN users u ON s.user_id = u.id
		WHERE s.token = ?
	`, token).Scan(&user.ID, &user.Username, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionExpired
		}
		return nil, err
	}
	if time.Now().After(expiresAt) {
		s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
		return nil, ErrSessionExpired
	}
	return &user, nil
}

func (s *Service) Logout(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
