//go:build !fpdf

package pdfgen

import (
	"strings"
	"testing"
)

func TestPDFCPUStretchUnsupported(t *testing.T) {
	err := New(Options{Stretch: true}).AddImage("card.png", "card.png", 0, 0, 85.6, 53.98)
	if err == nil {
		t.Fatal("AddImage() succeeded, want unsupported stretch error")
	}
	if !strings.Contains(err.Error(), "stretch is unsupported") {
		t.Fatalf("AddImage() error = %q", err)
	}
}
