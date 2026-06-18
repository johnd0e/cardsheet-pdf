//go:build fpdf || gopdf || gofpdf || canvas

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func runExtract(args []string) int {
	fmt.Println("unsupported feature: rebuild with the default pdfcpu backend")
	return 1
}

func expandPDFInputs(files []string) ([]string, func(), error) {
	for _, path := range files {
		if strings.EqualFold(filepath.Ext(path), ".pdf") {
			return nil, nil, fmt.Errorf("%s: unsupported feature: rebuild with the default pdfcpu backend", path)
		}
	}
	return files, nil, nil
}
