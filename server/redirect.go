package server

import (
	"net/http"
)

type HttpsRedirectHandler struct{}

func (h *HttpsRedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.URL.Path
	if len(r.URL.RawQuery) > 0 {
		target += ("?" + r.URL.RawQuery)
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func NewHttpsRedirectHandler() *HttpsRedirectHandler {
	return &HttpsRedirectHandler{}
}
