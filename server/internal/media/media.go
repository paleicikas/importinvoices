package media

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	asposepdf "github.com/aspose-pdf-foss/aspose-pdf-foss-for-go"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type MediaService struct {
	tempDir string
}

func New(tempDir string) *MediaService {
	return &MediaService{tempDir: tempDir}
}

// ConvertToImages converts a PDF or image file to one or more JPEG images.
// It returns a slice of paths to the generated image files.
// The caller is responsible for deleting these files.
func (s *MediaService) ConvertToImages(ctx context.Context, inputPath string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))

	// Ensure temp directory exists
	if err := os.MkdirAll(s.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Create a unique prefix for this conversion
	baseName := filepath.Base(inputPath)
	prefix := strings.TrimSuffix(baseName, ext)

	if ext == ".pdf" {
		return s.convertPdfToImages(inputPath, prefix)
	}

	// Handle as image
	outputPath := filepath.Join(s.tempDir, fmt.Sprintf("%s-page-0.jpg", prefix))
	if err := s.convertImageToJpeg(inputPath, outputPath); err != nil {
		return nil, fmt.Errorf("image conversion failed: %w", err)
	}

	return []string{outputPath}, nil
}

func (s *MediaService) convertPdfToImages(inputPath, prefix string) ([]string, error) {
	doc, err := asposepdf.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}

	var paths []string
	pageCount := doc.PageCount()
	if pageCount > 10 {
		pageCount = 10 // Limit to 10 pages as in original code
	}

	// Use 150 DPI for good balance between quality and size
	res := asposepdf.NewResolution(150)
	device := asposepdf.NewJpegDevice(res, 90)

	for i := 1; i <= pageCount; i++ {
		page, err := doc.Page(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d: %w", i, err)
		}

		outputPath := filepath.Join(s.tempDir, fmt.Sprintf("%s-page-%d.jpg", prefix, i-1))
		f, err := os.Create(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
		}

		err = device.Process(page, f)
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, fmt.Errorf("failed to render page %d: %w", i, err)
		}
		paths = append(paths, outputPath)
	}

	return paths, nil
}

func (s *MediaService) convertImageToJpeg(inputPath, outputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	return jpeg.Encode(out, img, &jpeg.Options{Quality: 90})
}

func init() {
	// Register image formats
	// png and gif are registered by their imports
	// webp and tiff are registered by their imports from golang.org/x/image
}
