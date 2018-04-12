package server

import (
	"bytes"
	"encoding/json"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/model"
	execproto "github.com/postverta/pv_exec/proto/exec"
	"github.com/gorilla/mux"
	gcontext "golang.org/x/net/context"
	"log"
	"net/http"
	"strings"
)

func HandleAppPackagesGet(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	// Right now the algorithm is to get both package.json and
	// package-lock.json.  From package.json we can get the list of first-level
	// dependencies, and from package-lock.json we can know the actual version
	// of those packages.
	packageJson, err := ContextReadFile(context, "package.json")
	if err != nil {
		log.Println("[ERROR] Cannot read package.json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	packageLockJson, err := ContextReadFile(context, "package-lock.json")
	if err != nil {
		log.Println("[ERROR] Cannot read package-lock.json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dec := json.NewDecoder(bytes.NewReader(packageJson))
	packageDict := make(map[string]interface{})
	err = dec.Decode(&packageDict)
	if err != nil {
		log.Println("[ERROR] Cannot decode package.json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dec = json.NewDecoder(bytes.NewReader(packageLockJson))
	packageLockDict := make(map[string]interface{})
	err = dec.Decode(&packageLockDict)
	if err != nil {
		log.Println("[ERROR] Cannot decode package-lock.json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	depsElement, found := packageDict["dependencies"]
	var deps map[string]interface{}
	if found {
		deps = depsElement.(map[string]interface{})
	} else {
		deps = make(map[string]interface{})
	}

	lockDepsElement, found := packageLockDict["dependencies"]
	var lockDeps map[string]interface{}
	if found {
		lockDeps = lockDepsElement.(map[string]interface{})
	} else {
		lockDeps = make(map[string]interface{})
	}

	results := make(map[string]interface{})
	for name, _ := range deps {
		if lockDeps[name] == nil {
			// This shouldn't happen, usually meaning an inconsistency
			// between package.json and package-lock.json. Log a message.
			log.Printf("[ERROR] Cannot find package %s in package-lock.json", name)
			continue
		}

		lockDep := lockDeps[name].(map[string]interface{})
		if lockDep == nil {
			continue
		}

		version := lockDep["version"].(string)
		if version == "" {
			continue
		}

		result := make(map[string]interface{})
		results[name] = result
		result["version"] = version
		result["api"] = nil
	}

	// Check all the APIs enabled for this app. If any package is added
	// as part of an API, we need to mark it.
	apis, err := model.C().GetApisByIds(app.ApiIds)
	if err != nil {
		log.Println("[ERROR] Cannot read APIs from database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, api := range apis {
		for _, pkg := range api.Packages {
			result, found := results[pkg]
			if !found {
				log.Println("Inconsistency between APIs and packages")
				continue
			}
			resultMap := result.(map[string]interface{})
			resultMap["api"] = api.Name
		}
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(results)
	w.Write(buf)
}

func HandleAppPackagePost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	packageName := vars["name"]
	parts := strings.Split(packageName, "@")

	if len(parts) != 2 && len(parts) != 3 {
		log.Println("[WARNING] Package name must be in the format of [@scope/]pkg@version")
		w.WriteHeader(http.StatusBadRequest)
	}

	if len(parts) == 3 && parts[0] != "" {
		log.Println("[WARNING] Wrong scope format")
		w.WriteHeader(http.StatusBadRequest)
	}

	var name string
	var version string
	if len(parts) == 2 {
		name = parts[0]
		version = parts[1]
	} else {
		name = "@" + parts[1]
		version = parts[2]
	}

	req := &execproto.ExecReq{
		TaskName: "npm_install_pkg",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "NAME",
				Value: name,
			},
			&execproto.ExecReq_KeyValuePair{
				Key:   "VERSION",
				Value: version,
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

	// Also sync types
	err = ContextSyncTypes(context)
	if err != nil {
		log.Println("[ERROR] Cannot sync types:", err)
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
}

func HandleAppPackageDelete(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	name := vars["name"]

	req := &execproto.ExecReq{
		TaskName: "npm_remove_pkg",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "NAME",
				Value: name,
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

	// Also sync types
	err = ContextSyncTypes(context)
	if err != nil {
		log.Println("[ERROR] Cannot sync types:", err)
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
}
