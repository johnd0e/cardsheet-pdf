//go:build !fpdf && !gopdf && !gofpdf && !canvas

package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"cardsheet-pdf/pdfgen"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestRenamedPathAddsNumericSuffix(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "card.png")
	if err := os.WriteFile(existing, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := renamedPath(existing); got != filepath.Join(dir, "card-1.png") {
		t.Fatalf("renamedPath() = %q", got)
	}
}

func TestPDFCPUGenerateExtractPreservesSourceNames(t *testing.T) {
	dir := t.TempDir()
	first := writePNG(t, filepath.Join(dir, "front.png"), color.RGBA{R: 255, A: 255})
	second := writePNG(t, filepath.Join(dir, "back.png"), color.RGBA{B: 255, A: 255})

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

func TestPDFInputRejectsExternalPDFWithoutSourceNames(t *testing.T) {
	dir := t.TempDir()
	img := writePNG(t, filepath.Join(dir, "foreign.png"), color.RGBA{G: 255, A: 255})
	pdf := filepath.Join(dir, "foreign.pdf")
	if err := writeForeignPDF(pdf, img); err != nil {
		t.Fatal(err)
	}

	_, err := extractPDFImagesToDir(pdf, filepath.Join(dir, "extract-input"), conflictRename, true)
	if err == nil {
		t.Fatal("expected PDF input to reject a PDF without cardsheet source metadata")
	}

	written, err := extractPDFImagesToDir(pdf, filepath.Join(dir, "extract-any"), conflictRename, false)
	if err != nil {
		t.Fatalf("extract should accept arbitrary PDFs: %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("extracted %d images, want 1", len(written))
	}
	if got := filepath.Base(written[0]); got != "foreign1.png" {
		t.Fatalf("fallback name = %q, want foreign1.png", got)
	}
}

func writePNG(t *testing.T, path string, c color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, c)
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
	return path
}

func writeForeignPDF(outPDF, imagePath string) error {
	doc := map[string]any{
		"paper":  "A4",
		"origin": "LowerLeft",
		"pages": map[string]any{
			"1": map[string]any{
				"content": map[string]any{
					"image": []map[string]any{
						{
							"src":    imagePath,
							"pos":    []float64{20, 700},
							"width":  120,
							"height": 80,
						},
					},
				},
			},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}
	f, err := os.Create(outPDF)
	if err != nil {
		return err
	}
	defer f.Close()
	api.DisableConfigDir()
	return api.Create(nil, &buf, f, model.NewDefaultConfiguration())
}
