package server

import (
	"github.com/postverta/pv_backend/logmgr"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
)

func HandleInternalAppLogPost(w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, false)
	vars := mux.Vars(r)
	appId := vars["id"]

	msg, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("[WARNING] Cannot read request body, err:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = logmgr.L().WriteLine(appId, string(msg))
	if err != nil {
		log.Println("[ERROR] Cannot write log entry, err:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: if a websocket session is open for the app, send the log to the session as well
	w.WriteHeader(http.StatusOK)
}

/*
func HandleInternalAppBackupGet(w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, false)

	vars := mux.Vars(r)
	appId := vars["id"]
	app, err := model.C().GetApp(appId)

	if err != nil {
		log.Println("[ERROR] Cannot get app in database:", err)
		SetCommonHeaders(w, true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if app == nil {
		log.Println("[WARNING] Cannot find app")
		SetCommonHeaders(w, true)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Use "quick" mode for the context, so the container won't linger around.
	context, closeFunc, err := cluster.C().GetContext(app.Id, app.DiskId, true)
	if err != nil {
		log.Println("[ERROR] Cannot get context for app", app.Id, "err:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer closeFunc()

	req := &execproto.ExecReq{
		TaskName:          "file_backup",
		WaitForCompletion: true,
	}
	// Use the raw API to get the exec service client, to bypass refreshing
	// This is a bit hacky.
	execServiceClient := execproto.NewExecServiceClient(context.GrpcConn)
	resp, err := execServiceClient.Exec(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp.Data)
}
*/
