package server

import (
	"net/http"
)

func SetCommonHeaders(w http.ResponseWriter, isJson bool) {
	if isJson {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func SetCommonOptionsHeaders(w http.ResponseWriter, allowedMethods string, isJson bool) {
	SetCommonHeaders(w, isJson)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
}
