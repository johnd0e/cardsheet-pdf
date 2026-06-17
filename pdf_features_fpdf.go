//go:build fpdf

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cardsheet-pdf/internal/pdfimages"
)

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
	images, err := pdfimages.Read(pdfPath)
	if err != nil {
		return nil, err
	}
	names, validSourceNames, err := pdfimages.ValidatedSourceNames(pdfPath, images)
	if err != nil {
		return nil, err
	}
	if requireSourceNames && !validSourceNames {
		return nil, fmt.Errorf("%s: PDF input must be a PDF previously created by cardsheet", pdfPath)
	}

	written := []string{}
	base := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	reader := newStdinReader()
	for i, img := range images {
		name := names[img.ObjectNumber]
		if name == "" {
			name = fmt.Sprintf("%s%d.%s", base, i+1, img.Extension)
		}
		target, ok, err := resolveConflict(filepath.Join(outDir, filepath.Base(name)), mode, reader)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if err := writeFPDFExtractedImage(target, img); err != nil {
			return nil, err
		}
		written = append(written, target)
	}
	return written, nil
}

func writeFPDFExtractedImage(path string, img pdfimages.Image) error {
	if img.Extension == "jpg" {
		return writeExtractedImage(path, bytes.NewReader(img.Encoded))
	}
	if img.Extension == "png" {
		data, err := pdfimages.EncodePNG(img)
		if err != nil {
			return err
		}
		return writeExtractedImage(path, bytes.NewReader(data))
	}
	return fmt.Errorf("object %d: unsupported image extension %q", img.ObjectNumber, img.Extension)
}
