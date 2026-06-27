package media

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// DetectExt inspects file content (magic bytes) and returns a normalized extension.
// Supported: .pdf, .jpg, .jpeg, .png, .gif, .webp, .tif, .tiff
func DetectExt(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 512)
	n, err := io.ReadFull(f, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	if n == 0 {
		return "", fmt.Errorf("file is empty")
	}
	header = header[:n]

	switch {
	case bytes.HasPrefix(header, []byte("%PDF")):
		return ".pdf", nil
	case bytes.HasPrefix(header, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg", nil
	case bytes.HasPrefix(header, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return ".png", nil
	case bytes.HasPrefix(header, []byte("GIF87a")), bytes.HasPrefix(header, []byte("GIF89a")):
		return ".gif", nil
	case len(header) >= 12 && bytes.HasPrefix(header, []byte("RIFF")) && bytes.Equal(header[8:12], []byte("WEBP")):
		return ".webp", nil
	case bytes.HasPrefix(header, []byte{0x49, 0x49, 0x2A, 0x00}), bytes.HasPrefix(header, []byte{0x4D, 0x4D, 0x00, 0x2A}):
		return ".tiff", nil
	default:
		return "", fmt.Errorf("unsupported file format")
	}
}
