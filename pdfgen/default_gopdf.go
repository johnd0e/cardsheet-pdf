//go:build gopdf

package pdfgen

import "github.com/signintech/gopdf"

const mmPerPoint = 25.4 / 72.0

func init() {
	BackendName = "gopdf"
}

type impl struct {
	pdf gopdf.GoPdf
}

func newImpl() Generator {
	g := &impl{}
	g.pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	g.pdf.AddPage()
	return g
}

func (g *impl) AddImage(path, sourceName string, x, y, w, h float64) error {
	cfg, err := imageConfig(path)
	if err != nil {
		return err
	}
	x, y, w, h = fitImageRect(x, y, w, h, float64(cfg.Width), float64(cfg.Height))
	return g.pdf.Image(path, points(x), points(y), &gopdf.Rect{W: points(w), H: points(h)})
}

func (g *impl) NewPage() {
	g.pdf.AddPage()
}

func (g *impl) Save(out string) error {
	return g.pdf.WritePdf(out)
}

func points(mm float64) float64 {
	return mm / mmPerPoint
}
