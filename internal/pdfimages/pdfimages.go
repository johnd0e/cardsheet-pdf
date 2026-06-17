package pdfimages

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	ManifestMarker  = "cardsheet"
	ManifestVersion = 1
	manifestPrefix  = "% cardsheet-manifest "
)

type Manifest struct {
	Marker  string          `json:"marker"`
	Version int             `json:"version"`
	Images  []ManifestImage `json:"images"`
}

type ManifestImage struct {
	ObjectNumber  int    `json:"objectNumber"`
	SourceName    string `json:"sourceName"`
	Extension     string `json:"extension"`
	EncodedSHA256 string `json:"encodedSha256"`
	DecodedSHA256 string `json:"decodedSha256,omitempty"`
}

type Image struct {
	ObjectNumber  int
	Generation    int
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
	indirectObjectRe = regexp.MustCompile(`(?s)(\d+)\s+(\d+)\s+obj\b.*?endobj`)
	streamRe         = regexp.MustCompile(`(?s)^(.*?)stream\r?\n(.*?)\r?\nendstream`)
	intRe            = regexp.MustCompile(`/%s\s+(\d+)`)
	nameRe           = regexp.MustCompile(`/%s\s+/([A-Za-z0-9]+)`)
	filterArrayRe    = regexp.MustCompile(`/Filter\s*\[(.*?)\]`)
	filterNameRe     = regexp.MustCompile(`/Filter\s*/([A-Za-z0-9]+)`)
	arrayNameRe      = regexp.MustCompile(`/([A-Za-z0-9]+)`)
	sourceNameRe     = regexp.MustCompile(`/CardsheetSourceFilename\s*(\((?:\\.|[^\\)])*\)|<([0-9A-Fa-f]*)>)`)
	trailerRe        = regexp.MustCompile(`(?s)trailer\s*(<<.*?>>)\s*startxref`)
)

func Read(path string) ([]Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	matches := indirectObjectRe.FindAllSubmatch(data, -1)
	images := make([]Image, 0, len(matches))
	for _, m := range matches {
		stream := streamRe.FindSubmatch(m[0])
		if stream == nil {
			continue
		}
		dict := string(stream[1])
		if !strings.Contains(dict, "/Subtype /Image") {
			continue
		}
		objNr, _ := strconv.Atoi(string(m[1]))
		genNr, _ := strconv.Atoi(string(m[2]))
		img := Image{
			ObjectNumber: objNr,
			Generation:   genNr,
			Width:        parseInt(dict, "Width"),
			Height:       parseInt(dict, "Height"),
			ColorSpace:   parseName(dict, "ColorSpace"),
			Bits:         parseInt(dict, "BitsPerComponent"),
			Filters:      parseFilters(dict),
			DecodeParms:  parseDecodeParms(dict),
			Encoded:      append([]byte(nil), stream[2]...),
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

func WriteXObjectSourceNames(path string, images []Image, sourceNames []string) error {
	if len(images) != len(sourceNames) {
		return fmt.Errorf("cardsheet source-name validation failed: found %d image objects for %d sources", len(images), len(sourceNames))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	objects := parseObjects(data)
	if len(objects) == 0 {
		return fmt.Errorf("cardsheet source-name validation failed: no PDF objects found")
	}
	namesByObject := map[int]string{}
	for i, img := range images {
		namesByObject[img.ObjectNumber] = filepath.Base(sourceNames[i])
	}
	replaced := 0
	for i := range objects {
		name, ok := namesByObject[objects[i].Number]
		if !ok {
			continue
		}
		updated, err := withSourceName(objects[i].Data, name)
		if err != nil {
			return fmt.Errorf("object %d: %w", objects[i].Number, err)
		}
		objects[i].Data = updated
		replaced++
	}
	if replaced != len(sourceNames) {
		return fmt.Errorf("cardsheet source-name validation failed: annotated %d image objects for %d sources", replaced, len(sourceNames))
	}
	trailer := originalTrailer(data)
	if trailer == "" {
		trailer = fmt.Sprintf("<< /Size %d >>", maxObjectNumber(objects)+1)
	} else {
		trailer = replaceTrailerSize(trailer, maxObjectNumber(objects)+1)
	}
	return os.WriteFile(path, rebuildPDF(data, objects, trailer), 0644)
}

func WriteManifest(path string, images []Image, sourceNames []string) error {
	if len(images) != len(sourceNames) {
		return fmt.Errorf("cardsheet manifest validation failed: found %d image objects for %d sources", len(images), len(sourceNames))
	}
	entries := make([]ManifestImage, len(images))
	for i, img := range images {
		entries[i] = ManifestImage{
			ObjectNumber:  img.ObjectNumber,
			SourceName:    filepath.Base(sourceNames[i]),
			Extension:     img.Extension,
			EncodedSHA256: img.EncodedSHA256,
			DecodedSHA256: img.DecodedSHA256,
		}
	}
	manifest := Manifest{
		Marker:  ManifestMarker,
		Version: ManifestVersion,
		Images:  entries,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n%s%s\n", manifestPrefix, base64.StdEncoding.EncodeToString(raw))
	return err
}

func ValidatedSourceNames(path string, images []Image) (map[int]string, bool, error) {
	names := map[int]string{}
	for _, img := range images {
		if img.SourceName == "" {
			return ValidatedNames(path, images)
		}
		names[img.ObjectNumber] = filepath.Base(img.SourceName)
	}
	if len(images) == 0 {
		return nil, false, nil
	}
	return names, true, nil
}

func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	idx := bytes.LastIndex(data, []byte(manifestPrefix))
	if idx < 0 {
		return nil, nil
	}
	line := data[idx+len(manifestPrefix):]
	if nl := bytes.IndexAny(line, "\r\n"); nl >= 0 {
		line = line[:nl]
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(line)))
	if err != nil {
		return nil, fmt.Errorf("invalid cardsheet manifest encoding: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("invalid cardsheet manifest: %w", err)
	}
	if manifest.Marker != ManifestMarker || manifest.Version != ManifestVersion {
		return nil, fmt.Errorf("unsupported cardsheet manifest")
	}
	return &manifest, nil
}

func ValidatedNames(path string, images []Image) (map[int]string, bool, error) {
	manifest, err := ReadManifest(path)
	if err != nil || manifest == nil {
		return nil, false, err
	}
	if len(manifest.Images) != len(images) {
		return nil, false, nil
	}
	byObject := map[int]Image{}
	for _, img := range images {
		byObject[img.ObjectNumber] = img
	}
	names := map[int]string{}
	for _, entry := range manifest.Images {
		img, ok := byObject[entry.ObjectNumber]
		if !ok || img.EncodedSHA256 != entry.EncodedSHA256 {
			return nil, false, nil
		}
		if entry.DecodedSHA256 != "" && img.DecodedSHA256 != entry.DecodedSHA256 {
			return nil, false, nil
		}
		names[entry.ObjectNumber] = filepath.Base(entry.SourceName)
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

type pdfObject struct {
	Number int
	Data   []byte
}

func parseObjects(data []byte) []pdfObject {
	matches := indirectObjectRe.FindAllSubmatch(data, -1)
	objects := make([]pdfObject, 0, len(matches))
	for _, m := range matches {
		objNr, _ := strconv.Atoi(string(m[1]))
		objects = append(objects, pdfObject{
			Number: objNr,
			Data:   append([]byte(nil), m[0]...),
		})
	}
	sort.SliceStable(objects, func(i, j int) bool {
		return objects[i].Number < objects[j].Number
	})
	return objects
}

func withSourceName(obj []byte, name string) ([]byte, error) {
	stream := streamRe.FindSubmatchIndex(obj)
	if stream == nil {
		return nil, fmt.Errorf("image object has no stream")
	}
	dict := string(obj[stream[2]:stream[3]])
	if !strings.Contains(dict, "/Subtype /Image") {
		return nil, fmt.Errorf("object is not an image XObject")
	}
	if loc := sourceNameRe.FindStringIndex(dict); loc != nil {
		dict = dict[:loc[0]] + "/CardsheetSourceFilename " + pdfString(name) + dict[loc[1]:]
	} else {
		idx := strings.LastIndex(dict, ">>")
		if idx < 0 {
			return nil, fmt.Errorf("image dictionary has no closing marker")
		}
		dict = dict[:idx] + "/CardsheetSourceFilename " + pdfString(name) + "\n" + dict[idx:]
	}
	out := make([]byte, 0, len(obj)+len(dict)-stream[3]+stream[2])
	out = append(out, obj[:stream[2]]...)
	out = append(out, dict...)
	out = append(out, obj[stream[3]:]...)
	return out, nil
}

func rebuildPDF(original []byte, objects []pdfObject, trailer string) []byte {
	headerEnd := bytes.IndexByte(original, '\n')
	header := []byte("%PDF-1.7\n")
	if headerEnd >= 0 && bytes.HasPrefix(original, []byte("%PDF-")) {
		header = append([]byte(nil), original[:headerEnd+1]...)
	}
	var out bytes.Buffer
	out.Write(header)
	offsets := map[int]int{}
	for _, obj := range objects {
		offsets[obj.Number] = out.Len()
		out.Write(obj.Data)
		if !bytes.HasSuffix(obj.Data, []byte("\n")) {
			out.WriteByte('\n')
		}
	}
	xrefOffset := out.Len()
	size := maxObjectNumber(objects) + 1
	out.WriteString("xref\n")
	fmt.Fprintf(&out, "0 %d\n", size)
	out.WriteString("0000000000 65535 f \n")
	for i := 1; i < size; i++ {
		if offset, ok := offsets[i]; ok {
			fmt.Fprintf(&out, "%010d 00000 n \n", offset)
		} else {
			out.WriteString("0000000000 65535 f \n")
		}
	}
	out.WriteString("trailer\n")
	out.WriteString(trailer)
	out.WriteString("\nstartxref\n")
	fmt.Fprintf(&out, "%d\n%%%%EOF\n", xrefOffset)
	return out.Bytes()
}

func originalTrailer(data []byte) string {
	m := trailerRe.FindSubmatch(data)
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

func replaceTrailerSize(trailer string, size int) string {
	if regexp.MustCompile(`/Size\s+\d+`).MatchString(trailer) {
		return regexp.MustCompile(`/Size\s+\d+`).ReplaceAllString(trailer, fmt.Sprintf("/Size %d", size))
	}
	idx := strings.LastIndex(trailer, ">>")
	if idx < 0 {
		return fmt.Sprintf("<< /Size %d >>", size)
	}
	return trailer[:idx] + fmt.Sprintf("/Size %d\n", size) + trailer[idx:]
}

func maxObjectNumber(objects []pdfObject) int {
	maxNr := 0
	for _, obj := range objects {
		if obj.Number > maxNr {
			maxNr = obj.Number
		}
	}
	return maxNr
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

func pdfString(s string) string {
	var b strings.Builder
	b.WriteByte('(')
	for _, r := range s {
		switch r {
		case '\\', '(', ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(')')
	return b.String()
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
