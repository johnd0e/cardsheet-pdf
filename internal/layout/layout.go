package layout

const (
	ModeNormal = "normal"
	ModeSide   = "side"
)

type Options struct {
	Mode               string
	Gap                float64
	Gaps               []float64
	VGap               float64
	AlternateNormalGap bool
	CardW              float64
	CardH              float64
	PageW              float64
	PageH              float64
	TopMargin          float64
	BottomMargin       float64
}

type Placement struct {
	Path string
	Page int
	X    float64
	Y    float64
	W    float64
	H    float64
}

func Plan(files []string, opts Options) []Placement {
	if opts.Mode == ModeSide {
		return planSide(files, opts)
	}
	return planNormal(files, opts)
}

func planSide(files []string, opts Options) []Placement {
	const cols = 2

	bottomLimit := opts.PageH - opts.BottomMargin
	xs := columnPositions(opts, cols)

	page := 0
	col := 0
	row := 0
	placements := make([]Placement, 0, len(files))

	for _, path := range files {
		x := xs[col]
		y := opts.TopMargin + float64(row)*(opts.CardH+opts.VGap)

		if y+opts.CardH > bottomLimit {
			page++
			row = 0
			y = opts.TopMargin
		}

		placements = append(placements, Placement{
			Path: path,
			Page: page,
			X:    x,
			Y:    y,
			W:    opts.CardW,
			H:    opts.CardH,
		})

		col++
		if col == cols {
			col = 0
			row++
		}
	}

	return placements
}

func planNormal(files []string, opts Options) []Placement {
	bottomLimit := opts.PageH - opts.BottomMargin

	xs := fittingColumnPositions(opts)
	maxCols := len(xs)

	page := 0
	col := 0
	y := opts.TopMargin
	placed := 0
	placements := make([]Placement, 0, len(files))

	for _, path := range files {
		effectiveVGap := opts.VGap
		if opts.AlternateNormalGap {
			effectiveVGap = 5.0
			if placed%2 == 1 {
				effectiveVGap = 10.0
			}
		}

		if y+opts.CardH > bottomLimit {
			if col+1 < maxCols {
				col++
				y = opts.TopMargin
			} else {
				page++
				col = 0
				y = opts.TopMargin
			}
		}

		placements = append(placements, Placement{
			Path: path,
			Page: page,
			X:    xs[col],
			Y:    y,
			W:    opts.CardW,
			H:    opts.CardH,
		})

		placed++
		y += opts.CardH + effectiveVGap
	}

	return placements
}

func gaps(opts Options) []float64 {
	if len(opts.Gaps) > 0 {
		return opts.Gaps
	}
	return []float64{opts.Gap}
}

func gapAt(values []float64, index int) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[index%len(values)]
}

func fittingColumnPositions(opts Options) []float64 {
	gs := gaps(opts)
	cols := 1
	totalW := opts.CardW
	for {
		nextW := totalW + gapAt(gs, cols-1) + opts.CardW
		if nextW > opts.PageW {
			break
		}
		totalW = nextW
		cols++
	}
	return columnPositions(opts, cols)
}

func columnPositions(opts Options, cols int) []float64 {
	if cols < 1 {
		cols = 1
	}
	gs := gaps(opts)
	totalW := float64(cols) * opts.CardW
	for i := 0; i < cols-1; i++ {
		totalW += gapAt(gs, i)
	}

	x := (opts.PageW - totalW) / 2
	xs := make([]float64, cols)
	for i := range xs {
		xs[i] = x
		if i < cols-1 {
			x += opts.CardW + gapAt(gs, i)
		}
	}
	return xs
}
