//go:build gofpdf

package pdfgen

import "github.com/phpdave11/gofpdf"

func init() {
	BackendName = "gofpdf"
}

type impl struct {
	pdf *gofpdf.Fpdf
}

func newImpl() Generator {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	return &impl{pdf: pdf}
}

func (g *impl) AddImage(path, sourceName string, x, y, w, h float64) error {
	info := g.pdf.RegisterImage(path, "")
	if err := g.pdf.Error(); err != nil {
		return err
	}
	if info != nil {
		x, y, w, h = fitImageRect(x, y, w, h, info.Width(), info.Height())
	}
	g.pdf.Image(path, x, y, w, h, false, "", 0, "")
	return g.pdf.Error()
}

func (g *impl) NewPage() {
	g.pdf.AddPage()
}

func (g *impl) Save(out string) error {
	return g.pdf.OutputFileAndClose(out)
}
