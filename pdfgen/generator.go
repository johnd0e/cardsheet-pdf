package pdfgen

// Generator is the minimal interface used by the CLI.
type Generator interface {
	AddImage(path, sourceName string, x, y, w, h float64) error
	NewPage()
	Save(out string) error
}

type Options struct {
	Stretch bool
}

var BackendName = "unknown"
var BackendVersion = "unknown"

// New returns a Generator implementation for the active build (fpdf or pdfcpu).
func New(opts ...Options) Generator {
	options := Options{}
	if len(opts) > 0 {
		options = opts[0]
	}
	return newImpl(options)
}
