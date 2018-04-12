package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/model"
	execproto "github.com/postverta/pv_exec/proto/exec"
	"github.com/satori/go.uuid"
	gcontext "golang.org/x/net/context"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"
)

// Couple of heuristics to look for the root directory in the ZIP file
func findProjectRootAndPackageJson(zipContent []byte) (rootDir string, packageJsonString string, err error) {
	dataReader := bytes.NewReader(zipContent)
	zipReader, err := zip.NewReader(dataReader, int64(len(zipContent)))
	if err != nil {
		return "", "", err
	}

	// The algorithm is fairly simple and naive. We find the package.json with the shortest path length.
	var packageJsonFile *zip.File
	shortestPathLength := 0
	rootDir = ""
	for _, file := range zipReader.File {
		if file.Name == "package.json" || strings.HasSuffix(file.Name, "/package.json") {
			if len(file.Name) < shortestPathLength || shortestPathLength == 0 {
				shortestPathLength = len(file.Name)
				packageJsonFile = file
				rootDir = path.Dir(file.Name)
			}
		}
	}

	if packageJsonFile == nil {
		return "", "", fmt.Errorf("Cannot find package.json in the zip")
	}

	packageJsonReader, err := packageJsonFile.Open()
	if err != nil {
		return "", "", err
	}
	defer packageJsonReader.Close()

	packageJsonContent, err := ioutil.ReadAll(packageJsonReader)
	if err != nil {
		return "", "", err
	}

	return rootDir, string(packageJsonContent), nil
}

func HandleAppsUploadPost(userId string, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("[WARNING] Cannot read body:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rootDir, packageJsonString, err := findProjectRootAndPackageJson(content)
	if err != nil {
		log.Println("[WARNING] Cannot get package.json:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	description, err := GetPackageJsonDescription(packageJsonString)
	if err != nil {
		log.Println("[WARNING] Cannot parse package.json")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Allocate a new worktree ID
	app := &model.App{
		WorktreeId:  uuid.NewV4().String(),
		Description: description,
		UserId:      userId,
		StartCmd:    "node index.js",
		EnvVars:     []model.KeyValuePair{},
		ApiIds:      []string{},
	}

	app, err = model.C().NewApp(app)
	if err != nil {
		log.Println("[ERROR] Cannot create app in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	context, closeFunc, err := cluster.C().GetContext(app.Id, "", app.WorktreeId)
	if err != nil {
		log.Println("[ERROR] Cannot get context for app", app.Id, "err:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer closeFunc()

	execReq := &execproto.ExecReq{
		TaskName: "zip_import",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "PROJECTROOT",
				Value: rootDir,
			},
		},
		Data:              content,
		WaitForCompletion: true,
	}
	_, err = context.GetExecServiceClient().Exec(gcontext.Background(), execReq)
	if err != nil {
		log.Println("[ERROR] Cannot exec command:", err)
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

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}
