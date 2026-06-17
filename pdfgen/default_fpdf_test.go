//go:build fpdf

package pdfgen

import "testing"

func TestFitImageRectPreservesAspectForWideImage(t *testing.T) {
	x, y, w, h := fitImageRect(10, 20, 100, 50, 200, 50)

	if x != 10 || w != 100 {
		t.Fatalf("x,w = %.2f,%.2f; want 10,100", x, w)
	}
	if y != 32.5 || h != 25 {
		t.Fatalf("y,h = %.2f,%.2f; want 32.5,25", y, h)
	}
}

func TestFitImageRectPreservesAspectForTallImage(t *testing.T) {
	x, y, w, h := fitImageRect(10, 20, 100, 50, 50, 100)

	if y != 20 || h != 50 {
		t.Fatalf("y,h = %.2f,%.2f; want 20,50", y, h)
	}
	if x != 47.5 || w != 25 {
		t.Fatalf("x,w = %.2f,%.2f; want 47.5,25", x, w)
	}
}
