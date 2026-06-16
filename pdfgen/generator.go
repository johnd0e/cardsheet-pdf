package pdfgen

// Generator is the minimal interface used by the CLI.
type Generator interface {
	AddImage(path string, x, y, w, h float64) error
	NewPage()
	Save(out string) error
}

var BackendName = "unknown"
var BackendVersion = "unknown"

// New returns a Generator implementation for the active build (fpdf or pdfcpu).
func New() Generator {
	return newImpl()
}
