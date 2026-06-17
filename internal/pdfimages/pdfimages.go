package pdfimages

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Image struct {
	ObjectNumber  int
	Width         int
	Height        int
	ColorSpace    string
	Bits          int
	Filters       []string
	DecodeParms   map[string]int
	Encoded       []byte
	Decoded       []byte
	EncodedSHA256 string
	DecodedSHA256 string
	Extension     string
	SourceName    string
}

var (
	objectStreamRe = regexp.MustCompile(`(?s)(\d+)\s+\d+\s+obj(.*?)stream\r?\n(.*?)\r?\nendstream`)
	intRe          = regexp.MustCompile(`/%s\s+(\d+)`)
	nameRe         = regexp.MustCompile(`/%s\s+/([A-Za-z0-9]+)`)
	filterArrayRe  = regexp.MustCompile(`/Filter\s*\[(.*?)\]`)
	filterNameRe   = regexp.MustCompile(`/Filter\s*/([A-Za-z0-9]+)`)
	arrayNameRe    = regexp.MustCompile(`/([A-Za-z0-9]+)`)
	sourceNameRe   = regexp.MustCompile(`/CardsheetSourceFilename\s*(\((?:\\.|[^\\)])*\)|<([0-9A-Fa-f]*)>)`)
)

func Read(path string) ([]Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	matches := objectStreamRe.FindAllSubmatch(data, -1)
	images := make([]Image, 0, len(matches))
	for _, m := range matches {
		dict := string(m[2])
		if !strings.Contains(dict, "/Subtype /Image") {
			continue
		}
		objNr, _ := strconv.Atoi(string(m[1]))
		img := Image{
			ObjectNumber: objNr,
			Width:        parseInt(dict, "Width"),
			Height:       parseInt(dict, "Height"),
			ColorSpace:   parseName(dict, "ColorSpace"),
			Bits:         parseInt(dict, "BitsPerComponent"),
			Filters:      parseFilters(dict),
			DecodeParms:  parseDecodeParms(dict),
			Encoded:      append([]byte(nil), m[3]...),
			SourceName:   parseSourceName(dict),
		}
		img.EncodedSHA256 = sum(img.Encoded)
		if err := decode(&img); err != nil {
			return nil, err
		}
		if len(img.Decoded) > 0 {
			img.DecodedSHA256 = sum(img.Decoded)
		}
		images = append(images, img)
	}
	return images, nil
}

func ValidatedSourceNames(path string, images []Image) (map[int]string, bool, error) {
	names := map[int]string{}
	for _, img := range images {
		if img.SourceName == "" {
			return nil, false, nil
		}
		names[img.ObjectNumber] = filepath.Base(img.SourceName)
	}
	if len(images) == 0 {
		return nil, false, nil
	}
	return names, true, nil
}

func EncodePNG(img Image) ([]byte, error) {
	decoded, err := flateDecoded(img)
	if err != nil {
		return nil, err
	}
	pixels, err := undoPredictor(decoded, img)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	switch img.ColorSpace {
	case "DeviceGray":
		gray := image.NewGray(image.Rect(0, 0, img.Width, img.Height))
		copy(gray.Pix, pixels)
		if err := png.Encode(&out, gray); err != nil {
			return nil, err
		}
	case "DeviceRGB":
		rgba := image.NewRGBA(image.Rect(0, 0, img.Width, img.Height))
		p := 0
		for y := 0; y < img.Height; y++ {
			for x := 0; x < img.Width; x++ {
				rgba.SetRGBA(x, y, color.RGBA{R: pixels[p], G: pixels[p+1], B: pixels[p+2], A: 255})
				p += 3
			}
		}
		if err := png.Encode(&out, rgba); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("object %d: unsupported color space /%s", img.ObjectNumber, img.ColorSpace)
	}
	return out.Bytes(), nil
}

func decode(img *Image) error {
	if len(img.Filters) == 0 {
		return fmt.Errorf("object %d: image stream has no supported /Filter", img.ObjectNumber)
	}
	if len(img.Filters) == 1 && img.Filters[0] == "DCTDecode" {
		img.Extension = "jpg"
		return nil
	}
	if len(img.Filters) == 1 && img.Filters[0] == "FlateDecode" {
		decoded, err := flateDecoded(*img)
		if err != nil {
			return err
		}
		pixels, err := undoPredictor(decoded, *img)
		if err != nil {
			return err
		}
		img.Decoded = pixels
		img.Extension = "png"
		return nil
	}
	return fmt.Errorf("object %d: unsupported image filter %s", img.ObjectNumber, strings.Join(img.Filters, ","))
}

func flateDecoded(img Image) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(img.Encoded))
	if err != nil {
		return nil, fmt.Errorf("object %d: invalid FlateDecode stream: %w", img.ObjectNumber, err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("object %d: invalid FlateDecode stream: %w", img.ObjectNumber, err)
	}
	return out, nil
}

func undoPredictor(data []byte, img Image) ([]byte, error) {
	if img.Bits != 8 {
		return nil, fmt.Errorf("object %d: unsupported BitsPerComponent %d", img.ObjectNumber, img.Bits)
	}
	colors := 0
	switch img.ColorSpace {
	case "DeviceGray":
		colors = 1
	case "DeviceRGB":
		colors = 3
	default:
		return nil, fmt.Errorf("object %d: unsupported color space /%s", img.ObjectNumber, img.ColorSpace)
	}
	columns := img.DecodeParms["Columns"]
	if columns == 0 {
		columns = img.Width
	}
	rowLen := columns * colors
	predictor := img.DecodeParms["Predictor"]
	if predictor == 0 || predictor == 1 {
		if len(data) != rowLen*img.Height {
			return nil, fmt.Errorf("object %d: decoded image length mismatch", img.ObjectNumber)
		}
		return data, nil
	}
	if predictor < 10 {
		return nil, fmt.Errorf("object %d: unsupported FlateDecode predictor %d", img.ObjectNumber, predictor)
	}
	out := make([]byte, rowLen*img.Height)
	prev := make([]byte, rowLen)
	src := 0
	for y := 0; y < img.Height; y++ {
		if src >= len(data) {
			return nil, fmt.Errorf("object %d: decoded image length mismatch", img.ObjectNumber)
		}
		filter := data[src]
		src++
		if src+rowLen > len(data) {
			return nil, fmt.Errorf("object %d: decoded image length mismatch", img.ObjectNumber)
		}
		row := append([]byte(nil), data[src:src+rowLen]...)
		src += rowLen
		reconstructPNGRow(row, prev, colors, filter)
		copy(out[y*rowLen:(y+1)*rowLen], row)
		copy(prev, row)
	}
	return out, nil
}

func reconstructPNGRow(row, prev []byte, bpp int, filter byte) {
	for i := range row {
		left, up, upLeft := byte(0), prev[i], byte(0)
		if i >= bpp {
			left = row[i-bpp]
			upLeft = prev[i-bpp]
		}
		switch filter {
		case 1:
			row[i] += left
		case 2:
			row[i] += up
		case 3:
			row[i] += byte((int(left) + int(up)) / 2)
		case 4:
			row[i] += paeth(left, up, upLeft)
		}
	}
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa := absInt(p - int(a))
	pb := absInt(p - int(b))
	pc := absInt(p - int(c))
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func parseInt(dict, key string) int {
	re := regexp.MustCompile(fmt.Sprintf(intRe.String(), regexp.QuoteMeta(key)))
	m := re.FindStringSubmatch(dict)
	if len(m) != 2 {
		return 0
	}
	v, _ := strconv.Atoi(m[1])
	return v
}

func parseName(dict, key string) string {
	re := regexp.MustCompile(fmt.Sprintf(nameRe.String(), regexp.QuoteMeta(key)))
	m := re.FindStringSubmatch(dict)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

func parseFilters(dict string) []string {
	if m := filterArrayRe.FindStringSubmatch(dict); len(m) == 2 {
		names := arrayNameRe.FindAllStringSubmatch(m[1], -1)
		out := make([]string, 0, len(names))
		for _, name := range names {
			out = append(out, name[1])
		}
		return out
	}
	if m := filterNameRe.FindStringSubmatch(dict); len(m) == 2 {
		return []string{m[1]}
	}
	return nil
}

func parseDecodeParms(dict string) map[string]int {
	out := map[string]int{}
	idx := strings.Index(dict, "/DecodeParms")
	if idx < 0 {
		return out
	}
	rest := dict[idx:]
	for _, key := range []string{"Predictor", "Colors", "BitsPerComponent", "Columns"} {
		if v := parseInt(rest, key); v != 0 {
			out[key] = v
		}
	}
	return out
}

func parseSourceName(dict string) string {
	m := sourceNameRe.FindStringSubmatch(dict)
	if len(m) == 0 {
		return ""
	}
	if strings.HasPrefix(m[1], "(") {
		return filepath.Base(unescapePDFString(m[1][1 : len(m[1])-1]))
	}
	raw, err := hex.DecodeString(m[2])
	if err != nil {
		return ""
	}
	return filepath.Base(string(raw))
}

func unescapePDFString(s string) string {
	var b strings.Builder
	escaped := false
	for _, r := range s {
		if !escaped {
			if r == '\\' {
				escaped = true
				continue
			}
			b.WriteRune(r)
			continue
		}
		switch r {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		default:
			b.WriteRune(r)
		}
		escaped = false
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
