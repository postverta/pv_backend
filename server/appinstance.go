package server

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/model"
	execproto "github.com/postverta/pv_exec/proto/exec"
	worktreeproto "github.com/postverta/pv_exec/proto/worktree"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"
	gcontext "golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

func HandleAppAlivePost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	context.Refresh()

	// Mark the app as accessed if I'm the owner
	if userId != "" && userId == app.UserId {
		app.AccessedTime = time.Now()
		err := model.C().UpdateApp(app, []string{"AccessedTime"})
		if err != nil {
			log.Println("[ERROR] Cannot update app in database:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppFilesGet(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	req := &execproto.ExecReq{
		TaskName:          "file_list",
		WaitForCompletion: true,
	}
	resp, err := context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	output := string(resp.Data)
	files := strings.Split(output, "\n")

	// remove the last empty element
	if len(files) > 0 && files[len(files)-1] == "" {
		files = files[0 : len(files)-1]
	}

	buf, _ := json.Marshal(files)

	w.WriteHeader(http.StatusOK)
	w.Write(buf)
}

func HandleAppFileGet(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, false)
	vars := mux.Vars(r)
	path, err := base64.StdEncoding.DecodeString(vars["path"])
	if err != nil {
		log.Println("[WARNING] Cannot decode path:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	content, err := ContextReadFile(context, string(path))
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func HandleAppFilePost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	path, err := base64.StdEncoding.DecodeString(vars["path"])
	if err != nil {
		log.Println("[WARNING] Cannot decode path:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024)
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("[ERROR] Cannot read body:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &execproto.ExecReq{
		TaskName: "file_write",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "FILEPATH",
				Value: string(path),
			},
		},
		Data:              content,
		WaitForCompletion: true,
	}
	_, err = context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if NeedUpdateSourceTimestamp(string(path)) {
		err = model.C().UpdateAppSourceTimestamp(app)
		if err != nil {
			log.Println("[ERROR] Cannot update source timestamp:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppFileMovePost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	path, err := base64.StdEncoding.DecodeString(vars["path"])
	if err != nil {
		log.Println("[WARNING] Cannot decode path:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	type Input struct {
		To string
	}
	input := Input{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&input)
	if err != nil {
		log.Println("[WARNING] Cannot unmarshal input:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &execproto.ExecReq{
		TaskName: "file_move",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "OLD_FILEPATH",
				Value: string(path),
			},
			&execproto.ExecReq_KeyValuePair{
				Key:   "NEW_FILEPATH",
				Value: input.To,
			},
		},
	}
	_, err = context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if NeedUpdateSourceTimestamp(string(path)) || NeedUpdateSourceTimestamp(input.To) {
		err = model.C().UpdateAppSourceTimestamp(app)
		if err != nil {
			log.Println("[ERROR] Cannot update source timestamp:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppFileCopyPost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	path, err := base64.StdEncoding.DecodeString(vars["path"])
	if err != nil {
		log.Println("[WARNING] Cannot decode path:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	type Input struct {
		To string
	}
	input := Input{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&input)
	if err != nil {
		log.Println("[WARNING] Cannot unmarshal input:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &execproto.ExecReq{
		TaskName: "file_copy",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "OLD_FILEPATH",
				Value: string(path),
			},
			&execproto.ExecReq_KeyValuePair{
				Key:   "NEW_FILEPATH",
				Value: input.To,
			},
		},
	}
	_, err = context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if NeedUpdateSourceTimestamp(string(path)) || NeedUpdateSourceTimestamp(input.To) {
		err = model.C().UpdateAppSourceTimestamp(app)
		if err != nil {
			log.Println("[ERROR] Cannot update source timestamp:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppFileDelete(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	path, err := base64.StdEncoding.DecodeString(vars["path"])
	if err != nil {
		log.Println("[WARNING] Cannot decode path:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &execproto.ExecReq{
		TaskName: "file_delete",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "FILEPATH",
				Value: string(path),
			},
		},
	}
	_, err = context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if NeedUpdateSourceTimestamp(string(path)) {
		err = model.C().UpdateAppSourceTimestamp(app)
		if err != nil {
			log.Println("[ERROR] Cannot update source timestamp:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppExportGet(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, false)

	// get package.json file for packages
	packageJsonContent, err := ContextReadFile(context, "package.json")
	if err != nil {
		log.Println("[ERROR] Cannot get package.json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dec := json.NewDecoder(bytes.NewReader(packageJsonContent))
	packageDict := make(map[string]interface{})
	err = dec.Decode(&packageDict)
	if err != nil {
		log.Println("[ERROR] Cannot decode package.json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Add the metadata fields to package.json
	packageDict["name"] = app.Name
	packageDict["version"] = "1.0.0"
	packageDict["description"] = app.Description
	packageDict["scripts"] = map[string]string{
		"start": app.StartCmd,
	}

	req := &execproto.ExecReq{
		TaskName:          "file_export_zip",
		WaitForCompletion: true,
	}
	resp, err := context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot run command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// golang's zip archive library doesn't allow us to append files, so have
	// to unzip and zip again
	zipReader, err := zip.NewReader(bytes.NewReader(resp.Data), int64(len(resp.Data)))
	if err != nil {
		log.Println("[ERROR] Cannot open zip reader:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Add all existing files in the archive to the new archive
	for _, rf := range zipReader.File {
		fileReader, err := rf.Open()
		if err != nil {
			// Ignore error
			log.Println("[ERROR] Cannot open zipped file:", err)
			continue
		}

		fileWriter, err := zipWriter.Create(rf.Name)
		if err != nil {
			// Ignore error
			fileReader.Close()
			log.Println("[ERROR] Cannot write zipped file:", err)
			continue
		}

		_, err = io.Copy(fileWriter, fileReader)
		if err != nil {
			log.Println("[ERROR] Cannot copy zipped file:", err)
		}

		fileReader.Close()
	}

	// Add package.json file
	packageJsonWriter, err := zipWriter.Create("package.json")
	if err != nil {
		log.Println("[ERROR] Cannot write zipped file:", err)
		return
	}

	enc := json.NewEncoder(packageJsonWriter)
	enc.SetIndent("", "  ")
	err = enc.Encode(packageDict)
	if err != nil {
		log.Println("[ERROR] Cannot encode package.json:", err)
		return
	}

	if len(app.EnvVars) > 0 {
		// Add .env file
		dotEnvWriter, err := zipWriter.Create(".env")
		if err != nil {
			log.Println("[ERROR] Cannot write zipped file:", err)
			return
		}

		for _, kv := range app.EnvVars {
			_, err = dotEnvWriter.Write([]byte(fmt.Sprintf("%s=%s\n", kv.Key, kv.Value)))
			if err != nil {
				log.Println("[ERROR] Cannot write zipped file:", err)
				return
			}
		}
	}
}

func HandleAppEnablePost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	err := ContextEnableAppProcess(context, app)
	if err != nil {
		log.Println("[ERROR] Cannot start process:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppUpdatePost(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	// We always try to enable the app before restarting it, in case the app is still sleeping
	err := ContextEnableAppProcess(context, app)
	if err != nil {
		log.Println("[ERROR] Cannot enable app:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = ContextRestartAppProcess(context, app)
	if err != nil {
		log.Println("[ERROR] Cannot restart app:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = model.C().UpdateAppRunningTimestamp(app)
	if err != nil {
		log.Println("[ERROR] Cannot update running timestamp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppForkPost(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	// Here are two cases:
	// 1) if no context of the source app exists, simply create a new app
	// and context using the existing app's worktree as source.
	// 2) if a context exists for the source app, first trigger a save to
	// make sure its worktree image is up-to-date, then create a new app and
	// context using the worktree as source.

	context, closeFunc := cluster.C().GetExistingContext(app.Id)
	if context != nil {
		wtClient := context.GetWorktreeServiceClient()
		_, err := wtClient.Save(gcontext.Background(), &worktreeproto.SaveReq{})
		if err != nil {
			log.Println("[ERROR] Cannot save worktree for app", app.Id, "err:", err)
			w.WriteHeader(http.StatusInternalServerError)
			closeFunc()
			return
		}
		closeFunc()
	}

	// Copy EnvVar keys, not values
	newEnvVars := make([]model.KeyValuePair, len(app.EnvVars))
	for i, kv := range app.EnvVars {
		newEnvVars[i] = model.KeyValuePair{
			Key:   kv.Key,
			Value: "",
		}
	}

	forkApp := &model.App{
		WorktreeId:  uuid.NewV4().String(),
		Description: app.Description,
		Icon:        app.Icon,
		UserId:      userId,
		StartCmd:    app.StartCmd,
		EnvVars:     newEnvVars,
		ApiIds:      app.ApiIds,
	}

	forkApp, err := model.C().NewApp(forkApp)
	if err != nil {
		log.Println("[ERROR] Cannot create fork app in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	context, closeFunc, err = cluster.C().GetContext(forkApp.Id, app.WorktreeId, forkApp.WorktreeId)
	if err != nil {
		log.Println("[ERROR] Cannot create fork app context:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	closeFunc()

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(forkApp.ToJsonMap())
	w.Write(buf)
}
