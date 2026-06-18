package main

import (
	"flag"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

func backendModulePath(name string) string {
	switch name {
	case "fpdf":
		return "codeberg.org/go-pdf/fpdf"
	case "gopdf":
		return "github.com/signintech/gopdf"
	case "gofpdf":
		return "github.com/phpdave11/gofpdf"
	case "canvas":
		return "github.com/tdewolff/canvas"
	case "pdfcpu":
		return "github.com/pdfcpu/pdfcpu"
	default:
		return ""
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "extract" {
		os.Exit(runExtract(os.Args[2:]))
	}

	var gaps repeatedFloatFlag
	var vgap float64
	var dpi int
	var verbose bool
	var sideBySide bool
	var outFile string
	var showVersion bool

	flag.Var(&gaps, "gap", "Horizontal gap (mm); repeat to alternate")
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
	if modulePath := backendModulePath(pdfgen.BackendName); modulePath != "" {
		pdfgen.BackendVersion = depVersionOrUnknown(modulePath)
	}

	if showVersion {
		fmt.Printf("cardsheet %s\nbackend: %s %s\n", AppVersion, pdfgen.BackendName, pdfgen.BackendVersion)
		return
	}

	files, err := expandWildcards(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "input error:", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println("Usage: cardsheet [options] img1 img2 ...")
		os.Exit(1)
	}
	if dpi < 0 {
		fmt.Fprintln(os.Stderr, "input error: -dpi must be greater than or equal to 0")
		os.Exit(1)
	}

	files, pdfCleanup, err := expandPDFInputs(files)
	if pdfCleanup != nil {
		defer pdfCleanup()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "input error:", err)
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
			gaps = repeatedFloatFlag{10}
		} else {
			gaps = repeatedFloatFlag{5}
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
		Gap:                gaps[0],
		Gaps:               []float64(gaps),
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
	for i, p := range placements {
		for currentPage < p.Page {
			gen.NewPage()
			currentPage++
		}
		sourceName := filepath.Base(p.Path)
		if i < len(prepared) {
			sourceName = prepared[i].SourceName
		}
		if err := gen.AddImage(p.Path, sourceName, p.X, p.Y, p.W, p.H); err != nil {
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

func expandWildcards(args []string) ([]string, error) {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if !hasWildcard(arg) {
			out = append(out, arg)
			continue
		}
		matches, err := filepath.Glob(arg)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid wildcard pattern", arg)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: wildcard matched no files", arg)
		}
		sort.Strings(matches)
		out = append(out, matches...)
	}
	return out, nil
}

func hasWildcard(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

type repeatedFloatFlag []float64

func (f *repeatedFloatFlag) Set(value string) error {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	*f = append(*f, v)
	return nil
}

func (f *repeatedFloatFlag) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}
	parts := make([]string, len(*f))
	for i, v := range *f {
		parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strings.Join(parts, ",")
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
