package images

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

type Options struct {
	CardWMM float64
	CardHMM float64
	MaxDPI  int
}

type Result struct {
	OriginalPath string
	Path         string
	Format       string
	Width        int
	Height       int
	EffectiveX   float64
	EffectiveY   float64
	OutputWidth  int
	OutputHeight int
	OutputX      float64
	OutputY      float64
	OriginalSize int64
	OutputSize   int64
	Resized      bool
	KeptOriginal bool
	Temp         bool
}

func Prepare(paths []string, opts Options) ([]Result, func(), error) {
	results := make([]Result, 0, len(paths))
	tempFiles := []string{}

	cleanup := func() {
		for _, path := range tempFiles {
			_ = os.Remove(path)
		}
	}

	for _, path := range paths {
		result, err := prepareOne(path, opts)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		if result.Temp {
			tempFiles = append(tempFiles, result.Path)
		}
		results = append(results, result)
	}

	return results, cleanup, nil
}

func prepareOne(path string, opts Options) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return Result{}, fmt.Errorf("%s: unsupported or unreadable image: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}

	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	result := Result{
		OriginalPath: path,
		Path:         path,
		Format:       format,
		Width:        w,
		Height:       h,
		EffectiveX:   effectiveDPI(w, opts.CardWMM),
		EffectiveY:   effectiveDPI(h, opts.CardHMM),
		OutputWidth:  w,
		OutputHeight: h,
		OutputX:      effectiveDPI(w, opts.CardWMM),
		OutputY:      effectiveDPI(h, opts.CardHMM),
		OriginalSize: info.Size(),
		OutputSize:   info.Size(),
	}

	if opts.MaxDPI <= 0 {
		return result, nil
	}

	targetW := int(math.Round(mmToInches(opts.CardWMM) * float64(opts.MaxDPI)))
	targetH := int(math.Round(mmToInches(opts.CardHMM) * float64(opts.MaxDPI)))
	if targetW <= 0 || targetH <= 0 || (w <= targetW && h <= targetH) {
		return result, nil
	}

	scale := math.Min(float64(targetW)/float64(w), float64(targetH)/float64(h))
	newW := max(1, int(math.Round(float64(w)*scale)))
	newH := max(1, int(math.Round(float64(h)*scale)))

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)

	outPath, err := writeTempImage(dst, path, format)
	if err != nil {
		return Result{}, err
	}
	outInfo, err := os.Stat(outPath)
	if err != nil {
		_ = os.Remove(outPath)
		return Result{}, fmt.Errorf("%s: %w", outPath, err)
	}

	if outInfo.Size() >= result.OriginalSize {
		_ = os.Remove(outPath)
		result.KeptOriginal = true
		return result, nil
	}

	result.Path = outPath
	result.OutputWidth = newW
	result.OutputHeight = newH
	result.OutputX = effectiveDPI(newW, opts.CardWMM)
	result.OutputY = effectiveDPI(newH, opts.CardHMM)
	result.OutputSize = outInfo.Size()
	result.Resized = true
	result.Temp = true
	return result, nil
}

func writeTempImage(img image.Image, sourcePath, format string) (string, error) {
	ext := "." + strings.ToLower(format)
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	if ext != ".jpg" && ext != ".png" {
		ext = strings.ToLower(filepath.Ext(sourcePath))
	}
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		ext = ".jpg"
	}

	f, err := os.CreateTemp("", "cardsheet-*"+ext)
	if err != nil {
		return "", err
	}
	path := f.Name()

	var encodeErr error
	switch ext {
	case ".png":
		encodeErr = png.Encode(f, img)
	default:
		encodeErr = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	}

	closeErr := f.Close()
	if encodeErr != nil {
		_ = os.Remove(path)
		return "", encodeErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}
	return path, nil
}

func effectiveDPI(px int, mm float64) float64 {
	return float64(px) / mmToInches(mm)
}

func mmToInches(mm float64) float64 {
	return mm / 25.4
}
