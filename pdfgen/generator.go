package pdfgen

// Generator is the minimal interface used by the CLI.
type Generator interface {
	AddImage(path, sourceName string, x, y, w, h float64) error
	NewPage()
	Save(out string) error
}

var BackendName = "unknown"
var BackendVersion = "unknown"

// New returns a Generator implementation for the active build tag.
func New() Generator {
	return newImpl()
}
