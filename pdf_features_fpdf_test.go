//go:build fpdf

package main

import (
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"cardsheet-pdf/pdfgen"

	"codeberg.org/go-pdf/fpdf"
)

func TestFPDFGenerateExtractPreservesSourceNames(t *testing.T) {
	dir := t.TempDir()
	first := writeFPDFTestPNG(t, filepath.Join(dir, "front.png"), color.RGBA{R: 255, A: 255})
	second := writeFPDFTestPNG(t, filepath.Join(dir, "back.png"), color.RGBA{B: 255, A: 255})
	outPDF := filepath.Join(dir, "cards.pdf")

	gen := pdfgen.New()
	if err := gen.AddImage(first, "front.png", 0, 0, 85.6, 53.98); err != nil {
		t.Fatal(err)
	}
	if err := gen.AddImage(second, "back.png", 0, 60, 85.6, 53.98); err != nil {
		t.Fatal(err)
	}
	if err := gen.Save(outPDF); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "extract")
	written, err := extractPDFImagesToDir(outPDF, outDir, conflictRename, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("extracted %d images, want 2: %v", len(written), written)
	}
	for _, name := range []string{"front.png", "back.png"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("missing extracted %s: %v", name, err)
		}
	}
}

func TestFPDFPDFInputAcceptsCardsheetPDF(t *testing.T) {
	dir := t.TempDir()
	img := writeFPDFTestPNG(t, filepath.Join(dir, "front.png"), color.RGBA{G: 255, A: 255})
	outPDF := filepath.Join(dir, "cards.pdf")

	gen := pdfgen.New()
	if err := gen.AddImage(img, "front.png", 0, 0, 85.6, 53.98); err != nil {
		t.Fatal(err)
	}
	if err := gen.Save(outPDF); err != nil {
		t.Fatal(err)
	}

	files, cleanup, err := expandPDFInputs([]string{outPDF})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "front.png" {
		t.Fatalf("expanded files = %v, want one front.png", files)
	}
}

func TestFPDFPDFInputRejectsManifestMismatch(t *testing.T) {
	dir := t.TempDir()
	img := writeFPDFTestPNG(t, filepath.Join(dir, "front.png"), color.RGBA{G: 255, A: 255})
	outPDF := filepath.Join(dir, "cards.pdf")

	gen := pdfgen.New()
	if err := gen.AddImage(img, "front.png", 0, 0, 85.6, 53.98); err != nil {
		t.Fatal(err)
	}
	if err := gen.Save(outPDF); err != nil {
		t.Fatal(err)
	}
	badManifest := `{"marker":"cardsheet","version":1,"images":[{"objectNumber":999,"sourceName":"wrong.png","extension":"png","encodedSha256":"bad"}]}`
	f, err := os.OpenFile(outPDF, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n% cardsheet-manifest " + base64.StdEncoding.EncodeToString([]byte(badManifest)) + "\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = expandPDFInputs([]string{outPDF})
	if err == nil {
		t.Fatal("expected PDF input to reject mismatched cardsheet manifest")
	}
}

func TestFPDFExtractArbitraryJPEGPDFFallbackName(t *testing.T) {
	dir := t.TempDir()
	img := writeFPDFTestJPEG(t, filepath.Join(dir, "foreign.jpg"), color.RGBA{R: 255, G: 200, A: 255})
	pdfPath := filepath.Join(dir, "foreign.pdf")
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.Image(img, 10, 10, 40, 30, false, "", 0, "")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		t.Fatal(err)
	}

	written, err := extractPDFImagesToDir(pdfPath, filepath.Join(dir, "extract"), conflictRename, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 {
		t.Fatalf("extracted %d images, want 1", len(written))
	}
	if got := filepath.Base(written[0]); got != "foreign1.jpg" {
		t.Fatalf("fallback name = %q, want foreign1.jpg", got)
	}
}

func TestFPDFPDFInputRejectsArbitraryPDF(t *testing.T) {
	dir := t.TempDir()
	img := writeFPDFTestJPEG(t, filepath.Join(dir, "foreign.jpg"), color.RGBA{R: 255, A: 255})
	pdfPath := filepath.Join(dir, "foreign.pdf")
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.Image(img, 10, 10, 40, 30, false, "", 0, "")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		t.Fatal(err)
	}

	_, _, err := expandPDFInputs([]string{pdfPath})
	if err == nil {
		t.Fatal("expected arbitrary PDF input rejection")
	}
}

func writeFPDFTestPNG(t *testing.T, path string, c color.Color) string {
	t.Helper()
	img := solidImage(c)
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
	return path
}

func writeFPDFTestJPEG(t *testing.T, path string, c color.Color) string {
	t.Helper()
	img := solidImage(c)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func solidImage(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 16, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}
