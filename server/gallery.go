package server

import (
	"encoding/json"
	"github.com/postverta/pv_backend/model"
	"log"
	"net/http"
	"strconv"
)

func HandleGalleryAppsGet(w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	queries := r.URL.Query()
	limitSlice, found := queries["limit"]
	if !found || len(limitSlice) == 0 {
		log.Println("[WARNING] Cannot find limit in query")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	limit, err := strconv.Atoi(limitSlice[0])
	if err != nil {
		log.Println("[WARNING] Cannot convert limit to number:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	apps, err := model.C().GetGalleryApps(limit)
	if err != nil {
		log.Println("[ERROR] Cannot get apps in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	output := make([]map[string]interface{}, 0)
	for _, app := range apps {
		output = append(output, app.ToJsonMap())
	}

	buf, _ := json.Marshal(output)
	w.WriteHeader(http.StatusOK)
	w.Write(buf)
}
