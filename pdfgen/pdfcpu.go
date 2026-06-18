//go:build !fpdf

package pdfgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
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
	pages       []page
	sourceNames []string
	seenSrc     map[string]bool
	stretch     bool
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

func newImpl(opts Options) Generator {
	return &impl{pages: []page{{}}, seenSrc: map[string]bool{}, stretch: opts.Stretch}
}

func (g *impl) AddImage(path, sourceName string, x, y, w, h float64) error {
	if g.stretch {
		return errors.New("stretch is unsupported by the pdfcpu backend; use -tags fpdf or omit -stretch")
	}
	if len(g.pages) == 0 {
		g.NewPage()
	}
	if !g.seenSrc[path] {
		g.seenSrc[path] = true
		g.sourceNames = append(g.sourceNames, filepath.Base(sourceName))
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
	if err := api.Create(nil, &buf, f, conf); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if len(g.sourceNames) == 0 {
		return nil
	}
	return annotateSourceNames(out, g.sourceNames)
}

func mm(v float64) float64 {
	return types.ToUserSpace(v, types.MILLIMETRES)
}

func annotateSourceNames(path string, sourceNames []string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.OPTIMIZE
	ctx, err := api.ReadValidateAndOptimize(f, conf)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	objNrs := make([]int, 0, len(ctx.Optimize.ImageObjects))
	for objNr := range ctx.Optimize.ImageObjects {
		objNrs = append(objNrs, objNr)
	}
	sort.Ints(objNrs)
	for i, objNr := range objNrs {
		if i >= len(sourceNames) {
			break
		}
		ctx.Optimize.ImageObjects[objNr].ImageDict.InsertString("CardsheetSourceFilename", filepath.Base(sourceNames[i]))
	}
	ctx.ResetWriteContext()
	return api.WriteContextFile(ctx, path)
}
