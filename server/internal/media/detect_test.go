package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectExt(t *testing.T) {
	dir := t.TempDir()

	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	pdf := write("doc.pdf", []byte("%PDF-1.4\n"))
	if ext, err := DetectExt(pdf); err != nil || ext != ".pdf" {
		t.Fatalf("pdf: got %q, %v", ext, err)
	}

	jpeg := write("photo.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'})
	if ext, err := DetectExt(jpeg); err != nil || ext != ".jpg" {
		t.Fatalf("jpeg: got %q, %v", ext, err)
	}

	png := write("img.png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	if ext, err := DetectExt(png); err != nil || ext != ".png" {
		t.Fatalf("png: got %q, %v", ext, err)
	}

	txt := write("bad.txt", []byte("not an invoice"))
	if _, err := DetectExt(txt); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
