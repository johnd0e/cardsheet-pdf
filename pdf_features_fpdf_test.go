//go:build fpdf

package main

import "testing"

func TestFPDFExtractUnsupported(t *testing.T) {
	if code := runExtract([]string{"input.pdf"}); code == 0 {
		t.Fatal("runExtract() succeeded, want unsupported feature failure")
	}
}

func TestFPDFPDFInputUnsupported(t *testing.T) {
	_, _, err := expandPDFInputs([]string{"input.pdf"})
	if err == nil {
		t.Fatal("expected unsupported PDF input error")
	}
}
