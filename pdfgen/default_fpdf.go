//go:build fpdf

package pdfgen

import "codeberg.org/go-pdf/fpdf"

func init() {
	BackendName = "fpdf"
	// BackendVersion will be filled from build info in main if available.
}

type impl struct {
	pdf     *fpdf.Fpdf
	stretch bool
}

func newImpl(opts Options) Generator {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	return &impl{pdf: pdf, stretch: opts.Stretch}
}

func (g *impl) AddImage(path, sourceName string, x, y, w, h float64) error {
	info := g.pdf.RegisterImage(path, "")
	if err := g.pdf.Error(); err != nil {
		return err
	}
	if info != nil && !g.stretch {
		x, y, w, h = fitImageRect(x, y, w, h, info.Width(), info.Height())
	}
	// fpdf.Image accepts a file path for raster images.
	g.pdf.Image(path, x, y, w, h, false, "", 0, "")
	return g.pdf.Error()
}

func (g *impl) NewPage() {
	g.pdf.AddPage()
}

func (g *impl) Save(out string) error {
	return g.pdf.OutputFileAndClose(out)
}
