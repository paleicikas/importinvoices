package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const MinPasswordLength = 8

var ErrPasswordTooShort = errors.New("password must be at least 8 characters")

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

func (s *Service) CreateUser(ctx context.Context, email, password, name string) (*domain.User, error) {
	return s.insertUser(ctx, s.store.DB(), email, password, name)
}

func (s *Service) insertUser(ctx context.Context, exec dbExecutor, email, password, name string) (*domain.User, error) {
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hash),
		Name:         name,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = exec.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.PasswordHash, user.Name, user.CreatedAt.Unix(), user.UpdatedAt.Unix())
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*domain.User, error) {
	var user domain.User
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users WHERE email = ?`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid email or password")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)
	return &user, nil
}

func (s *Service) CreateOrganization(ctx context.Context, title string) (*domain.Organization, error) {
	return s.insertOrganization(ctx, s.store.DB(), title)
}

func (s *Service) insertOrganization(ctx context.Context, exec dbExecutor, title string) (*domain.Organization, error) {
	org := &domain.Organization{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := exec.ExecContext(ctx, `
		INSERT INTO organizations (id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?)`,
		org.ID, org.Title, org.CreatedAt.Unix(), org.UpdatedAt.Unix())
	if err != nil {
		return nil, err
	}

	return org, nil
}

func (s *Service) CreateSession(ctx context.Context, userID string) (*domain.Session, error) {
	if err := s.CleanupExpiredSessions(ctx); err != nil {
		return nil, err
	}

	session := &domain.Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		Token:     uuid.New().String(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	_, err := s.store.DB().ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.Token, session.ExpiresAt.Unix(), session.CreatedAt.Unix())
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Service) GetUserBySessionToken(ctx context.Context, token string) (*domain.User, error) {
	var user domain.User
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.created_at, u.updated_at
		FROM users u
		JOIN sessions s ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > ?`,
		token, time.Now().Unix()).Scan(&user.ID, &user.Email, &user.Name, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)
	return &user, nil
}

func (s *Service) GetUser(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id, email, name, webhook_urls, created_at, updated_at
		FROM users WHERE id = ?`, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.WebhookUrls, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)
	return &user, nil
}

func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM invoices WHERE user_id = ?", userID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Service) GetOrganization(ctx context.Context) (*domain.Organization, error) {
	var org domain.Organization
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id, title, created_at, updated_at
		FROM organizations 
		WHERE id != '00000000-0000-0000-0000-000000000000'
		LIMIT 1`).Scan(&org.ID, &org.Title, &createdAt, &updatedAt)
	if err != nil {
		// Fallback to any organization if no non-system one found
		err = s.store.DB().QueryRowContext(ctx, `
			SELECT id, title, created_at, updated_at
			FROM organizations LIMIT 1`).Scan(&org.ID, &org.Title, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
	}

	org.CreatedAt = time.Unix(createdAt, 0)
	org.UpdatedAt = time.Unix(updatedAt, 0)
	return &org, nil
}

func (s *Service) DeleteSession(ctx context.Context, token string) error {
	_, err := s.store.DB().ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Service) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.store.DB().ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Service) CleanupExpiredSessions(ctx context.Context) error {
	_, err := s.store.DB().ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	return err
}

func (s *Service) UpdateUserWebhooks(ctx context.Context, userID string, urls map[string]string) error {
	raw, err := json.Marshal(urls)
	if err != nil {
		return err
	}
	str := string(raw)
	_, err = s.store.DB().ExecContext(ctx, `
		UPDATE users SET webhook_urls = ?, updated_at = ?
		WHERE id = ?`, str, time.Now().Unix(), userID)
	return err
}

func WebhookURLForEvent(raw *string, eventType string) string {
	if raw == nil || *raw == "" {
		return ""
	}
	var urls map[string]string
	if json.Unmarshal([]byte(*raw), &urls) != nil {
		return ""
	}
	return urls[eventType]
}

func (s *Service) UpdateUser(ctx context.Context, userID, name, email string) error {
	_, err := s.store.DB().ExecContext(ctx, `
		UPDATE users SET name = ?, email = ?, updated_at = ?
		WHERE id = ?`, name, email, time.Now().Unix(), userID)
	return err
}

func (s *Service) UpdatePassword(ctx context.Context, userID, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ?
		WHERE id = ?`, string(hash), time.Now().Unix(), userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Service) UpdateOrganization(ctx context.Context, title string) error {
	_, err := s.store.DB().ExecContext(ctx, `
		UPDATE organizations SET title = ?, updated_at = ?`, title, time.Now().Unix())
	return err
}

type dbExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
