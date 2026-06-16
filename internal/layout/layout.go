package layout

const (
	ModeNormal = "normal"
	ModeSide   = "side"
)

type Options struct {
	Mode               string
	Gap                float64
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
	xStart := (opts.PageW - float64(cols)*(opts.CardW+opts.Gap) + opts.Gap) / 2

	page := 0
	col := 0
	row := 0
	placements := make([]Placement, 0, len(files))

	for _, path := range files {
		x := xStart + float64(col)*(opts.CardW+opts.Gap)
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

	maxCols := int((opts.PageW + opts.Gap) / (opts.CardW + opts.Gap))
	if maxCols < 1 {
		maxCols = 1
	}

	xStart := (opts.PageW - float64(maxCols)*(opts.CardW+opts.Gap) + opts.Gap) / 2

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

		x := xStart + float64(col)*(opts.CardW+opts.Gap)
		placements = append(placements, Placement{
			Path: path,
			Page: page,
			X:    x,
			Y:    y,
			W:    opts.CardW,
			H:    opts.CardH,
		})

		placed++
		y += opts.CardH + effectiveVGap
	}

	return placements
}
