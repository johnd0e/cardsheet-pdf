//go:build fpdf

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func runExtract(args []string) int {
	fmt.Println("unsupported feature: rebuild without -tags fpdf")
	return 1
}

func expandPDFInputs(files []string) ([]string, func(), error) {
	for _, path := range files {
		if strings.EqualFold(filepath.Ext(path), ".pdf") {
			return nil, nil, fmt.Errorf("%s: unsupported feature: rebuild without -tags fpdf", path)
		}
	}
	return files, nil, nil
}
