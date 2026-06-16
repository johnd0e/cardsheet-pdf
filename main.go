package main

import (
	"flag"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"cardsheet-pdf/internal/images"
	"cardsheet-pdf/internal/layout"
	"cardsheet-pdf/internal/version"
	"cardsheet-pdf/pdfgen"
)

var AppVersion = "unknown"

func flagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func depVersionOrUnknown(modulePrefix string) string {
	v := version.FindDepVersion(modulePrefix)
	if v == "" {
		return "unknown"
	}
	return v
}

func main() {
	var gap float64
	var vgap float64
	var dpi int
	var verbose bool
	var sideBySide bool
	var outFile string
	var showVersion bool

	flag.Float64Var(&gap, "gap", -1, "Horizontal gap (mm)")
	flag.Float64Var(&vgap, "vgap", -1, "Vertical gap (mm)")
	flag.IntVar(&dpi, "dpi", 0, "Limit embedded image resolution to this effective DPI")
	flag.BoolVar(&verbose, "verbose", false, "Show image DPI and resize information")
	flag.BoolVar(&sideBySide, "side-by-side", false, "Force side-by-side layout")
	flag.StringVar(&outFile, "out", "output.pdf", "Output file")
	flag.BoolVar(&showVersion, "version", false, "Show version and exit")
	flag.Parse()

	// Populate AppVersion and backend version from build info (if available).
	// Preserve explicit release metadata injected via -ldflags "-X main.AppVersion=...".
	if AppVersion == "unknown" {
		AppVersion = version.TryBuildInfoVersion()
	}
	pdfgen.BackendVersion = depVersionOrUnknown("github.com/go-pdf/fpdf")
	if pdfgen.BackendVersion == "unknown" {
		pdfgen.BackendVersion = depVersionOrUnknown("github.com/pdfcpu/pdfcpu")
	}

	if showVersion {
		fmt.Printf("cardsheet %s\nbackend: %s %s\n", AppVersion, pdfgen.BackendName, pdfgen.BackendVersion)
		return
	}

	files := flag.Args()
	if len(files) == 0 {
		fmt.Println("Usage: cardsheet [options] img1 img2 ...")
		os.Exit(1)
	}
	if dpi < 0 {
		fmt.Fprintln(os.Stderr, "input error: -dpi must be greater than or equal to 0")
		os.Exit(1)
	}

	prepared, cleanup, err := images.Prepare(files, images.Options{
		CardWMM: 85.6,
		CardHMM: 53.98,
		MaxDPI:  dpi,
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "input error:", err)
		os.Exit(1)
	}
	if verbose {
		printImageInfo(prepared, dpi)
	}

	paths := make([]string, len(prepared))
	for i, img := range prepared {
		paths[i] = img.Path
	}

	mode := layout.ModeNormal
	if sideBySide {
		mode = layout.ModeSide
	}

	if !flagPassed("gap") {
		if mode == layout.ModeNormal {
			gap = 10
		} else {
			gap = 5
		}
	}

	vgapPassed := flagPassed("vgap")
	if !vgapPassed {
		if mode == layout.ModeNormal {
			vgap = 5
		} else {
			vgap = 10
		}
	}

	placements := layout.Plan(paths, layout.Options{
		Mode:               mode,
		Gap:                gap,
		VGap:               vgap,
		AlternateNormalGap: mode == layout.ModeNormal && !vgapPassed,
		CardW:              85.6,
		CardH:              53.98,
		PageW:              210.0,
		PageH:              297.0,
		TopMargin:          20.0,
		BottomMargin:       20.0,
	})

	gen := pdfgen.New()
	currentPage := 0
	for _, p := range placements {
		for currentPage < p.Page {
			gen.NewPage()
			currentPage++
		}
		if err := gen.AddImage(p.Path, p.X, p.Y, p.W, p.H); err != nil {
			fmt.Fprintln(os.Stderr, "add image error:", err)
			os.Exit(2)
		}
	}

	if err := gen.Save(outFile); err != nil {
		fmt.Fprintln(os.Stderr, "save error:", err)
		os.Exit(2)
	}

	fmt.Println("Saved:", outFile)
}

func printImageInfo(results []images.Result, maxDPI int) {
	for _, img := range results {
		minDPI := min(img.EffectiveX, img.EffectiveY)
		status := "ok"
		if maxDPI <= 0 && minDPI < 300 {
			status = "below 300 dpi"
		}

		fmt.Printf(
			"%s: %dx%d, effective %.0fx%.0f dpi",
			img.OriginalPath,
			img.Width,
			img.Height,
			img.EffectiveX,
			img.EffectiveY,
		)

		if maxDPI > 0 {
			if img.KeptOriginal {
				fmt.Print(", kept original; resized candidate was larger")
				fmt.Println()
				continue
			}

			saved := img.OriginalSize - img.OutputSize
			savedPct := 0.0
			if img.OriginalSize > 0 {
				savedPct = float64(saved) * 100 / float64(img.OriginalSize)
			}
			fmt.Printf(
				", output %dx%d, %.0fx%.0f dpi, size %s -> %s (%s%.1f%%)",
				img.OutputWidth,
				img.OutputHeight,
				img.OutputX,
				img.OutputY,
				formatBytes(img.OriginalSize),
				formatBytes(img.OutputSize),
				sign(savedPct),
				abs(savedPct),
			)
			if !img.Resized {
				fmt.Print(", unchanged")
			}
		} else {
			fmt.Printf(", %s", status)
		}
		fmt.Println()
	}
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TiB", value/unit)
}

func sign(v float64) string {
	if v > 0 {
		return "-"
	}
	return "+"
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
