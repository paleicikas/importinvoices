package service

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Service) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.store.DB().QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (s *Service) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.store.DB().ExecContext(ctx, `
		INSERT INTO settings (key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().Unix(), time.Now().Unix())
	return err
}

func (s *Service) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.store.DB().QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, nil
}
