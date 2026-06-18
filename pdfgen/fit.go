package pdfgen

func fitImageRect(x, y, w, h, imgW, imgH float64) (float64, float64, float64, float64) {
	if imgW <= 0 || imgH <= 0 || w <= 0 || h <= 0 {
		return x, y, w, h
	}
	imgAspect := imgW / imgH
	boxAspect := w / h
	if imgAspect > boxAspect {
		fittedH := w / imgAspect
		return x, y + (h-fittedH)/2, w, fittedH
	}
	fittedW := h * imgAspect
	return x + (w-fittedW)/2, y, fittedW, h
}
