package server

import (
	"encoding/json"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/model"
	execproto "github.com/postverta/pv_exec/proto/exec"
	"github.com/gorilla/mux"
	gcontext "golang.org/x/net/context"
	"log"
	"net/http"
)

func HandleAppApisGet(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	// Note this function will return all APIs, but each API will have a field "enabled" to
	// show whether it is enabled for this app
	SetCommonHeaders(w, true)
	apis, err := model.C().GetApis()
	if err != nil {
		log.Println("[ERROR] Cannot get APIs in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	results := make([]map[string]interface{}, len(apis))
	for i, api := range apis {
		results[i] = api.ToJsonMap()
		results[i]["enabled"] = false
		for _, apiId := range app.ApiIds {
			if apiId == api.Id {
				results[i]["enabled"] = true
				break
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(results)
	w.Write(buf)
}

func HandleAppApiPost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	apiId := vars["api_id"]

	api, err := model.C().GetApi(apiId)
	if err != nil {
		log.Println("[ERROR] Cannot get API in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if api == nil {
		log.Println("[WARNING] Cannot find API")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	found := false
	for _, apiId := range app.ApiIds {
		if apiId == api.Id {
			found = true
			break
		}
	}

	if found {
		log.Println("[INFO] API is already enabled")
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, pkg := range api.Packages {
		req := &execproto.ExecReq{
			TaskName: "npm_install_pkg",
			KeyValues: []*execproto.ExecReq_KeyValuePair{
				&execproto.ExecReq_KeyValuePair{
					Key:   "NAME",
					Value: pkg,
				},
			},
			WaitForCompletion: true,
		}
		_, err := context.GetExecServiceClient().Exec(gcontext.Background(), req)
		if err != nil {
			log.Println("[ERROR] Cannot run command:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	app.ApiIds = append(app.ApiIds, api.Id)
	err = model.C().UpdateApp(app, []string{"ApiIds"})
	if err != nil {
		log.Println("[ERROR] Cannot save app to database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = model.C().UpdateAppSourceTimestamp(app)
	if err != nil {
		log.Println("[ERROR] Cannot update source timestamp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
	return
}

func HandleAppApiDelete(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	apiId := vars["api_id"]

	api, err := model.C().GetApi(apiId)
	if err != nil {
		log.Println("[ERROR] Cannot get API in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if api == nil {
		log.Println("[WARNING] Cannot find API")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	found := false
	idx := 0
	for i, apiId := range app.ApiIds {
		if apiId == api.Id {
			found = true
			idx = i
			break
		}
	}

	if !found {
		log.Println("[INFO] API is already disabled")
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, pkg := range api.Packages {
		req := &execproto.ExecReq{
			TaskName: "npm_remove_pkg",
			KeyValues: []*execproto.ExecReq_KeyValuePair{
				&execproto.ExecReq_KeyValuePair{
					Key:   "NAME",
					Value: pkg,
				},
			},
			WaitForCompletion: true,
		}
		_, err := context.GetExecServiceClient().Exec(gcontext.Background(), req)
		if err != nil {
			log.Println("[ERROR] Cannot run command:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	copy(app.ApiIds[idx:], app.ApiIds[idx+1:])
	app.ApiIds[len(app.ApiIds)-1] = ""
	app.ApiIds = app.ApiIds[:len(app.ApiIds)-1]

	err = model.C().UpdateApp(app, []string{"ApiIds"})
	if err != nil {
		log.Println("[ERROR] Cannot save app to database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = model.C().UpdateAppSourceTimestamp(app)
	if err != nil {
		log.Println("[ERROR] Cannot update source timestamp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
	return
}
