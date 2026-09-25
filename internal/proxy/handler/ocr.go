package handler

import "net/http"

// OCRProcess handles POST /v1/ocr — forwards to the resolved provider's OCR endpoint.
func (h *Handlers) OCRProcess(w http.ResponseWriter, r *http.Request) {
	h.proxyPassthrough(w, r, "ocr")
}
