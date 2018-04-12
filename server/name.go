package server

import (
	"encoding/json"
	"github.com/postverta/pv_backend/model"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func HandleNameGet(w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	vars := mux.Vars(r)
	appName := vars["name"]

	app, err := model.C().GetAppByName(appName)
	if err != nil {
		log.Println("[ERROR] Cannot get app in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if app == nil {
		log.Println("[WARNING] Cannot find app")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	// Because this API is public, we only return the ID of the app
	buf, _ := json.Marshal(map[string]string{
		"id": app.Id,
	})
	w.Write(buf)
}
