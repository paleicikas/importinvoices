package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/export"
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
	return s.insertUser(ctx, s.store.DB(), email, password, name, domain.RoleOperator)
}

// CreateUserWithRole creates a user with an explicit role. Used by the admin
// user-management UI. role must be domain.RoleAdmin or domain.RoleOperator.
func (s *Service) CreateUserWithRole(ctx context.Context, email, password, name, role string) (*domain.User, error) {
	if role != domain.RoleAdmin && role != domain.RoleOperator {
		return nil, errors.New("invalid role")
	}
	return s.insertUser(ctx, s.store.DB(), email, password, name, role)
}

func (s *Service) insertUser(ctx context.Context, exec dbExecutor, email, password, name, role string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	if role == "" {
		role = domain.RoleOperator
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
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = exec.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role, user.CreatedAt.Unix(), user.UpdatedAt.Unix())
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var user domain.User
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id, email, password_hash, name, role, created_at, updated_at
		FROM users WHERE email = ?`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &createdAt, &updatedAt)
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

// VerifyUserPassword checks the supplied password against the user's stored
// bcrypt hash. Used by the profile password-change flow to require the current
// password before accepting a new one. Returns an error if the user does not
// exist or the password does not match.
func (s *Service) VerifyUserPassword(ctx context.Context, userID, password string) error {
	var hash string
	err := s.store.DB().QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("invalid current password")
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return errors.New("invalid current password")
	}
	return nil
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
		SELECT u.id, u.email, u.name, u.role, u.created_at, u.updated_at
		FROM users u
		JOIN sessions s ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > ?`,
		token, time.Now().Unix()).Scan(&user.ID, &user.Email, &user.Name, &user.Role, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)
	return &user, nil
}

// GetUserByEmail returns the user with the given email address, or
// sql.ErrNoRows if no such user exists. Used by the CLI reset-password
// subcommand to locate an account without going through Authenticate.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var user domain.User
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id, email, name, role, webhook_urls, created_at, updated_at
		FROM users WHERE email = ?`, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.WebhookUrls, &createdAt, &updatedAt)
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
		SELECT id, email, name, role, webhook_urls, created_at, updated_at
		FROM users WHERE id = ?`, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.WebhookUrls, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)
	return &user, nil
}

// DefaultUser returns the first user (by creation order). It is used by
// background entry points that lack an HTTP request context (e.g. the MCP
// server's import_invoice tool) to attribute an action to a concrete user in
// single-tenant deployments. Returns sql.ErrNoRows if no user exists.
func (s *Service) DefaultUser(ctx context.Context) (*domain.User, error) {
	var user domain.User
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id, email, name, role, webhook_urls, created_at, updated_at
		FROM users ORDER BY created_at ASC LIMIT 1`).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.WebhookUrls, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)
	return &user, nil
}

// ListUsers returns all users ordered by creation time.
func (s *Service) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT id, email, name, role, webhook_urls, created_at, updated_at
		FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		var createdAt, updatedAt int64
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.WebhookUrls, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		u.CreatedAt = time.Unix(createdAt, 0)
		u.UpdatedAt = time.Unix(updatedAt, 0)
		users = append(users, u)
	}
	return users, rows.Err()
}

// AdminCount returns the number of admin users.
func (s *Service) AdminCount(ctx context.Context) (int, error) {
	var n int
	err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE role = ?`, domain.RoleAdmin).Scan(&n)
	return n, err
}

// UpdateUserRole changes a user's role. It enforces strict last-admin
// protection: the acting user cannot change their own role, and the last
// remaining admin cannot be demoted. actingUserID is the user requesting the
// change (from reqctx.User).
func (s *Service) UpdateUserRole(ctx context.Context, actingUserID, targetUserID, role string) error {
	if role != domain.RoleAdmin && role != domain.RoleOperator {
		return errors.New("invalid role")
	}
	if actingUserID == targetUserID {
		return errors.New("you cannot change your own role")
	}

	target, err := s.GetUser(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target.Role == domain.RoleAdmin && role == domain.RoleOperator {
		count, err := s.AdminCount(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("you cannot remove the last admin")
		}
	}

	_, err = s.store.DB().ExecContext(ctx, `
		UPDATE users SET role = ?, updated_at = ? WHERE id = ?`,
		role, time.Now().Unix(), targetUserID)
	return err
}

func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM invoices WHERE user_id = ?", userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteUserAs deletes a user. actingUserID is the user requesting the delete
// (from reqctx.User). It enforces strict last-admin protection: a user cannot
// delete themselves, and the last remaining admin cannot be deleted.
func (s *Service) DeleteUserAs(ctx context.Context, actingUserID, userID string) error {
	if actingUserID == userID {
		return errors.New("you cannot delete your own account")
	}

	target, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if target.Role == domain.RoleAdmin {
		count, err := s.AdminCount(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("you cannot remove the last admin")
		}
	}

	return s.DeleteUser(ctx, userID)
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
	// Validate every webhook URL at save time so bad/internal URLs cannot be
	// persisted and later triggered. Empty values are allowed (clears the event).
	for event, u := range urls {
		if u == "" {
			continue
		}
		if err := export.ValidateWebhookURL(u); err != nil {
			return fmt.Errorf("webhook URL for %s: %w", event, err)
		}
	}
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
	email = strings.ToLower(strings.TrimSpace(email))
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
