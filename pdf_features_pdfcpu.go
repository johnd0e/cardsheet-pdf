//go:build !fpdf && !gopdf && !gofpdf && !canvas

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
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

type conflictMode int

const (
	conflictAsk conflictMode = iota
	conflictOverwrite
	conflictRename
)

func runExtract(args []string) int {
	fs := flagSet("cardsheet extract")
	outDir := fs.String("out-dir", ".", "Output directory")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files")
	rename := fs.Bool("rename", false, "Rename on conflicts")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *overwrite && *rename {
		fmt.Fprintln(os.Stderr, "input error: --overwrite and --rename are mutually exclusive")
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: cardsheet extract [--out-dir DIR] [--overwrite | --rename] input.pdf")
		return 1
	}
	mode := conflictAsk
	if *overwrite {
		mode = conflictOverwrite
	} else if *rename {
		mode = conflictRename
	}
	if err := extractPDFImages(fs.Arg(0), *outDir, mode); err != nil {
		fmt.Fprintln(os.Stderr, "extract error:", err)
		return 1
	}
	return 0
}

func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
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
	reader := bufio.NewReader(os.Stdin)
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

func writeExtractedImage(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func resolveConflict(path string, mode conflictMode, reader *bufio.Reader) (string, bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, true, nil
	}
	if err != nil {
		return "", false, err
	}
	switch mode {
	case conflictOverwrite:
		return path, true, nil
	case conflictRename:
		return renamedPath(path), true, nil
	}
	if !isInteractive() {
		return "", false, fmt.Errorf("%s exists; use --overwrite or --rename", path)
	}
	fmt.Printf("%s exists (%s, modified %s). overwrite/rename/skip? [o/r/s]: ",
		path, formatBytes(info.Size()), info.ModTime().Format("2006-01-02 15:04:05"))
	answer, err := reader.ReadString('\n')
	if err != nil {
		return "", false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "o", "overwrite":
		return path, true, nil
	case "r", "rename":
		return renamedPath(path), true, nil
	default:
		return path, false, nil
	}
}

func renamedPath(path string) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
