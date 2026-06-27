package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/media"
)

func (s *Service) ImportInvoice(ctx context.Context, userID, orgID, filename string, r io.Reader) (*domain.Invoice, error) {
	// 1. Calculate checksum and save to temp buffer
	hash := sha256.New()
	tempFile, err := os.CreateTemp("", "upload-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()
	defer func() { _ = tempFile.Close() }()

	if _, err := io.Copy(io.MultiWriter(hash, tempFile), r); err != nil {
		return nil, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	detectedExt, err := media.DetectExt(tempFile.Name())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	nameExt := strings.ToLower(filepath.Ext(filename))
	if nameExt == ".jpeg" {
		nameExt = ".jpg"
	}
	if nameExt != "" && nameExt != detectedExt && !(nameExt == ".tif" && detectedExt == ".tiff") {
		return nil, fmt.Errorf("%s: file content is %s but filename extension is %s", filename, detectedExt, nameExt)
	}

	// 2. Check for duplicates
	var existingID string
	var duplicateOfID *string
	status := "pending"
	err = s.store.DB().QueryRowContext(ctx, "SELECT id FROM invoices WHERE checksum = ? AND org_id = ?", checksum, orgID).Scan(&existingID)
	if err == nil {
		status = "duplicate"
		duplicateOfID = &existingID
	}

	// 3. Save to storage
	ext := detectedExt
	storageName := fmt.Sprintf("%s/%s%s", userID, checksum, ext)
	if _, err := tempFile.Seek(0, 0); err != nil {
		return nil, err
	}
	storagePath, err := s.storage.Save(storageName, tempFile)
	if err != nil {
		return nil, err
	}
	// Store relative path in DB
	relStoragePath := s.storage.RelativePath(storagePath)

	// 4. Create invoice record
	inv := &domain.Invoice{
		ID:          uuid.New().String(),
		UserID:      userID,
		OrgID:       orgID,
		Status:      status,
		Filename:    filename,
		Checksum:    checksum,
		StoragePath: relStoragePath,
		DuplicateOfID: duplicateOfID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 4.5 Generate preview if media service is available
	if s.media != nil {
		imagePaths, err := s.media.ConvertToImages(ctx, storagePath)
		if err == nil && len(imagePaths) > 0 {
			// Use the first page as preview
			previewPath := imagePaths[0]
			
			// Move preview to storage
			previewName := fmt.Sprintf("%s/%s-preview.jpg", userID, checksum)
			f, err := os.Open(previewPath)
			if err == nil {
				defer func() { _ = f.Close() }()
				finalPreviewPath, err := s.storage.Save(previewName, f)
				if err == nil {
					relPreviewPath := s.storage.RelativePath(finalPreviewPath)
					inv.PreviewPath = &relPreviewPath
				}
			}
			
			// Cleanup temp images
			for _, p := range imagePaths {
				_ = os.Remove(p)
			}
		}
	}

	_, err = s.store.DB().ExecContext(ctx, `
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, preview_path, duplicate_of_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.UserID, inv.OrgID, inv.Status, inv.Filename, inv.Checksum, inv.StoragePath, inv.PreviewPath, inv.DuplicateOfID, inv.CreatedAt.Unix(), inv.UpdatedAt.Unix())
	if err != nil {
		return nil, err
	}

	// 5. Queue for processing (only if not duplicate)
	if status == "pending" && s.worker != nil {
		s.worker.Queue(inv.ID)
	}

	return inv, nil
}
