//go:build fpdf

package pdfgen

import (
	"path/filepath"

	"cardsheet-pdf/internal/pdfimages"

	"codeberg.org/go-pdf/fpdf"
)

func init() {
	BackendName = "fpdf"
	// BackendVersion will be filled from build info in main if available.
}

type impl struct {
	pdf         *fpdf.Fpdf
	sourceNames []string
	seenSrc     map[string]bool
}

func newImpl() Generator {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	return &impl{pdf: pdf, seenSrc: map[string]bool{}}
}

func (g *impl) AddImage(path, sourceName string, x, y, w, h float64) error {
	info := g.pdf.RegisterImage(path, "")
	if err := g.pdf.Error(); err != nil {
		return err
	}
	if !g.seenSrc[path] {
		g.seenSrc[path] = true
		g.sourceNames = append(g.sourceNames, filepath.Base(sourceName))
	}
	if info != nil {
		x, y, w, h = fitImageRect(x, y, w, h, info.Width(), info.Height())
	}
	// fpdf.Image accepts a file path for raster images.
	g.pdf.Image(path, x, y, w, h, false, "", 0, "")
	return g.pdf.Error()
}

func fitImageRect(x, y, w, h, imgW, imgH float64) (float64, float64, float64, float64) {
	if imgW <= 0 || imgH <= 0 || w <= 0 || h <= 0 {
		return x, y, w, h
	}
	imgAspect := imgW / imgH
	boxAspect := w / h
	if imgAspect > boxAspect {
		fittedH := w / imgAspect
		return x, y + (h-fittedH)/2, w, fittedH
	}
	fittedW := h * imgAspect
	return x + (w-fittedW)/2, y, fittedW, h
}

func (g *impl) NewPage() {
	g.pdf.AddPage()
}

func (g *impl) Save(out string) error {
	if err := g.pdf.OutputFileAndClose(out); err != nil {
		return err
	}
	if len(g.sourceNames) == 0 {
		return nil
	}
	images, err := pdfimages.Read(out)
	if err != nil {
		return err
	}
	return pdfimages.WriteManifest(out, images, g.sourceNames)
}
