package main

import (
	"image"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	dir := "server/internal/testdata"
	_ = os.MkdirAll(dir, 0755)

	// 1x1 PNG
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	f, _ := os.Create(filepath.Join(dir, "sample.png"))
	_ = png.Encode(f, img)
	f.Close()

	// 1x1 GIF
	f, _ = os.Create(filepath.Join(dir, "sample.gif"))
	_ = gif.Encode(f, img, nil)
	f.Close()

	// Minimal PDF (just enough to be recognized by some tools, but maybe not Aspose)
	// Actually, for Aspose we might need a real valid PDF.
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF")
	_ = os.WriteFile(filepath.Join(dir, "sample.pdf"), pdfContent, 0644)

	// Invalid PDF
	_ = os.WriteFile(filepath.Join(dir, "invalid.pdf"), []byte("not a pdf"), 0644)
}
