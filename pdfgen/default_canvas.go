//go:build canvas

package pdfgen

import (
	"os"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/pdf"
)

const (
	a4WidthMMCanvas  = 210.0
	a4HeightMMCanvas = 297.0
)

func init() {
	BackendName = "canvas"
}

type impl struct {
	pages   []*canvas.Canvas
	current *canvas.Canvas
}

func newImpl() Generator {
	g := &impl{}
	g.NewPage()
	return g
}

func (g *impl) AddImage(path, sourceName string, x, y, w, h float64) error {
	img, err := loadImage(path)
	if err != nil {
		return err
	}
	bounds := img.Bounds()
	x, y, w, h = fitImageRect(x, y, w, h, float64(bounds.Dx()), float64(bounds.Dy()))

	ctx := canvas.NewContext(g.current)
	resolution := canvas.DPMM(float64(bounds.Dx()) / w)
	ctx.DrawImage(x, a4HeightMMCanvas-y-h, img, resolution)
	return nil
}

func (g *impl) NewPage() {
	c := canvas.New(a4WidthMMCanvas, a4HeightMMCanvas)
	g.pages = append(g.pages, c)
	g.current = c
}

func (g *impl) Save(out string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	renderer := pdf.New(f, a4WidthMMCanvas, a4HeightMMCanvas, nil)
	for i, page := range g.pages {
		if i > 0 {
			renderer.NewPage(a4WidthMMCanvas, a4HeightMMCanvas)
		}
		page.RenderTo(renderer)
	}
	if err := renderer.Close(); err != nil {
		return err
	}
	return f.Close()
}
