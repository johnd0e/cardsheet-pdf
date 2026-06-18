//go:build !fpdf

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

const sourceFilenameKey = "CardsheetSourceFilename"

func runExtract(args []string) int {
	pdfPath, outDir, mode, ok := parseExtractArgs(args)
	if !ok {
		return 1
	}
	if err := extractPDFImages(pdfPath, outDir, mode); err != nil {
		fmt.Fprintln(os.Stderr, "extract error:", err)
		return 1
	}
	return 0
}

func expandPDFInputs(files []string) ([]string, func(), error) {
	out := make([]string, 0, len(files))
	tempDir := ""
	cleanup := func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}

	for _, path := range files {
		if !strings.EqualFold(filepath.Ext(path), ".pdf") {
			out = append(out, path)
			continue
		}
		if tempDir == "" {
			var err error
			tempDir, err = os.MkdirTemp("", "cardsheet-pdf-input-*")
			if err != nil {
				return nil, nil, err
			}
		}
		extracted, err := extractPDFImagesToDir(path, tempDir, conflictRename, true)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		out = append(out, extracted...)
	}

	if tempDir == "" {
		return out, nil, nil
	}
	return out, cleanup, nil
}

func extractPDFImages(pdfPath, outDir string, mode conflictMode) error {
	_, err := extractPDFImagesToDir(pdfPath, outDir, mode, false)
	return err
}

func extractPDFImagesToDir(pdfPath, outDir string, mode conflictMode, requireSourceNames bool) ([]string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, err
	}
	f, err := os.Open(pdfPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.EXTRACTIMAGES
	ctx, err := api.ReadValidateAndOptimize(f, conf)
	if err != nil {
		return nil, err
	}

	written := []string{}
	base := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	reader := newStdinReader()
	fallback := 1
	for page := 1; page <= ctx.PageCount; page++ {
		images, err := pdfcpu.ExtractPageImages(ctx, page, false)
		if err != nil {
			return nil, err
		}
		objNrs := make([]int, 0, len(images))
		for objNr := range images {
			objNrs = append(objNrs, objNr)
		}
		sort.Ints(objNrs)
		for _, objNr := range objNrs {
			img := images[objNr]
			name := sourceName(ctx, objNr)
			if name == "" {
				if requireSourceNames {
					return nil, fmt.Errorf("%s: PDF input must be a PDF previously created by cardsheet", pdfPath)
				}
				name = fmt.Sprintf("%s%d.%s", base, fallback, extension(img.FileType))
			}
			fallback++
			target, ok, err := resolveConflict(filepath.Join(outDir, filepath.Base(name)), mode, reader)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if err := writeExtractedImage(target, img.Reader); err != nil {
				return nil, err
			}
			written = append(written, target)
		}
	}
	return written, nil
}

func sourceName(ctx *model.Context, objNr int) string {
	imageObj := ctx.Optimize.ImageObjects[objNr]
	if imageObj == nil || imageObj.ImageDict == nil {
		return ""
	}
	obj, ok := imageObj.ImageDict.Find(sourceFilenameKey)
	if !ok {
		return ""
	}
	switch v := obj.(type) {
	case types.StringLiteral:
		return filepath.Base(v.Value())
	case types.HexLiteral:
		return filepath.Base(v.Value())
	default:
		return ""
	}
}

func extension(fileType string) string {
	fileType = strings.TrimPrefix(strings.ToLower(fileType), ".")
	if fileType == "jpeg" {
		return "jpg"
	}
	if fileType == "" {
		return "img"
	}
	return fileType
}
