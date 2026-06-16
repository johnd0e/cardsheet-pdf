//go:build pdfcpu

package pdfgen

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

const a4HeightMM = 297.0

func init() {
	BackendName = "pdfcpu"
}

type impl struct {
	pages []page
}

type page struct {
	images []imageBox
}

type imageBox struct {
	Src    string     `json:"src"`
	Pos    [2]float64 `json:"pos"`
	Width  float64    `json:"width"`
	Height float64    `json:"height"`
}

type createDoc struct {
	Paper  string                `json:"paper"`
	Origin string                `json:"origin"`
	Pages  map[string]createPage `json:"pages"`
}

type createPage struct {
	Content createContent `json:"content"`
}

type createContent struct {
	Images []imageBox `json:"image"`
}

func newImpl() Generator {
	return &impl{pages: []page{{}}}
}

func (g *impl) AddImage(path string, x, y, w, h float64) error {
	if len(g.pages) == 0 {
		g.NewPage()
	}

	g.pages[len(g.pages)-1].images = append(g.pages[len(g.pages)-1].images, imageBox{
		Src:    path,
		Pos:    [2]float64{mm(x), mm(a4HeightMM - y - h)},
		Width:  mm(w),
		Height: mm(h),
	})
	return nil
}

func (g *impl) NewPage() {
	g.pages = append(g.pages, page{})
}

func (g *impl) Save(out string) error {
	doc := createDoc{
		Paper:  "A4",
		Origin: "LowerLeft",
		Pages:  make(map[string]createPage, len(g.pages)),
	}

	for i, p := range g.pages {
		doc.Pages[strconv.Itoa(i+1)] = createPage{
			Content: createContent{
				Images: p.images,
			},
		}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	api.DisableConfigDir()
	conf := model.NewDefaultConfiguration()
	return api.Create(nil, &buf, f, conf)
}

func mm(v float64) float64 {
	return types.ToUserSpace(v, types.MILLIMETRES)
}
