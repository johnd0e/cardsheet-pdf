package layout

import "testing"

func closeEnough(a, b float64) bool {
	const epsilon = 0.000001
	if a > b {
		return a-b < epsilon
	}
	return b-a < epsilon
}

func baseOptions() Options {
	return Options{
		Mode:         ModeNormal,
		Gap:          10,
		VGap:         5,
		CardW:        85.6,
		CardH:        53.98,
		PageW:        210,
		PageH:        297,
		TopMargin:    20,
		BottomMargin: 20,
	}
}

func TestNormalLayoutKeepsExistingGridCentering(t *testing.T) {
	files := []string{"a.jpg"}
	got := Plan(files, baseOptions())

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	// Existing behavior: center the two-column grid, not a single partial column.
	if !closeEnough(got[0].X, 14.4) {
		t.Fatalf("X = %.2f, want 14.40", got[0].X)
	}
	if !closeEnough(got[0].Y, 20) {
		t.Fatalf("Y = %.2f, want 20.00", got[0].Y)
	}
}

func TestNormalLayoutUsesExplicitVGap(t *testing.T) {
	opts := baseOptions()
	opts.VGap = 12
	opts.AlternateNormalGap = false

	got := Plan([]string{"a.jpg", "b.jpg"}, opts)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	wantY := 20 + opts.CardH + 12
	if !closeEnough(got[1].Y, wantY) {
		t.Fatalf("Y = %.2f, want %.2f", got[1].Y, wantY)
	}
}

func TestNormalLayoutAlternatesDefaultVGap(t *testing.T) {
	opts := baseOptions()
	opts.AlternateNormalGap = true

	got := Plan([]string{"a.jpg", "b.jpg", "c.jpg"}, opts)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}

	wantThirdY := 20 + opts.CardH + 5 + opts.CardH + 10
	if !closeEnough(got[2].Y, wantThirdY) {
		t.Fatalf("Y = %.2f, want %.2f", got[2].Y, wantThirdY)
	}
}

func TestNormalLayoutMovesToNextColumn(t *testing.T) {
	opts := baseOptions()
	opts.AlternateNormalGap = true

	got := Plan([]string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"}, opts)
	if len(got) != 5 {
		t.Fatalf("len(got) = %d, want 5", len(got))
	}

	if got[4].Page != 0 {
		t.Fatalf("Page = %d, want 0", got[4].Page)
	}
	if !closeEnough(got[4].X, 110) {
		t.Fatalf("X = %.2f, want 110.00", got[4].X)
	}
	if !closeEnough(got[4].Y, 20) {
		t.Fatalf("Y = %.2f, want 20.00", got[4].Y)
	}
}

func TestSideLayoutStartsNewPage(t *testing.T) {
	opts := baseOptions()
	opts.Mode = ModeSide
	opts.Gap = 5
	opts.VGap = 10

	got := Plan([]string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg", "f.jpg", "g.jpg", "h.jpg", "i.jpg"}, opts)
	if len(got) != 9 {
		t.Fatalf("len(got) = %d, want 9", len(got))
	}

	if got[8].Page != 1 {
		t.Fatalf("Page = %d, want 1", got[8].Page)
	}
	if !closeEnough(got[8].Y, 20) {
		t.Fatalf("Y = %.2f, want 20.00", got[8].Y)
	}
}
