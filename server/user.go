package server

import (
	"encoding/json"
	"github.com/postverta/pv_backend/model"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func HandleUserGet(w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	userId := vars["id"]
	user, err := model.C().GetUser(userId)
	if err != nil {
		log.Println("[ERROR] Cannot get user:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(user)
	w.Write(buf)
}
