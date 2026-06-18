//go:build fpdf

package pdfgen

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestFPDFStretchGeneratesPDF(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "card.png")
	writeTestPNG(t, imagePath)

	outPDF := filepath.Join(dir, "card.pdf")
	gen := New(Options{Stretch: true})
	if err := gen.AddImage(imagePath, "card.png", 0, 0, 85.6, 53.98); err != nil {
		t.Fatal(err)
	}
	if err := gen.Save(outPDF); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outPDF); err != nil {
		t.Fatal(err)
	}
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
